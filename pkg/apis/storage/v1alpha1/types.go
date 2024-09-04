/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Initializer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InitializerSpec   `json:"spec"`
	Status InitializerStatus `json:"status"`
}

type InitializerSpec struct {
	Enabled         bool               `json:"enabled,omitempty"`
	InitContainers  []corev1.Container `json:"initContainers,omitempty"`
	PVCMatchers     []PVCMatcher       `json:"pvcMatchers,omitempty"`
	PVCInitializers []PVCInitializer   `json:"pvcInitializers,omitempty"`
}

type PVCInitializer struct {
	// PVCMatcherName represents the name of PVCMatcher
	PVCMatcherName string `json:"pvcMatcherName,omitempty"`

	// InitContainerName represents the name of the init container
	InitContainerName string `json:"initContainerName,omitempty"`

	// MountPathRoot represents the root path of the mount point in the init container, default is "/".
	MountPathRoot string `json:"mountPathRoot,omitempty"`
}

// PVCMatcher is used to filter PVCs. If no selector is specified, it will match any PVC.
type PVCMatcher struct {
	// Name is the matcher name
	Name string `json:"name,omitempty"`

	// StorageClass matches the PVC's storage class
	StorageClass *GenericSelector `json:"storageClass,omitempty"`

	// Namespace matches the PVC's namespace
	Namespace *GenericSelector `json:"namespace,omitempty"`

	// Workspace matches the PVC's workspace
	Workspace *GenericSelector `json:"workspace,omitempty"`
}

type InitializerStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InitializerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Initializer `json:"items"`
}
