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
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
)

type MemberClusterServiceReconciler struct {
	ClusterClient *memberClusterManager
	Log           logr.Logger
	Scheme        *runtime.Scheme
}

func (r *MemberClusterServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("member-cluster-resources", req.NamespacedName)

	service := &v1.Service{}
	err := r.ClusterClient.Get(ctx, req.NamespacedName, service)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	r.Log.Info("received Service", "member-cluster-id", r.ClusterClient.GetClusterID(), "name", req.NamespacedName)
	if err == nil {
		// Consider Service without federation annotation same as remove Service (from Federated Service).
		r := clusterclient.Resource{Object: service}
		if r.UserDefinedFederatedKey() == nil && len(r.FederationID()) == 0 {
			service = &v1.Service{}
			err = errors.NewNotFound(schema.GroupResource{}, req.Name)
		}
	}

	if errors.IsNotFound(err) {
		service.Name = req.Name
		service.Namespace = req.Namespace
	}

	resource := clusterclient.Resource{
		Object:  service,
		Cluster: r.ClusterClient.GetClusterID(),
	}
	r.ClusterClient.AddToResourceEventQueue(resource)

	return ctrl.Result{}, nil
}

func (r *MemberClusterServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
		Complete(r)
}
