/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package federation

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	federationv1alpha1 "github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/internal/fedmanager"
)

// FedReconciler reconciles a Federation object.
type FedReconciler struct {
	client.Client
	mutex              sync.Mutex
	Log                logr.Logger
	Scheme             *runtime.Scheme
	federationManager  fedmanager.FederationManager
	fedMgrStopChannel  chan struct{}
	fedConfig          *federationv1alpha1.Federation
	needReconcile      bool
	federationID       clusterclient.FederationID
	failedToAddMembers map[clusterclient.ClusterID]memberAddResult
}

type memberAddResult struct {
	clusterID     clusterclient.ClusterID
	clusterClient clusterclient.ClusterClient
	err           error
}

// +kubebuilder:rbac:groups=mcs.crd.antrea.io,resources=federations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcs.crd.antrea.io,resources=federations/status,verbs=get;update;patch

func (r *FedReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("federation", req.NamespacedName)

	federation := &federationv1alpha1.Federation{}
	err := r.Get(ctx, req.NamespacedName, federation)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if errors.IsNotFound(err) {
		r.processDelete()
		return ctrl.Result{}, nil
	}

	err = r.processCreateOrUpdate(federation)
	return ctrl.Result{}, err
}

func (r *FedReconciler) setReconcile() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.needReconcile = true
}

func (r *FedReconciler) processCreateOrUpdate(federation *federationv1alpha1.Federation) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var err error

	if federation != nil {
		// Update cached member clusters.
		r.fedConfig = federation.DeepCopy()

		// Return error in following cases (so that it gets retried):
		//     - if clusterset claim ID is not configured.
		clusterClaimList := &federationv1alpha1.ClusterClaimList{}
		err = r.List(context.TODO(), clusterClaimList, client.InNamespace(config.AntreaSystemNamespace))
		if err != nil {
			return err
		}
		if len(clusterClaimList.Items) == 0 {
			return fmt.Errorf("clusterset claim ID not configured for the cluster")
		}

		var clustersetClaimID string
		wellKnownClusterSetClaimIDExist := false
		for _, clusterClaim := range clusterClaimList.Items {
			if strings.Compare(clusterClaim.Spec.Name, federationv1alpha1.WellKnownClusterClaimClusterSet) == 0 {
				wellKnownClusterSetClaimIDExist = true
				clustersetClaimID = clusterClaim.Spec.Value
				break
			}
		}

		if !wellKnownClusterSetClaimIDExist {
			return fmt.Errorf("clusterset claim ID not configured for the cluster")
		}

		r.federationID = clusterclient.FederationID(clustersetClaimID)
		if r.federationManager == nil {
			r.federationManager = fedmanager.NewFederationManager(r.federationID, r.Log)
			r.fedMgrStopChannel = make(chan struct{})
			go func() {
				r.Log.Info("starting federation manager", "federation-id", r.federationID)
				err = r.federationManager.Start(r.fedMgrStopChannel)
				// will reach here only when federation manager exits for any reason
				r.Log.Info("federation manager exited", "federation-id", r.federationID, "error", err)
				r.mutex.Lock()
				r.federationManager = nil
				r.fedMgrStopChannel = nil
				r.mutex.Unlock()
			}()
			r.Log.Info("federation-manager started", "federation-id", r.federationID)
		} else {
			r.Log.Info("federation-manager exists", "federation-id", r.federationID)
		}
	} else if !r.needReconcile {
		return nil
	}
	r.needReconcile = false
	if r.federationManager == nil {
		return nil
	}
	// process members add/remove
	membersToAdd, membersToRemove := findMembersToAddRemove(r.fedConfig.Spec.Members, r.federationManager.GetMemberClusters())
	r.Log.Info("federation member processing", "toAdd", len(membersToAdd), "toRemove", len(membersToRemove))
	addErr := r.processMembersAdd(r.federationID, r.fedConfig, membersToAdd)
	err = multierr.Append(err, addErr)
	removeErr := r.processMembersRemove(membersToRemove)
	err = multierr.Append(err, removeErr)

	return err
}

func (r *FedReconciler) processDelete() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.federationManager != nil {
		r.fedMgrStopChannel <- struct{}{}
	}

	r.fedMgrStopChannel = nil
	r.federationManager = nil
}

