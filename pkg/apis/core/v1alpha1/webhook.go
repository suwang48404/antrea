/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package v1alpha1

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
)

// WebhookImpl implements webhook validator of a resource.
type WebhookImpl interface {
	Default(in *ExternalEntity)
	ValidateCreate(in *ExternalEntity) error
	ValidateUpdate(in *ExternalEntity, old runtime.Object) error
	ValidateDelete(in *ExternalEntity) error
}

var (
	externalEntityWebhook WebhookImpl
)

// RegisterWebhook registers webhook implementation of a resource.
func RegisterWebhook(in runtime.Object, webhook WebhookImpl) error {
	switch in.(type) {
	case *ExternalEntity:
		if externalEntityWebhook != nil {
			return fmt.Errorf("externalEntityWebhook already registered")
		}
		externalEntityWebhook = webhook
	default:
		return fmt.Errorf("unknown type %s to register webhook", reflect.TypeOf(in).Elem().Name())
	}
	return nil
}

// Default implements webhook Defaulter.
func (in *ExternalEntity) Default() {
	if externalEntityWebhook != nil {
		externalEntityWebhook.Default(in)
	}
	return
}

// ValidateCreate implements webhook Validator.
func (in *ExternalEntity) ValidateCreate() error {
	if externalEntityWebhook != nil {
		return externalEntityWebhook.ValidateCreate(in)
	}
	return nil
}

// ValidateUpdate implements webhook Validator.
func (in *ExternalEntity) ValidateUpdate(old runtime.Object) error {
	if externalEntityWebhook != nil {
		return externalEntityWebhook.ValidateUpdate(in, old)
	}

	return nil
}

// ValidateDelete implements webhook Validator.
func (in *ExternalEntity) ValidateDelete() error {
	if externalEntityWebhook != nil {
		return externalEntityWebhook.ValidateDelete(in)
	}
	return nil
}
