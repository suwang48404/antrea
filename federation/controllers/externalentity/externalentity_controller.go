/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package externalentity

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/webhook"
	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

const (

	// K8s label requirements, https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	LabelSizeLimit  = 64
	LabelExpression = "[^a-zA-Z0-9_-]+"
)

var (
	externalEntityOpRetryInterval = time.Second * 5
)

// ExternalEntityOwner specifies common interface to access an owner resource of ExternalEntity.
type ExternalEntityOwner interface {
	runtime.Object
	// GetEndPointAddresses returns IP addresses of ExternalEntityOwner.
	// Passing client in case there are references needs to be retrieved from local cache.
	GetEndPointAddresses(client client.Client) ([]string, error)
	// GetEndPointPort returns port and port name, if applicable, of ExternalEntityOwner.
	GetEndPointPort(client client.Client) []antreatypes.NamedPort
	// GetTags returns tags of ExternalEntityOwner
	GetTags() map[string]string
	// GetLabels returns labels specific to ExternalEntityOwner.
	GetLabels(client client.Client) map[string]string
	// GetExternalNode returns external node/controller associated with this ExternalEntityOwner.
	GetExternalNode(client client.Client) string
	// Copy return a duplicate of current ExternalEntityOwner
	Copy() ExternalEntityOwner
	// EmbedType returns the underlying ExternalEntity owner resource.
	EmbedType() runtime.Object
	// IsFedResource is true if only reconcile federated resources.
	IsFedResource() bool
}

// Dependent is used to uniquely identify a dependent.
type Dependent struct {
	OwnerOf   string
	Name      string
	Namespace string
}

// DependedExternalEntityOwner specifies interface retrieve ExternalEntityOwner dependents.
type DependedExternalEntityOwner interface {
	//  SetIndexer let owner to set indexer if any.
	SetIndexer(mgr ctrl.Manager) error
	// GetDependents returns dependents of owner.
	GetDependents(client client.Client) []Dependent
}

var (
	RegisteredExternalEntityOwners = make(map[string]*ExternalEntityOwnerReconciler)
)

// RegisterExternalEntityOwner register an owner resource with externalentity controller.
func RegisterExternalEntityOwner(name string, owner ExternalEntityOwner, otherOwns ...runtime.Object) {
	RegisteredExternalEntityOwners[name] = &ExternalEntityOwnerReconciler{
		EntityOwner: owner,
		Owns:        otherOwns,
		Name:        name,
	}
	// Update webhook with registered owner.
	webhook.RegisteredOwner[reflect.TypeOf(owner.EmbedType()).Elem().Name()] = struct{}{}
}

// GetExternalEntityLabelKind returns value of ExternalEntity kind label.
func GetExternalEntityLabelKind(obj runtime.Object) string {
	return strings.ToLower(reflect.TypeOf(obj).Elem().Name())
}

func GetExternalEntityName(obj runtime.Object) string {
	access, _ := meta.Accessor(obj)
	return strings.ToLower(reflect.TypeOf(obj).Elem().Name()) + "-" + access.GetName()
}

func getObjectKeyFromOwner(owner ExternalEntityOwner) client.ObjectKey {
	access, _ := meta.Accessor(owner)
	return client.ObjectKey{Namespace: access.GetNamespace(),
		Name: GetExternalEntityName(owner.EmbedType())}
}

// ExternalEntityReconciler reconciles a ExternalEntity object.
type ExternalEntityReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// nolint:lll
// +kubebuilder:rbac:groups=crd.antrea.io,resources=externalentities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.antrea.io,resources=externalentities/status,verbs=get;update;patch

func (r *ExternalEntityReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("externalentity", req.NamespacedName)

	// your logic here
	return ctrl.Result{}, nil
}

