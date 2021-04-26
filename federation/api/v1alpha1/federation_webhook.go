/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
)

// log is for logging in this package.
var (
	federationlog = logf.Log.WithName("federation-resource")
	client        controllerclient.Client
)

func (r *Federation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// nolint:lll
// +kubebuilder:webhook:path=/mutate-federation-antrea-io-v1alpha1-federation,mutating=true,failurePolicy=fail,sideEffects=None,groups=mcs.crd.antrea.io,resources=federations,verbs=create;update,versions=v1alpha1,name=mfederation.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Federation{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Federation) Default() {
	federationlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// nolint:lll
// +kubebuilder:webhook:verbs=create;update,path=/validate-federation-antrea-io-v1alpha1-federation,mutating=false,failurePolicy=fail,sideEffects=None,groups=mcs.crd.antrea.io,resources=federations,versions=v1alpha1,name=vfederation.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Federation{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Federation) ValidateCreate() error {
	federationlog.Info("validate create", "name", r.Name)
	var err error

	// well known claim identifying this cluster and clusterset must exist before federation can be created.
	clusterClaims := &ClusterClaimList{}
	err = client.List(context.TODO(), clusterClaims, controllerclient.InNamespace(config.AntreaSystemNamespace))
	if err != nil {
		return err
	}
	wellKnownClusterClaimIDExists := false
	wellKnownClusterClaimClusterSetExists := false
	for _, clusterClaim := range clusterClaims.Items {
		if strings.Compare(clusterClaim.Spec.Name, string(WellKnownClusterClaimID)) == 0 {
			wellKnownClusterClaimIDExists = true
		}
		if strings.Compare(clusterClaim.Spec.Name, string(WellKnownClusterClaimClusterSet)) == 0 {
			wellKnownClusterClaimClusterSetExists = true
		}
	}
	if !wellKnownClusterClaimIDExists {
		return fmt.Errorf("clusterclaim identifying cluster using WellKnownClusterClaim %v must exist. "+
			"federation %v will not be created", WellKnownClusterClaimID, r.Name)
	}
	if !wellKnownClusterClaimClusterSetExists {
		return fmt.Errorf("clusterclaim identifying clusterset using WellKnownClusterClaim %v must exist. "+
			"federation %v will not be created", WellKnownClusterClaimClusterSet, r.Name)
	}

	// federation must be singleton per cluster in antrea namespace.
	federationList := &FederationList{}
	err = client.List(context.TODO(), federationList, controllerclient.InNamespace(config.AntreaSystemNamespace))
	if err != nil {
		return err
	}
	if len(federationList.Items) != 0 {
		federation := federationList.Items[0]
		return fmt.Errorf("federation %v already exists. federation is singleton per %v namespace. federation %v will not be created",
			federation.Name, config.AntreaSystemNamespace, r.Name)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Federation) ValidateUpdate(old runtime.Object) error {
	federationlog.Info("validate update", "name", r.Name)

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *Federation) ValidateDelete() error {
	federationlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
