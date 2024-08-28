package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/kubesphere/volume-initializer/pkg/apis/storage/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
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
	deserializer := codecs.UniversalDeserializer()
	pod := &corev1.Pod{}
	_, _, err := deserializer.Decode(raw, nil, pod)
	if err != nil {
		klog.ErrorS(err, "failed to decode raw object")
		return toV1AdmissionResponse(err)
	}

	reqInfo := NewReqInfo(pod)

	klog.Infof("request info: %v", reqInfo)
	return a.Decide(context.Background(), reqInfo)
}

const (
	EnvVarPVC1MountPath = "PVC_1_MOUNT_PATH"
)

func (a *Admitter) Decide(ctx context.Context, reqInfo *ReqInfo) *admissionv1.AdmissionResponse {
	var err error

	if reqInfo == nil || reqInfo.Pod == nil || len(reqInfo.Pod.Spec.Volumes) == 0 {
		return toV1AdmissionResponseWithPatch(nil)
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
			pvcInitContainer, err = getPVCInitContainer(pvc, initializerList)
			if err != nil {
				klog.ErrorS(err, "failed to get PVCInitContainer", "pvc", pvc.Name)
				return toV1AdmissionResponse(err)
			}
			if pvcInitContainer == nil {
				continue
			}
			if pvcInitContainer.MountPathRoot == "" {
				pvcInitContainer.MountPathRoot = "/"
			}
			container := pvcInitContainer.Container
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
)

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

func getPVCInitContainer(pvc *corev1.PersistentVolumeClaim, initializerList *v1alpha1.InitializerList) (*PVCInitContainer, error) {
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
			continue
		}
		for _, pvcInitializer := range initializer.Spec.PVCInitializers {
			pvcMatcher := getPvcMatcherByName(pvcInitializer.PVCMatcherName, initializer.Spec.PVCMatchers)
			if pvcMatch(pvc, pvcMatcher) {
				container := getContainerByName(pvcInitializer.InitContainerName, initializer.Spec.InitContainers)
				if container == nil {
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

func pvcMatch(pvc *corev1.PersistentVolumeClaim, matcher *v1alpha1.PVCMatcher) bool {
	return matcher.PVCTemplate.Spec.StorageClassName == pvc.Spec.StorageClassName
}
