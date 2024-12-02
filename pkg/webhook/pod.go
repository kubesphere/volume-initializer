package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"slices"

	"github.com/kubesphere/volume-initializer/pkg/apis/storage/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	tenantv1alpha1 "kubesphere.io/api/tenant/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ReqInfo struct {
	Pod *corev1.Pod
}

func NewReqInfo(pod *corev1.Pod) *ReqInfo {
	return &ReqInfo{
		Pod: pod,
	}
}

func toV1AdmissionResponseWithPatch(patch []byte) *admissionv1.AdmissionResponse {
	pt := admissionv1.PatchTypeJSONPatch
	resp := &admissionv1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}
	if len(patch) > 0 {
		resp.Patch = patch
		resp.PatchType = &pt
	}
	return resp
}

type AdmitterInterface interface {
	Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
	Decide(ctx context.Context, reqInfo *ReqInfo) *admissionv1.AdmissionResponse
}

type Admitter struct {
	client client.Client
}

var _ AdmitterInterface = (*Admitter)(nil)

func NewAdmitter() (*Admitter, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	cli, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	a := &Admitter{
		client: cli,
	}
	return a, nil
}

func NewAdmitterWithClient(client client.Client) AdmitterInterface {
	return &Admitter{
		client: client,
	}
}

func (a *Admitter) serverPVCRequest(w http.ResponseWriter, r *http.Request) {
	server(w, r, newDelegateToV1AdmitHandler(a.Admit))
}

func (a *Admitter) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	if ar.Request.Operation != admissionv1.Create {
		return toV1AdmissionResponseWithPatch(nil)
	}

	raw := ar.Request.Object.Raw
	namespace := ar.Request.Namespace
	deserializer := codecs.UniversalDeserializer()
	podObj := &corev1.Pod{}
	obj, _, err := deserializer.Decode(raw, nil, podObj)
	if err != nil {
		klog.ErrorS(err, "failed to decode raw object")
		return toV1AdmissionResponse(err)
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Infof("object %+v is not a pod", obj)
		return toV1AdmissionResponseWithPatch(nil)
	}

	// the creating pod(Request.Object) may not have name or namespace set, keep that in mind. We need to set it here.
	pod.Namespace = namespace

	reqInfo := NewReqInfo(pod)

	klog.Infof("request info: %+v", reqInfo)
	return a.Decide(context.Background(), reqInfo)
}

const (
	EnvVarPVC1MountPath = "PVC_1_MOUNT_PATH"
	EnvVarPVC1UID       = "PVC_1_UID"
	EnvVarPVC1GID       = "PVC_1_GID"
)

