/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package federation

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log2 "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	cfg "github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
)

var (
	// Use a raw client to get member cluster secrets from antrea-plus-system only.
	secretCient client.Client
	mutex       sync.Mutex
)

type memberClusterManager struct {
	client.Client
	mutex          sync.Mutex
	log            logr.Logger
	clusterManager manager.Manager
	// electionManager            manager.Manager
	clusterID                  clusterclient.ClusterID
	federationID               clusterclient.FederationID
	leaderEventQueue           workqueue.RateLimitingInterface
	resourceEventQueue         workqueue.RateLimitingInterface
	leaderElectionStopChannel  chan struct{}
	resourceMonitorStopChannel chan struct{}
	config                     *rest.Config
	scheme                     *runtime.Scheme
	fedNamespacedName          *types.NamespacedName
	fedReconciler              *FedReconciler
	status                     *v1alpha1.MemberClusterStatus
}

const (
	Unreachable = v1alpha1.ClusterStatusUnreachable
	Reachable   = v1alpha1.ClusterStatusReachable
)

func NewMemberClusterManager(fedNamespacedName *types.NamespacedName, federationID clusterclient.FederationID,
	clusterID clusterclient.ClusterID, url string, secretName string, scheme *runtime.Scheme, log logr.Logger,
	fc *FedReconciler) (clusterclient.ClusterClient, error) {
	log = log.WithName("member-cluster-manager-" + string(clusterID))
	log.V(1).Info("Create client to member cluster", "Cluster", clusterID)
	// read/decode secret (get token and crt)
	crtData, token, err := getSecretCACrtAndToken(secretName)
	if err != nil {
		return nil, err
	}

	// create manager for the member cluster
	config, err := clientcmd.BuildConfigFromFlags(url, "")
	if err != nil {
		return nil, err
	}
	config.BearerToken = string(token)
	config.CAData = crtData
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Logger:             log2.NullLogger{}, // log.WithName("Resource") to avoid excessive Endpoints logs.
	})
	if err != nil {
		return nil, err
	}

	// build member cluster client
	cluster := &memberClusterManager{
		Client:            mgr.GetClient(),
		mutex:             sync.Mutex{},
		log:               log,
		clusterManager:    mgr,
		clusterID:         clusterID,
		federationID:      federationID,
		fedNamespacedName: fedNamespacedName,
		config:            config,
		fedReconciler:     fc,
		status: &v1alpha1.MemberClusterStatus{
			ClusterID: string(clusterID),
			IsLeader:  false,
			Status:    Reachable,
			Error:     "none",
		},
	}

	// validate that member cluster can be part of this federation
	err = cluster.membershipValidation(config, scheme)
	if err != nil {
		return nil, err
	}
	// add controllers to the manager
	err = cluster.addMemberClusterControllers()
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

func getSecretCACrtAndToken(secretName string) ([]byte, []byte, error) {
	var err error
	mutex.Lock()
	if secretCient == nil {
		secretCient, err = client.New(ctrl.GetConfigOrDie(), client.Options{})
		if err != nil {
			mutex.Unlock()
			return nil, nil, err
		}
	}
	mutex.Unlock()
	secretObj := &v1.Secret{}
	secretNamespacedName := types.NamespacedName{
		Namespace: cfg.AntreaSystemNamespace,
		Name:      secretName,
	}
	err = secretCient.Get(context.TODO(), secretNamespacedName, secretObj)
	if err != nil {
		return nil, nil, err
	}

	caData, found := secretObj.Data[v1.ServiceAccountRootCAKey]
	if !found {
		return nil, nil, fmt.Errorf("ca.crt data not found in secret %v", secretName)
	}

	token, found := secretObj.Data[v1.ServiceAccountTokenKey]
	if !found {
		return nil, nil, fmt.Errorf("token not found in secret %v", secretName)
	}

	return caData, token, nil
}

func (c *memberClusterManager) GetFederationID() clusterclient.FederationID {
	return c.federationID
}

func (c *memberClusterManager) AddToLeaderEventQueue(event clusterclient.LeaderEvent) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.leaderEventQueue.Add(event)
}

func (c *memberClusterManager) AddToResourceEventQueue(event clusterclient.Resource) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.resourceEventQueue.Add(event)
}

func (c *memberClusterManager) membershipValidation(config *rest.Config, scheme *runtime.Scheme) error {
	var err error

	clusterClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	// make sure cluster has cluster ID configured and it matches with Federation configured cluster ID
	clusterClaims := &v1alpha1.ClusterClaimList{}
	err = clusterClient.List(context.TODO(), clusterClaims, client.InNamespace(cfg.AntreaSystemNamespace))
	if err != nil {
		return err
	}
	if len(clusterClaims.Items) == 0 {
		return fmt.Errorf("configured cluster ID not match (expected :%v, found:)", c.GetClusterID())
	}
	var clusterIDFromCluster string
	var federationIDFromCluster string
	for _, clusterClaim := range clusterClaims.Items {
		if strings.Compare(clusterClaim.Spec.Name, v1alpha1.WellKnownClusterClaimID) == 0 {
			clusterIDFromCluster = clusterClaim.Spec.Value
		}
		if strings.Compare(clusterClaim.Spec.Name, v1alpha1.WellKnownClusterClaimClusterSet) == 0 {
			federationIDFromCluster = clusterClaim.Spec.Value
		}
	}
	if strings.Compare(clusterIDFromCluster, string(c.GetClusterID())) != 0 {
		return fmt.Errorf("configured cluster ID not match (expected :%v, found : %v",
			c.GetClusterID(), clusterIDFromCluster)
	}

	// make sure cluster is part of this Federation
	if strings.Compare(federationIDFromCluster, string(c.GetFederationID())) != 0 {
		return fmt.Errorf("configured federation ID not match (expected :%v, found : %v",
			c.GetFederationID(), federationIDFromCluster)
	}

	return nil
}

