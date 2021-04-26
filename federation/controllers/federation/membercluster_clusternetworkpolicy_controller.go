/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package federation

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
)

type MemberClusterClusterNetworkPolicyReconciler struct {
	ClusterClient *memberClusterManager
	Log           logr.Logger
	Scheme        *runtime.Scheme
}

func (r *MemberClusterClusterNetworkPolicyReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("member-cluster-resources", req.NamespacedName)

	clusterNetworkPolicy := &antreanetworking.ClusterNetworkPolicy{}
	err := r.ClusterClient.Get(ctx, req.NamespacedName, clusterNetworkPolicy)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if err == nil {
		// Skip non federated resource on create/update. But not on delete as Annotation context is lost.
		r := clusterclient.Resource{Object: clusterNetworkPolicy}
		if r.UserDefinedFederatedKey() == nil {
			return ctrl.Result{}, nil
		}
	}

	r.Log.Info("received ClusterNetworkPolicy", "member-cluster-id", r.ClusterClient.GetClusterID(), "name", req.NamespacedName)

	if errors.IsNotFound(err) {
		clusterNetworkPolicy.Name = req.Name
		clusterNetworkPolicy.Namespace = req.Namespace
	}

	resource := clusterclient.Resource{
		Object:  clusterNetworkPolicy,
		Cluster: r.ClusterClient.GetClusterID(),
	}
	r.ClusterClient.AddToResourceEventQueue(resource)

	return ctrl.Result{}, nil
}

func (r *MemberClusterClusterNetworkPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&antreanetworking.ClusterNetworkPolicy{}).
		Complete(r)
}