func (a *Admitter) Decide(ctx context.Context, reqInfo *ReqInfo) *admissionv1.AdmissionResponse {
	var err error

	if reqInfo == nil || reqInfo.Pod == nil || len(reqInfo.Pod.Spec.Volumes) == 0 {
		return toV1AdmissionResponseWithPatch(nil)
	}

	var containerNames []string
	for _, c := range reqInfo.Pod.Spec.Containers {
		containerNames = append(containerNames, c.Name)
	}

	initializerList := &v1alpha1.InitializerList{}
	err = a.client.List(ctx, initializerList)
	if err != nil {
		klog.ErrorS(err, "failed to list Initializers")
		return toV1AdmissionResponse(err)
	}

	var initContainersToAdd []*corev1.Container
	for _, volume := range reqInfo.Pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvc := &corev1.PersistentVolumeClaim{}
			err = a.client.Get(ctx, types.NamespacedName{
				Namespace: reqInfo.Pod.Namespace,
				Name:      volume.PersistentVolumeClaim.ClaimName,
			}, pvc)
			if err != nil {
				klog.ErrorS(err, "failed to get PersistentVolumeClaim", "namespace", reqInfo.Pod.Namespace, "name", volume.PersistentVolumeClaim.ClaimName)
				return toV1AdmissionResponse(err)
			}
			var pvcInitContainer *PVCInitContainer
			pvcInitContainer, err = a.getPVCInitContainer(ctx, reqInfo, pvc, initializerList)
			if err != nil {
				klog.ErrorS(err, "failed to get PVCInitContainer", "pvc", pvc.Name)
				return toV1AdmissionResponse(err)
			}
			if pvcInitContainer == nil {
				klog.Infof("no initContainer matches pvc %s", pvc.Name)
				continue
			}
			if pvcInitContainer.MountPathRoot == "" {
				pvcInitContainer.MountPathRoot = "/"
			}
			container := pvcInitContainer.Container
			container.Name = fmt.Sprintf("%s-vol-%s", container.Name, volume.Name)

			// check if the container already exists
			if slices.Contains(containerNames, container.Name) {
				klog.Warningf("initContainer %s already exists in pod or patch", container.Name)
				continue
			}
			containerNames = append(containerNames, container.Name)

			mountPath := path.Join(pvcInitContainer.MountPathRoot, volume.Name)
			volumeMount := corev1.VolumeMount{
				Name:      volume.Name,
				MountPath: path.Join(pvcInitContainer.MountPathRoot, volume.Name),
			}
			container.VolumeMounts = append(container.VolumeMounts, volumeMount)
			envVarMountPath := corev1.EnvVar{
				Name:  EnvVarPVC1MountPath,
				Value: mountPath,
			}
			container.Env = append(container.Env, envVarMountPath)

			uid, gid := a.getVolumeUIDGIDFromPodLabels(volume.Name, reqInfo.Pod)
			if uid != "" {
				envVarUID := corev1.EnvVar{
					Name:  EnvVarPVC1UID,
					Value: uid,
				}
				container.Env = append(container.Env, envVarUID)
			}
			if gid != "" {
				envVarGID := corev1.EnvVar{
					Name:  EnvVarPVC1GID,
					Value: gid,
				}
				container.Env = append(container.Env, envVarGID)
			}

			initContainersToAdd = append(initContainersToAdd, container)
		}
	}

	if len(initContainersToAdd) > 0 {
		patch, err := initContainersToPatch(initContainersToAdd)
		if err != nil {
			klog.ErrorS(err, "failed to generate patch")
			return toV1AdmissionResponse(err)
		}
		return toV1AdmissionResponseWithPatch(patch)
	}

	return toV1AdmissionResponseWithPatch(nil)
}

const (
	podsInitContainerPatch string = `[
		 {"op":"add","path":"/spec/initContainers","value":%s}
	]`
	LabelVolumeUID         = "volume.storage.kubesphere.io/uid"
	LabelVolumeGID         = "volume.storage.kubesphere.io/gid"
	LabelSpecificVolumeUID = "%s.volume.storage.kubesphere.io/uid"
	LabelSpecificVolumeGID = "%s.volume.storage.kubesphere.io/gid"
)

func (a *Admitter) getVolumeUIDGIDFromPodLabels(volumeName string, pod *corev1.Pod) (uid, gid string) {
	for k, v := range pod.Labels {
		switch k {
		case LabelVolumeUID:
			uid = v
		case LabelVolumeGID:
			gid = v
		}
	}
	for k, v := range pod.Labels {
		annoUID := fmt.Sprintf(LabelSpecificVolumeUID, volumeName)
		annoGID := fmt.Sprintf(LabelSpecificVolumeGID, volumeName)
		switch k {
		case annoUID:
			uid = v
		case annoGID:
			gid = v
		}
	}
	return
}

func initContainersToPatch(initContainers []*corev1.Container) ([]byte, error) {
	containersBytes, err := json.Marshal(initContainers)
	if err != nil {
		return nil, err
	}
	patch := fmt.Sprintf(podsInitContainerPatch, string(containersBytes))
	return []byte(patch), nil
}

