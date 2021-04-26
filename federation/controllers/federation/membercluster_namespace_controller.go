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

type MemberClusterClusterNamespaceReconciler struct {
	ClusterClient *memberClusterManager
	Log           logr.Logger
	Scheme        *runtime.Scheme
}

func (r *MemberClusterClusterNamespaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("member-cluster-resources", req.NamespacedName)

	ns := &v1.Namespace{}
	err := r.ClusterClient.Get(ctx, req.NamespacedName, ns)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	r.Log.Info("received Namespace", "member-cluster-id", r.ClusterClient.GetClusterID(), "name", req.NamespacedName)

	if errors.IsNotFound(err) {
		ns.Name = req.Name
	}

	resource := clusterclient.Resource{
		Object:  ns,
		Cluster: r.ClusterClient.GetClusterID(),
	}
	r.ClusterClient.AddToResourceEventQueue(resource)

	return ctrl.Result{}, nil
}

func (r *MemberClusterClusterNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Namespace{}).
		Complete(r)
}
