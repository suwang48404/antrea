/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package externalentity

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

// ExternalEntityOwnerReconciler reconciles a ExternalEntity owner resource.
type ExternalEntityOwnerReconciler struct {
	// Client to APIServer
	client.Client
	// Log is used for logging.
	Log logr.Logger
	// EntityOwner specifies an owner resource of ExternalEntity resource.
	EntityOwner ExternalEntityOwner
	// Owns specifies other resources may be owned by EntityOwner.
	Owns []runtime.Object
	// Ch is used to notify ExternalEntityReconciler of owner resource changes.
	Ch   chan<- ExternalEntityOwner
	Name string
}

// Reconcile forwards any changes of an owner resource to ExternalEntityReconciler,
// allowing any changes to owner resource to propagate to generated ExternalEntity resource.
func (r *ExternalEntityOwnerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Name", r.Name, "ExternalEntityOwner", req.NamespacedName)

	owner := r.EntityOwner.Copy()
	if err := r.Get(context.TODO(), req.NamespacedName, owner.EmbedType()); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Unable to fetch")
			return ctrl.Result{}, err
		}
		// Fall through if owner is deleted.
		log.V(1).Info("Is delete")
		accessor, _ := meta.Accessor(owner)
		accessor.SetName(req.Name)
		accessor.SetNamespace(req.Namespace)
		if owner.IsFedResource() {
			// Setup dummy federationID if ExternalEntity needs to be removed.
			eeName := GetExternalEntityName(owner.EmbedType())
			ee := &antreacore.ExternalEntity{}
			if err := r.Get(context.Background(), client.ObjectKey{
				Namespace: req.Namespace,
				Name:      eeName,
			}, ee); err == nil {
				accessor.SetAnnotations(map[string]string{config.FederationIDAnnotation: "dummy"})
			}
		}
	}
	if owner.IsFedResource() {
		if len((clusterclient.Resource{Object: owner.EmbedType()}).FederationID()) == 0 {
			return ctrl.Result{}, nil
		}
	}
	log.V(1).Info("Forwarded to externalEntity controller")
	r.Ch <- owner
	return ctrl.Result{}, nil
}

// SetupWithManager registers with manager.
func (r *ExternalEntityOwnerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).For(r.EntityOwner.EmbedType())
	for _, owns := range r.Owns {
		builder = builder.Owns(owns)
	}
	return builder.Complete(r)
}