type PVCInitContainer struct {
	PVC           *corev1.PersistentVolumeClaim
	Container     *corev1.Container
	MountPathRoot string
}

// getPVCInitContainer returns a PVInitContainer that matches the pvc.
// If pvc does not match any pvcMatcher, nil will be returned.
// If pvc matches multiple pvcMatchers, the first one will be used and the corresponding initContainer will be returned.
func (a *Admitter) getPVCInitContainer(ctx context.Context, reqInfo *ReqInfo, pvc *corev1.PersistentVolumeClaim, initializerList *v1alpha1.InitializerList) (*PVCInitContainer, error) {
	getPvcMatcherByName := func(matcherName string, pvcMatchers []v1alpha1.PVCMatcher) *v1alpha1.PVCMatcher {
		for _, m := range pvcMatchers {
			if m.Name == matcherName {
				return &m
			}
		}
		return nil
	}

	getContainerByName := func(name string, containers []corev1.Container) *corev1.Container {
		for _, c := range containers {
			if c.Name == name {
				return &c
			}
		}
		return nil
	}

	for _, initializer := range initializerList.Items {
		if !initializer.Spec.Enabled {
			klog.Infof("initializer %s not enabled", initializer.Name)
			continue
		}
		for _, pvcInitializer := range initializer.Spec.PVCInitializers {
			pvcMatcher := getPvcMatcherByName(pvcInitializer.PVCMatcherName, initializer.Spec.PVCMatchers)
			match, err := a.pvcMatch(ctx, reqInfo.Pod, pvc, pvcMatcher)
			if err != nil {
				return nil, err
			}
			if match {
				container := getContainerByName(pvcInitializer.InitContainerName, initializer.Spec.InitContainers)
				if container == nil {
					klog.Warningf("initContainer %s not found in initializer %s", pvcInitializer.InitContainerName, initializer.Name)
					continue
				}
				pvcInitContainer := &PVCInitContainer{
					PVC:           pvc,
					Container:     container,
					MountPathRoot: pvcInitializer.MountPathRoot,
				}
				return pvcInitContainer, nil
			}
		}
	}
	return nil, nil
}

func (a *Admitter) pvcMatch(ctx context.Context, pod *corev1.Pod, pvc *corev1.PersistentVolumeClaim, pvcMatcher *v1alpha1.PVCMatcher) (bool, error) {
	var err error

	if pvcMatcher.PVC != nil {
		match := pvcMatcher.PVC.Match(pvc)
		if !match {
			return false, nil
		}
	}

	if pvcMatcher.Pod != nil {
		match := pvcMatcher.Pod.Match(pod)
		if !match {
			return false, nil
		}
	}

	if pvcMatcher.StorageClass != nil && pvc.Spec.StorageClassName != nil {
		if *pvc.Spec.StorageClassName == "" {
			return false, nil
		}
		sc := &v1.StorageClass{}
		err = a.client.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, sc)
		if err != nil {
			return false, err
		}
		match := pvcMatcher.StorageClass.Match(sc)
		if !match {
			return false, nil
		}
	}

	ns := &corev1.Namespace{}
	err = a.client.Get(ctx, types.NamespacedName{Name: pvc.Namespace}, ns)
	if err != nil {
		return false, err
	}

	if pvcMatcher.Namespace != nil {
		match := pvcMatcher.Namespace.Match(ns)
		if !match {
			return false, nil
		}
	}

	wsName, ok := ns.Labels["kubesphere.io/workspace"]
	if ok && pvcMatcher.Workspace != nil {
		if wsName == "" {
			return false, nil
		}
		ws := &tenantv1alpha1.Workspace{}
		err = a.client.Get(ctx, types.NamespacedName{Name: wsName}, ws)
		if err != nil {
			return false, err
		}
		match := pvcMatcher.Workspace.Match(ws)
		if !match {
			return false, nil
		}
	}

	return true, nil
}
