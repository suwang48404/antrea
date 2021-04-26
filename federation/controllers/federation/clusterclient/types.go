/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package clusterclient

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
)

type (
	FederationID string
	ClusterID    string
)

type LeaderEvent struct {
	IsLeader bool
	Cluster  ClusterID
}

func (e LeaderEvent) Key() string {
	return fmt.Sprintf("LeaderEvent:%s", e.Cluster)
}

type Resource struct {
	runtime.Object
	Cluster ClusterID
}

// key uniquely identifies a resource across entire federation.
func (r Resource) Key() string {
	obj, err := meta.Accessor(r.Object)
	if err != nil {
		return ""
	}
	if len(r.Cluster) == 0 {
		return r.FedKey()
	}
	return GenerateResourceKey(obj.GetName(), obj.GetNamespace(), r.Kind(), r.Cluster)
}

// fedKey uniquely identifies a federated resource.
func (r Resource) FedKey() string {
	obj, err := meta.Accessor(r.Object)
	if err != nil {
		return ""
	}
	if _, ok := obj.GetAnnotations()[config.FederationIDAnnotation]; !ok {
		return ""
	}
	return GenerateFedResourceKey(obj.GetName(), obj.GetNamespace(), r.Kind())
}

func (r Resource) Kind() string {
	// Use reflection because delete Resources do not have Kind field set.
	return reflect.TypeOf(r.Object).Elem().Name()
}

func (r Resource) Name() string {
	obj, err := meta.Accessor(r.Object)
	if err != nil {
		return ""
	}
	return obj.GetName()
}

func (r Resource) Namespace() string {
	obj, err := meta.Accessor(r.Object)
	if err != nil {
		return ""
	}
	return obj.GetNamespace()
}

func (r Resource) KindAndCluster() string {
	return string(r.Cluster) + ":" + r.Kind()
}

func (r Resource) Labels() map[string]string {
	obj, err := meta.Accessor(r.Object)
	if err != nil {
		return nil
	}
	return obj.GetLabels()
}

func (r Resource) Annotations() map[string]string {
	obj, err := meta.Accessor(r.Object)
	if err != nil {
		return nil
	}
	return obj.GetAnnotations()
}

func (r Resource) IsDelete() bool {
	return len(r.GetObjectKind().GroupVersionKind().Kind) == 0
}

func (r Resource) FederationID() FederationID {
	access, _ := meta.Accessor(r.Object)
	id, ok := access.GetAnnotations()[config.FederationIDAnnotation]
	if !ok {
		return ""
	}
	return FederationID(id)
}

func (r Resource) UserDefinedFederatedKey() *client.ObjectKey {
	access, _ := meta.Accessor(r.Object)
	for k, v := range access.GetAnnotations() {
		if strings.HasSuffix(k, config.UserFedAnnotationPostfix) {
			return &client.ObjectKey{
				Namespace: access.GetNamespace(),
				Name:      v,
			}
		}
	}
	return nil
}

func GenerateResourceKey(name, ns, kind string, cluster ClusterID) string {
	return fmt.Sprintf("%v:%v:%s/%s", cluster, kind, name, ns)
}

func GenerateFedResourceKey(name, ns, kind string) string {
	return fmt.Sprintf("%v:%s/%s", kind, name, ns)
}
