/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package webhook

import (
	"fmt"

	logging "github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

// log is for logging in this package.
var (
	externalEntityLog          logging.Logger
	ExternalEntityWebhookCache = NewCache(log.NullLogger{})
	RegisteredOwner            = make(map[string]struct{})
)

type ExternalEntityWebhook struct{}

func (r *ExternalEntityWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	ExternalEntityWebhookCache = NewCache(mgr.GetLogger().WithName("ee-cache"))
	externalEntity := &antreatypes.ExternalEntity{}
	_ = antreatypes.RegisterWebhook(externalEntity, r)
	return ctrl.NewWebhookManagedBy(mgr).
		For(externalEntity).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-crd-antrea-io-v1alpha2-externalentity,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.antrea.io,resources=externalentities,verbs=create;update,versions=v1alpha2,name=mexternalentity.kb.io,admissionReviewVersions={v1,v1beta1}
// nolint:lll
// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-crd-antrea-io-v1alpha2-externalentity,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.antrea.io,resources=externalentities,versions=v1alpha2,name=vexternalentity.kb.io,admissionReviewVersions={v1,v1beta1}
// nolint:lll

var _ webhook.Defaulter = &antreatypes.ExternalEntity{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *ExternalEntityWebhook) Default(in *antreatypes.ExternalEntity) {
	externalEntityLog.Info("default", "name", in.Name)
	// TODO(user): fill in your defaulting logic.
}

var _ webhook.Validator = &antreatypes.ExternalEntity{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ExternalEntityWebhook) ValidateCreate(in *antreatypes.ExternalEntity) error {
	externalEntityLog.Info("validate create", "name", in.Name, "generation", in.GetGeneration())
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ExternalEntityWebhook) ValidateUpdate(in *antreatypes.ExternalEntity, _ runtime.Object) error {
	externalEntityLog.Info("validate update", "name", in.Name, "generation", in.GetGeneration())
	if cached := ExternalEntityWebhookCache.Get(in); cached == nil {
		return nil
	}
	if err := ExternalEntityWebhookCache.Validate(in); err != nil {
		return fmt.Errorf("%v validate update: %w", in.Name, err)
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ExternalEntityWebhook) ValidateDelete(in *antreatypes.ExternalEntity) error {
	externalEntityLog.Info("validate delete", "name", in.Name, "generation", in.GetGeneration())
	cached := ExternalEntityWebhookCache.Get(in)
	if cached != nil && hasValidOwner(cached.(*antreatypes.ExternalEntity)) {
		return fmt.Errorf("%v validate delete while in use", in.Name)
	}
	return nil
}

// hasValidOwner returns true if its ownerReference pointing to an registered owner.
func hasValidOwner(in *antreatypes.ExternalEntity) bool {
	for _, owner := range in.OwnerReferences {
		externalEntityLog.V(1).Info("", "owner", owner.Kind)
		if owner.Controller == nil || !*owner.Controller {
			continue
		}
		if _, ok := RegisteredOwner[owner.Kind]; ok {
			return true
		}
	}
	return false
}