func (c *memberClusterManager) addMemberClusterControllers() error {
	log := c.log
	mgr := c.clusterManager

	if err := (&MemberClusterServiceReconciler{
		ClusterClient: c,
		Log:           log.WithName("service-controller"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&MemberClusterEndpointsReconciler{
		ClusterClient: c,
		Log:           log.WithName("endpoints-controller"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&MemberClusterNodeReconciler{
		ClusterClient: c,
		Log:           log.WithName("node-controller"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&MemberClusterNetworkPolicyReconciler{
		ClusterClient: c,
		Log:           log.WithName("networkpolicy-controller"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&MemberClusterClusterNetworkPolicyReconciler{
		ClusterClient: c,
		Log:           log.WithName("clusternetworkpolicy-controller"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&MemberClusterExternalEntityReconciler{
		ClusterClient: c,
		Log:           log.WithName("externalentity-controller"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&MemberClusterClusterNamespaceReconciler{
		ClusterClient: c,
		Log:           log.WithName("namespace-controller"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}

func (c *memberClusterManager) updateClusterStatus(status v1alpha1.ClusterStatus, isLeader bool, err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.status.Status = status
	c.status.IsLeader = isLeader
	if err != nil {
		c.status.Error = err.Error()
	} else {
		c.status.Error = "none"
	}
}

// //////////////////////////////////////////////////////
// 	ClusterClient Implementation
// //////////////////////////////////////////////////////.
func (c *memberClusterManager) GetClusterID() clusterclient.ClusterID {
	return c.clusterID
}

func (c *memberClusterManager) GetClusterStatus() *v1alpha1.MemberClusterStatus {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.status.DeepCopy()
}

func (c *memberClusterManager) StartLeaderElection(queue workqueue.RateLimitingInterface) (chan<- struct{}, error) {
	c.log.Info("Starting leader election monitoring")
	mgr, err := ctrl.NewManager(c.config, ctrl.Options{
		LeaderElection:          true,
		LeaderElectionNamespace: cfg.AntreaSystemNamespace,
		LeaderElectionID:        string(c.clusterID),
		Scheme:                  c.scheme,
		MetricsBindAddress:      "0",
		Logger:                  c.log.WithName("Election"),
	})
	if err != nil {
		c.log.Info("Cannot create election manager", "Error", err)
		return nil, err
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// c.electionManager = mgr
	if c.leaderEventQueue == nil {
		c.leaderEventQueue = queue
	}
	stopElectionWait := make(chan struct{})
	go func(stop <-chan struct{}) {
		select {
		case <-mgr.Elected():
			c.AddToLeaderEventQueue(clusterclient.LeaderEvent{
				IsLeader: true,
				Cluster:  c.clusterID,
			})
			// update status
			c.updateClusterStatus(c.status.Status, true, err)

		case <-stop:
		}
	}(stopElectionWait)

	if c.leaderElectionStopChannel == nil {
		c.leaderElectionStopChannel = make(chan struct{})
	}
	go func() {
		err := mgr.Start(c.leaderElectionStopChannel)
		c.log.Info("Election exited", "Error", err)
		// update status
		c.updateClusterStatus(c.status.Status, false, err)

		close(stopElectionWait)
		if err != nil {
			c.AddToLeaderEventQueue(clusterclient.LeaderEvent{
				IsLeader: false,
				Cluster:  c.clusterID,
			})
			// Restart leader election after an error.
			time.AfterFunc(time.Second, func() {
				for {
					select {
					case <-c.leaderElectionStopChannel:
						return
					default:
						if _, err := c.StartLeaderElection(nil); err == nil {
							return
						}
					}
				}
			})
		}
	}()
	return c.leaderElectionStopChannel, nil
}

func (c *memberClusterManager) StartMonitorResources(queue workqueue.RateLimitingInterface) (chan<- struct{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.log.Info("Starting resource monitoring")
	c.resourceEventQueue = queue
	c.resourceMonitorStopChannel = make(chan struct{})
	go func() {
		// update status
		c.updateClusterStatus(Reachable, c.status.IsLeader, nil)

		err := c.clusterManager.Start(c.resourceMonitorStopChannel)
		c.log.Info("Resource exited", "Error", err)
		if err != nil {
			// Removes itself from FedManager. Will be added back during
			// federation Reconciliation.
			_ = c.fedReconciler.federationManager.RemoveClusterClient(c)
			// Wait for client removed from fedManager.
			<-c.resourceMonitorStopChannel
			c.fedReconciler.setReconcile()
		}

		// update status
		c.updateClusterStatus(Unreachable, c.status.IsLeader, err)
	}()
	return c.resourceMonitorStopChannel, nil
}
