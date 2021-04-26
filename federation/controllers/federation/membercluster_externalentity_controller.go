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
	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

type MemberClusterExternalEntityReconciler struct {
	ClusterClient *memberClusterManager
	Log           logr.Logger
	Scheme        *runtime.Scheme
}

func (r *MemberClusterExternalEntityReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("member-cluster-resources", req.NamespacedName)

	externalEntity := &antreacore.ExternalEntity{}
	err := r.ClusterClient.Get(ctx, req.NamespacedName, externalEntity)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	r.Log.Info("received ExternalEntity", "member-cluster-id", r.ClusterClient.GetClusterID(), "name", req.NamespacedName)

	if errors.IsNotFound(err) {
		externalEntity.Name = req.Name
		externalEntity.Namespace = req.Namespace
	}

	resource := clusterclient.Resource{
		Object:  externalEntity,
		Cluster: r.ClusterClient.GetClusterID(),
	}
	r.ClusterClient.AddToResourceEventQueue(resource)

	return ctrl.Result{}, nil
}

func (r *MemberClusterExternalEntityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&antreacore.ExternalEntity{}).
		Complete(r)
}
