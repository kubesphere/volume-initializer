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
	PVCMatcherName    string `json:"pvcMatcherName,omitempty"`
	InitContainerName string `json:"initContainerName,omitempty"`
	MountPathRoot     string `json:"mountPathRoot,omitempty"`
}

type PVCMatcher struct {
	Name        string                       `json:"name,omitempty"`
	PVCTemplate corev1.PersistentVolumeClaim `json:"pvcTemplate,omitempty"`
}

type InitializerStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InitializerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Initializer `json:"items"`
}