// SetupWithManager registers ExternalEntityReconciler itself with manager.
// It also registers ExternalEntityOwnerReconciler with manager for each registered owner,
// and setting up channels to listen to owner resource events.
func (r *ExternalEntityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.V(1).Info("Registered owners", "Number", len(RegisteredExternalEntityOwners))
	for name, ownerReconciler := range RegisteredExternalEntityOwners {
		r.Log.Info("Setup externalentityowner_controller for ", "EntityOwner", name)
		// Setup owner reconciler.
		ownerReconciler.Log = r.Log.WithName(name + "ExternalEntity")
		ch := make(chan ExternalEntityOwner)
		ownerReconciler.Client = r.Client
		ownerReconciler.Ch = ch
		if mgr != nil {
			if err := ownerReconciler.SetupWithManager(mgr); err != nil {
				r.Log.Error(err, "Unable to setup externalentityowner_controller for owner", "EntityOwner", name)
				return err
			}
			if depOwner, ok := ownerReconciler.EntityOwner.(DependedExternalEntityOwner); ok {
				if err := depOwner.SetIndexer(mgr); err != nil {
					r.Log.Error(err, "Unable to setup indexer for dependedexternalentityowner", "EntityOwner", name)
					return err
				}
			}
		}
		go func(n string, c <-chan ExternalEntityOwner) {
			r.Log.Info("Listening to owner", "EntityOwner", n)
			var recv ExternalEntityOwner
			failedUpdates := make(map[string]ExternalEntityOwner)
			retryChan := make(chan ExternalEntityOwner)
			for {
				select {
				case recv = <-c:
					r.processOwnerEvent(recv, failedUpdates, retryChan, false)
				case recv = <-retryChan:
					r.processOwnerEvent(recv, failedUpdates, retryChan, true)
				}
			}
		}(name, ch)
	}
	return nil
}

// processOwnerEvent processes Create/Update/Delete events of owner resources forwarded from externalentity_owner_controllers.
// And create, update, delete corresponding ExternalEntity according.
func (r *ExternalEntityReconciler) processOwnerEvent(owner ExternalEntityOwner, failedUpdates map[string]ExternalEntityOwner,
	retryChan chan<- ExternalEntityOwner, isRetry bool) {
	var err error
	log := r.Log.WithName("processOwnerEvent")
	fetchKey := getObjectKeyFromOwner(owner)

	log.Info("Received owner event", "FetchKey", fetchKey, "isRetry", isRetry)
	if isRetry {
		prevOwner, ok := failedUpdates[fetchKey.String()]
		isOldInst := false
		if ok {
			// Handle multiple failures from different owner updates.
			// Only the latest update counts.
			acc1, _ := meta.Accessor(owner)
			acc2, _ := meta.Accessor(prevOwner)
			if acc1.GetGeneration() != acc2.GetGeneration() {
				isOldInst = true
			}
		}
		if !ok || isOldInst {
			log.Info("Ignore retry", "Key", fetchKey)
			return
		}
	}

	// Retry logic after processing
	defer func() {
		if err == nil {
			delete(failedUpdates, fetchKey.String())
			return
		}
		failedUpdates[fetchKey.String()] = owner
		time.AfterFunc(externalEntityOpRetryInterval, func() {
			retryChan <- owner
		})
	}()

	ctx := context.Background()
	ips, err := owner.GetEndPointAddresses(r.Client)
	if err != nil {
		log.Error(err, "Failed to get IP address for", "Name", fetchKey)
		return
	}

	if depOwner, ok := owner.(DependedExternalEntityOwner); ok {
		r.processDependents(depOwner)
	}

	isDelete := len(ips) == 0
	externEntity := &antreatypes.ExternalEntity{}
	isNotFound := false
	err = r.Client.Get(ctx, fetchKey, externEntity)
	if err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			log.Error(err, "Unable to fetch ", "Key", fetchKey)
			return
		}
		isNotFound = true
	}

	// No-op
	if isDelete && isNotFound {
		log.V(1).Info("Deleting non-existing resource", "Key", fetchKey)
		return
	}

	// Delete
	if isDelete && !isNotFound {
		webhook.ExternalEntityWebhookCache.Delete(externEntity)
		err = r.Client.Delete(ctx, externEntity)
		err = client.IgnoreNotFound(err)
		if err != nil {
			log.Error(err, "Unable to delete ", "Key", fetchKey)
		} else {
			log.V(1).Info("Deleted resource", "Key", fetchKey)
		}
		return
	}

	// Update
	if !isNotFound {
		base := client.MergeFrom(externEntity.DeepCopy())
		patch := r.patchExternalEntityFrom(owner, externEntity)
		webhook.ExternalEntityWebhookCache.Update(patch)
		if err = r.Client.Patch(ctx, patch, base); err != nil {
			log.Error(err, "Unable to patch ", "Key", fetchKey)
		} else {
			log.V(1).Info("Patched resource", "Key", fetchKey)
		}
		return
	}

	// Create
	externEntity = r.newExternalEntityFrom(owner, fetchKey.Name, fetchKey.Namespace)
	webhook.ExternalEntityWebhookCache.Update(externEntity)
	if err = r.Client.Create(ctx, externEntity); err != nil {
		log.Error(err, "Unable to create ", "Key", fetchKey)
	} else {
		log.V(1).Info("Created resource", "Key", fetchKey)
	}
}

