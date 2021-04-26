/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
)

// log is for logging in this package.
var (
	clusterclaimlog = logf.Log.WithName("clusterclaim-resource")
	ctrlClient      controllerclient.Client
)

func (r *ClusterClaim) SetupWebhookWithManager(mgr ctrl.Manager) error {
	ctrlClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// nolint:lll
// +kubebuilder:webhook:path=/mutate-federation-antrea-io-v1alpha1-clusterclaim,mutating=true,failurePolicy=fail,sideEffects=None,groups=mcs.crd.antrea.io,resources=clusterclaims,verbs=create;update,versions=v1alpha1,name=mclusterclaim.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &ClusterClaim{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *ClusterClaim) Default() {
	clusterclaimlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// nolint:lll
// +kubebuilder:webhook:verbs=create;update,path=/validate-federation-antrea-io-v1alpha1-clusterclaim,mutating=false,failurePolicy=fail,sideEffects=None,groups=mcs.crd.antrea.io,resources=clusterclaims,versions=v1alpha1,name=vclusterclaim.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &ClusterClaim{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterClaim) ValidateCreate() error {
	clusterclaimlog.Info("validate create", "name", r.Name)
	var err error

	// well known claim identifying the cluster should be singleton for the lifespan of a cluster.
	// well known claim identifying the clusterset should be singleton for the lifespan of a cluster.
	claimName := strings.TrimSpace(r.Spec.Name)
	if strings.Compare(claimName, string(WellKnownClusterClaimID)) == 0 ||
		strings.Compare(claimName, string(WellKnownClusterClaimClusterSet)) == 0 {
		claimList := &ClusterClaimList{}
		err = ctrlClient.List(context.TODO(), claimList, controllerclient.InNamespace(config.AntreaSystemNamespace))
		if err != nil {
			return err
		}
		for _, claim := range claimList.Items {
			if strings.Compare(claimName, claim.Spec.Name) == 0 {
				return fmt.Errorf("well known clusterclaim %v already exists for the cluster", claimName)
			}
		}
	}

	// validate claim value (follows dsn1123 convention)
	errs := validation.IsValidLabelValue(r.Spec.Value)
	if len(errs) != 0 {
		return fmt.Errorf("claim Spec.value validation failed (%v)", errs)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterClaim) ValidateUpdate(old runtime.Object) error {
	clusterclaimlog.Info("validate update", "name", r.Name)

	claimName := r.Spec.Name

	// well known claim identifying the cluster will be immutable for the duration of a cluster
	// well known claim identifying the clusterset will be immutable for the duration a cluster
	if strings.Compare(claimName, string(WellKnownClusterClaimID)) == 0 ||
		strings.Compare(claimName, string(WellKnownClusterClaimClusterSet)) == 0 {
		return fmt.Errorf("clusterclaims identifying the cluster and identifying clusterset cannot be" +
			"updated (delete and recreate only allowed)")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ClusterClaim) ValidateDelete() error {
	clusterclaimlog.Info("validate delete", "name", r.Name)

	var err error

	claimName := r.Spec.Name
	clusterClaims := &ClusterClaimList{}
	err = client.List(context.TODO(), clusterClaims, controllerclient.InNamespace(config.AntreaSystemNamespace))
	if err != nil {
		return err
	}
	wellKnownClusterClaimSetIDExists := false
	for _, clusterClaim := range clusterClaims.Items {
		if strings.Compare(clusterClaim.Spec.Name, string(WellKnownClusterClaimClusterSet)) == 0 {
			wellKnownClusterClaimSetIDExists = true
			break
		}
	}

	// if clusterSetId exists, clusterId cannot be deleted.
	if strings.Compare(claimName, string(WellKnownClusterClaimID)) == 0 && wellKnownClusterClaimSetIDExists {
		return fmt.Errorf("clusterclaim identifying the cluster cannot be" +
			"updated if clusterset claim exist ()")
	}

	federationList := &FederationList{}
	err = client.List(context.TODO(), federationList, controllerclient.InNamespace(config.AntreaSystemNamespace))
	if err != nil {
		return err
	}
	if len(federationList.Items) == 0 {
		return nil
	}

	// if federation configured on this Cluster, clusterID and clusterSetID cannot be deleted.
	// well known claim identifying the cluster must be immutable for the duration
	// of a cluster’s membership in a ClusterSet
	// well known claim identifying the clusterset must be immutable for the duration
	// of a cluster’s membership in a ClusterSet
	if strings.Compare(claimName, string(WellKnownClusterClaimID)) == 0 ||
		strings.Compare(claimName, string(WellKnownClusterClaimClusterSet)) == 0 {
		return fmt.Errorf("clusterclaims identifying the cluster and identifying clusterset cannot be" +
			"deleted if cluster is member of federation")
	}

	return nil
}
