/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package federation

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	federationv1alpha1 "github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
)

// ClusterClaimReconciler reconciles a ClusterClaim object.
type ClusterClaimReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mcs.crd.antrea.io,resources=clusterclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcs.crd.antrea.io,resources=clusterclaims/status,verbs=get;update;patch

func (r *ClusterClaimReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("clusterclaim", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *ClusterClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&federationv1alpha1.ClusterClaim{}).
		Complete(r)
}
