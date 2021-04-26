/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package webhook

import (
	"fmt"
	"reflect"
	"sync"

	logging "github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// Cache provide storage for k8s runtime object *before*
// it is Created/Updated/Deleted to API endpoint. The cache then is
// used by ValidationWebhook to validate a request.
// It is used as follows
// 1. controller Update cache before write to API endpoint.
// 2. webhook  validate request from cache in 1.
type Cache interface {
	// Update creates or updates an runtime.Object.
	Update(obj runtime.Object)

	// Validates returns true nil:
	// when Spec field exits, they deep equal, and  labels in obj subsumes labels in cached items.
	// or cache not found, that is request CRD is not owned by controller
	Validate(obj runtime.Object) error

	// Delete deletes obj from cache
	Delete(obj runtime.Object)

	// Has return cache resource.
	Get(obj runtime.Object) runtime.Object
}

func NewCache(logger logging.Logger) Cache {
	return &cache{
		cache:  make(map[string]runtime.Object),
		logger: logger,
	}
}

type cache struct {
	mutex  sync.Mutex
	cache  map[string]runtime.Object
	logger logging.Logger
}

func getObjectKey(obj runtime.Object) string {
	accessor, _ := meta.Accessor(obj)
	return reflect.TypeOf(obj).Elem().Name() + "/" + accessor.GetNamespace() + "/" + accessor.GetName()
}

func (c *cache) Update(obj runtime.Object) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := getObjectKey(obj)
	c.logger.V(1).Info("Update", "key", key)
	c.cache[key] = obj
}

func (c *cache) Delete(obj runtime.Object) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := getObjectKey(obj)
	c.logger.V(1).Info("delete", "key", key)
	delete(c.cache, key)
}

func (c *cache) Get(obj runtime.Object) runtime.Object {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	val, ok := c.cache[getObjectKey(obj)]
	if !ok {
		return nil
	}
	return val.DeepCopyObject()
}

func (c *cache) Validate(obj runtime.Object) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	key := getObjectKey(obj)
	c.logger.V(1).Info("Validate", "key", key)

	in, ok := c.cache[key]
	if !ok {
		return fmt.Errorf("resource %s is not found webhook cache", key)
	}

	acc1, _ := meta.Accessor(in)
	acc2, _ := meta.Accessor(obj)
	inlabels := acc1.GetLabels()
	objlabels := acc2.GetLabels()
	for k, v := range inlabels {
		ov, ok := objlabels[k]
		if !ok || ov != v {
			return fmt.Errorf("mismatch label: key=%v val=%v/%v", k, v, ov)
		}
	}

	spec1 := reflect.ValueOf(in).Elem().FieldByName("Spec").Interface()
	spec2 := reflect.ValueOf(obj).Elem().FieldByName("Spec").Interface()
	if !reflect.DeepEqual(spec1, spec2) {
		return fmt.Errorf("mismatch spec: %v : %v", spec1, spec2)
	}
	return nil
}
