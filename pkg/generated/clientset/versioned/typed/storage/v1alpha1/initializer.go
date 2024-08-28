/*
Copyright 2020 The KubeSphere Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"

	v1alpha1 "github.com/kubesphere/volume-initializer/pkg/apis/storage/v1alpha1"
	scheme "github.com/kubesphere/volume-initializer/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// InitializersGetter has a method to return a InitializerInterface.
// A group's client should implement this interface.
type InitializersGetter interface {
	Initializers() InitializerInterface
}

// InitializerInterface has methods to work with Initializer resources.
type InitializerInterface interface {
	Create(ctx context.Context, initializer *v1alpha1.Initializer, opts v1.CreateOptions) (*v1alpha1.Initializer, error)
	Update(ctx context.Context, initializer *v1alpha1.Initializer, opts v1.UpdateOptions) (*v1alpha1.Initializer, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, initializer *v1alpha1.Initializer, opts v1.UpdateOptions) (*v1alpha1.Initializer, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.Initializer, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.InitializerList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Initializer, err error)
	InitializerExpansion
}

// initializers implements InitializerInterface
type initializers struct {
	*gentype.ClientWithList[*v1alpha1.Initializer, *v1alpha1.InitializerList]
}

// newInitializers returns a Initializers
func newInitializers(c *StorageV1alpha1Client) *initializers {
	return &initializers{
		gentype.NewClientWithList[*v1alpha1.Initializer, *v1alpha1.InitializerList](
			"initializers",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *v1alpha1.Initializer { return &v1alpha1.Initializer{} },
			func() *v1alpha1.InitializerList { return &v1alpha1.InitializerList{} }),
	}
}