/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package federation

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
)

type MemberClusterNetworkPolicyReconciler struct {
	ClusterClient *memberClusterManager
	Log           logr.Logger
	Scheme        *runtime.Scheme
}

func (r *MemberClusterNetworkPolicyReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("member-cluster-resources", req.NamespacedName)

	networkPolicy := &antreanetworking.NetworkPolicy{}
	err := r.ClusterClient.Get(ctx, req.NamespacedName, networkPolicy)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if err == nil {
		// Consider NetworkPolicy without federation same as deletion of federated NetworkPolicy.
		r := clusterclient.Resource{Object: networkPolicy}
		if r.UserDefinedFederatedKey() == nil {
			networkPolicy = &antreanetworking.NetworkPolicy{}
			err = errors.NewNotFound(schema.GroupResource{}, req.Name)
		}
	}
	r.Log.Info("received NetworkPolicy", "member-cluster-id", r.ClusterClient.GetClusterID(), "name", req.NamespacedName)
	if errors.IsNotFound(err) {
		networkPolicy.Name = req.Name
		networkPolicy.Namespace = req.Namespace
	}

	resource := clusterclient.Resource{
		Object:  networkPolicy,
		Cluster: r.ClusterClient.GetClusterID(),
	}
	r.ClusterClient.AddToResourceEventQueue(resource)

	return ctrl.Result{}, nil
}

func (r *MemberClusterNetworkPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&antreanetworking.NetworkPolicy{}).
		Complete(r)
}
