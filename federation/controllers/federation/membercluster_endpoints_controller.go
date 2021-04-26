/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package federation

import (
	"context"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
)

type MemberClusterEndpointsReconciler struct {
	ClusterClient *memberClusterManager
	Log           logr.Logger
	Scheme        *runtime.Scheme
}

func (r *MemberClusterEndpointsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("member-cluster-resources", req.NamespacedName)

	endpoints := &v1.Endpoints{}
	err := r.ClusterClient.Get(ctx, req.NamespacedName, endpoints)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// TODO Endpoints are one of the busiest Resources, can we do better and filter some Endpoints here?
	// r.Log.Info("received Endpoints", "member-cluster-id", r.ClusterClient.GetClusterID(), "name", req.NamespacedName)

	if errors.IsNotFound(err) {
		endpoints.Name = req.Name
		endpoints.Namespace = req.Namespace
	}

	resource := clusterclient.Resource{
		Object:  endpoints,
		Cluster: r.ClusterClient.GetClusterID(),
	}
	r.ClusterClient.AddToResourceEventQueue(resource)

	return ctrl.Result{}, nil
}

func (r *MemberClusterEndpointsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Endpoints{}).
		Complete(r)
}