// processDependents processes dependents of an ExternalEntity owner.
func (r *ExternalEntityReconciler) processDependents(owner DependedExternalEntityOwner) {
	r.Log.V(1).Info("Process dependents for ", "owner", getObjectKeyFromOwner(owner.(ExternalEntityOwner)))
	for _, c := range owner.GetDependents(r.Client) {
		reconciler := RegisteredExternalEntityOwners[c.OwnerOf]
		obj := reconciler.EntityOwner.Copy()
		if err := r.Get(context.TODO(), client.ObjectKey{Name: c.Name, Namespace: c.Namespace}, obj.EmbedType()); err != nil {
			r.Log.Error(err, "Get dependent object", "OwnerOf", c.OwnerOf, "Name", c.Name, "Namespace", c.Namespace)
			continue
		}
		reconciler.Ch <- obj
	}
}

// newExternalEntityFrom generate an new ExternalEntity from owner.
func (r *ExternalEntityReconciler) newExternalEntityFrom(owner ExternalEntityOwner, name, namespace string) *antreatypes.ExternalEntity {
	externEntity := &antreatypes.ExternalEntity{}
	r.populateExternalEntityFrom(owner, externEntity)
	externEntity.SetName(name)
	externEntity.SetNamespace(namespace)
	accessor, err := meta.Accessor(owner.EmbedType())
	if err != nil {
		r.Log.Error(err, "Failed to get accessor", "owner", owner)
	}
	if err = ctrl.SetControllerReference(accessor, externEntity, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set owner reference",
			"Namespace", accessor.GetNamespace(), "name", accessor.GetName())
	}
	return externEntity
}

// patchExternalEntityFrom generate a patch for existing ExternalEntity from owner.
func (r *ExternalEntityReconciler) patchExternalEntityFrom(
	owner ExternalEntityOwner, patch *antreatypes.ExternalEntity) *antreatypes.ExternalEntity {
	r.populateExternalEntityFrom(owner, patch)
	return patch
}

func (r *ExternalEntityReconciler) populateExternalEntityFrom(owner ExternalEntityOwner, externEntity *antreatypes.ExternalEntity) {
	labels := make(map[string]string)
	accessor, _ := meta.Accessor(owner)
	labels[config.ExternalEntityLabelKeyKind] = GetExternalEntityLabelKind(owner.EmbedType())
	labels[config.ExternalEntityLabelKeyName] = strings.ToLower(accessor.GetName())
	labels[config.ExternalEntityLabelKeyNamespace] = strings.ToLower(accessor.GetNamespace())
	for key, val := range owner.GetLabels(r.Client) {
		labels[key] = val
	}
	for key, val := range owner.GetTags() {
		reg, _ := regexp.Compile(LabelExpression)
		fkey := reg.ReplaceAllString(key, "") + config.ExternalEntityLabelKeyTagPostfix
		if len(fkey) > LabelSizeLimit {
			fkey = fkey[:LabelSizeLimit]
		}
		fval := reg.ReplaceAllString(val, "")
		if len(fval) > LabelSizeLimit {
			fval = fval[:LabelSizeLimit]
		}
		labels[strings.ToLower(fkey)] = strings.ToLower(fval)
	}
	externEntity.SetLabels(labels)

	ipAddrs, _ := owner.GetEndPointAddresses(r.Client)
	endpoints := make([]antreatypes.Endpoint, 0, len(ipAddrs))
	for _, ip := range ipAddrs {
		endpoints = append(endpoints, antreatypes.Endpoint{IP: ip})
	}
	externEntity.Spec.Endpoints = endpoints
	externEntity.Spec.Ports = owner.GetEndPointPort(r.Client)
	externEntity.Spec.ExternalNode = owner.GetExternalNode(r.Client)
}