func (r *FedReconciler) processMembersAdd(fedID clusterclient.FederationID, federation *federationv1alpha1.Federation,
	members []*federationv1alpha1.MemberCluster) error {
	ch := make(chan memberAddResult)
	var wg sync.WaitGroup

	wg.Add(len(members))
	go func() {
		wg.Wait()
		close(ch)
	}()

	fedNamespacedName := &types.NamespacedName{
		Namespace: federation.Namespace,
		Name:      federation.Name,
	}
	for _, member := range members {
		clusterID := clusterclient.ClusterID(member.ClusterID)
		url := member.Server
		secretName := member.Secret

		r.Log.Info("creating cluster manager", "member-cluster-id", clusterID)

		go func(clusterID clusterclient.ClusterID, url string, secretName string, ch chan<- memberAddResult) {
			defer wg.Done()

			clusterClient, err := NewMemberClusterManager(fedNamespacedName, fedID, clusterID, url, secretName,
				r.Scheme, r.Log.WithName(string(clusterID)), r)

			ch <- memberAddResult{
				clusterID:     clusterID,
				clusterClient: clusterClient,
				err:           err,
			}
		}(clusterID, url, secretName, ch)
	}

	var err error
	for addResult := range ch {
		clusterID := addResult.clusterID

		if addResult.err != nil {
			r.Log.Error(addResult.err, "failed to create cluster", "member-cluster-id", addResult.clusterID)
			err = multierr.Append(err, addResult.err)

			r.failedToAddMembers[clusterID] = addResult
		} else {
			addErr := r.federationManager.AddClusterClient(addResult.clusterClient)
			if addErr != nil {
				r.Log.Error(addResult.err, "failed to add cluster", "member-cluster-id", addResult.clusterID)
				err = multierr.Append(err, addErr)

				r.failedToAddMembers[clusterID] = addResult
			} else {
				r.Log.Info("cluster added successfully", "member-cluster-id", addResult.clusterID)

				// remove from failed members if it exists
				delete(r.failedToAddMembers, clusterID)
			}
		}
	}

	return err
}

func (r *FedReconciler) processMembersRemove(members map[clusterclient.ClusterID]clusterclient.ClusterClient) error {
	var err error
	for _, memberClient := range members {
		removeErr := r.federationManager.RemoveClusterClient(memberClient)
		if removeErr != nil {
			err = multierr.Append(err, removeErr)
		}
	}
	return err
}

func findMembersToAddRemove(newMembers []federationv1alpha1.MemberCluster,
	oldMembers map[clusterclient.ClusterID]clusterclient.ClusterClient) ([]*federationv1alpha1.MemberCluster,
	map[clusterclient.ClusterID]clusterclient.ClusterClient) {
	var membersToAdd []*federationv1alpha1.MemberCluster

	for _, member := range newMembers {
		memberID := clusterclient.ClusterID(member.ClusterID)
		_, found := oldMembers[memberID]
		if !found {
			membersToAdd = append(membersToAdd, member.DeepCopy())
		} else {
			// oldMembers in the end will only have members to be removed.
			delete(oldMembers, memberID)
		}
	}

	return membersToAdd, oldMembers
}

func (r *FedReconciler) processFederationStatusUpdate() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.federationManager == nil {
		return nil
	}

	var clusterStatuses []federationv1alpha1.MemberClusterStatus

	currentMembers := r.fedConfig.Spec.Members
	memberClustersMap := r.federationManager.GetMemberClusters()

	for _, member := range currentMembers {
		clusterID := clusterclient.ClusterID(member.ClusterID)
		memberClient, found := memberClustersMap[clusterID]
		if found {
			clusterStatus := *memberClient.GetClusterStatus()
			clusterStatuses = append(clusterStatuses, clusterStatus)
			continue
		}

		failedMember, found := r.failedToAddMembers[clusterID]
		if found {
			clusterStatus := federationv1alpha1.MemberClusterStatus{
				ClusterID: string(failedMember.clusterID),
				IsLeader:  false,
				Status:    Unreachable,
				Error:     failedMember.err.Error(),
			}
			clusterStatuses = append(clusterStatuses, clusterStatus)
		}
	}

	fedNamespacedName := types.NamespacedName{
		Namespace: r.fedConfig.Namespace,
		Name:      r.fedConfig.Name,
	}
	federation := &federationv1alpha1.Federation{}
	err := r.Get(context.TODO(), fedNamespacedName, federation)
	if err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "failed to read federation object", "name", fedNamespacedName)
		return err
	}
	if errors.IsNotFound(err) {
		r.Log.Info("federation object does not exist", "name", fedNamespacedName)
		return err
	}

	federation.Status.MemberStatus = clusterStatuses

	err = r.Status().Update(context.TODO(), federation)
	if err != nil {
		r.Log.Error(err, "failed to update federation object status", "name", fedNamespacedName)
		return err
	}
	return nil
}

func (r *FedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.failedToAddMembers = make(map[clusterclient.ClusterID]memberAddResult)

	go func() {
		// Running background task here.
		for {
			<-time.After(time.Second * 5)
			// Check any inconsistency between member cluster
			// configuration and realization.
			_ = r.processCreateOrUpdate(nil)

			_ = r.processFederationStatusUpdate()
		}
	}()
	return ctrl.NewControllerManagedBy(mgr).
		For(&federationv1alpha1.Federation{}).
		Complete(r)
}
