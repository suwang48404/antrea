/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package fedmanager

import (
	"fmt"
	"sync"
	"time"

	logging "github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

type clientEvent struct {
	isAdd  bool
	client clusterclient.ClusterClient
}

func (e clientEvent) Key() string {
	return fmt.Sprintf("ClientEvent:%s", e.client.GetClusterID())
}

type keyedItem interface {
	Key() string
}

type retryItem struct {
	keyedItem
}

type FederationManager interface {
	// Start FederationManager and blocks.
	// It returns nil when stop is triggered, or returns an internal run-time error.
	Start(stop <-chan struct{}) error
	// AddClusterClient adds a member cluster to FederationManager.
	AddClusterClient(client clusterclient.ClusterClient) error
	// RemoveClusterClient removes a member cluster from FederationManager.
	RemoveClusterClient(client clusterclient.ClusterClient) error
	// GetMemberClusters returns all member clusters.
	GetMemberClusters() map[clusterclient.ClusterID]clusterclient.ClusterClient
}

const (
	// Cache indexers
	resourceIndexerByCluster             = "resource.by.cluster"
	resourceIndexerByKind                = "resource.by.kind"
	resourceIndexerByKindCluster         = "resource.by.kind.cluster"
	serviceIndexerByFederatedName        = "service.by.federated.name"
	serviceIndexerByClusterFederatedName = "service.by.cluster.federated.name"
	nodePortServiceIndexerByCluster      = "nodePort.service.by.cluster"
)

type federationManager struct {
	federationID     clusterclient.FederationID
	clusters         map[clusterclient.ClusterID]*federationClient
	eeFilters        map[clusterclient.ClusterID]map[string][]*antreanetworking.NetworkPolicyPeer
	clusterResources cache.Indexer
	fedResources     cache.Indexer
	queue            workqueue.RateLimitingInterface
	log              logging.Logger
	clusterSyncMap   sync.Map
	// retyrMap keeps track of in-process retry items.
	retryMap map[string]struct{}
}

func NewFederationManager(fedID clusterclient.FederationID, log logging.Logger) FederationManager {
	return &federationManager{
		federationID: fedID,
		clusters:     make(map[clusterclient.ClusterID]*federationClient),
		eeFilters:    make(map[clusterclient.ClusterID]map[string][]*antreanetworking.NetworkPolicyPeer),
		clusterResources: cache.NewIndexer(
			func(in interface{}) (string, error) {
				r := in.(clusterclient.Resource)
				return r.Key(), nil
			},
			cache.Indexers{
				resourceIndexerByCluster: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					return []string{string(r.Cluster)}, nil
				},
				resourceIndexerByKind: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					return []string{r.Kind()}, nil
				},
				serviceIndexerByFederatedName: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					if _, ok := r.Object.(*v1.Service); !ok {
						return nil, nil
					}
					return []string{r.UserDefinedFederatedKey().String()}, nil
				},
				serviceIndexerByClusterFederatedName: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					if _, ok := r.Object.(*v1.Service); !ok {
						return nil, nil
					}
					fedKey := r.UserDefinedFederatedKey()
					if fedKey == nil {
						return nil, nil
					}
					return []string{clusterclient.GenerateResourceKey(fedKey.Name, fedKey.Namespace, r.Kind(), r.Cluster)}, nil
				},
				nodePortServiceIndexerByCluster: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					svc, ok := r.Object.(*v1.Service)
					if !ok || svc.Spec.Type != v1.ServiceTypeNodePort {
						return nil, nil
					}
					return []string{string(r.Cluster)}, nil
				},
				resourceIndexerByKindCluster: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					return []string{r.KindAndCluster()}, nil
				},
			}),
		fedResources: cache.NewIndexer(
			func(in interface{}) (string, error) {
				r := in.(clusterclient.Resource)
				return r.Key(), nil
			},
			cache.Indexers{
				resourceIndexerByKind: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					if len(r.Cluster) > 0 {
						// Ignore cluster specific federated resources.
						return nil, nil
					}
					return []string{r.Kind()}, nil
				},
				resourceIndexerByKindCluster: func(in interface{}) ([]string, error) {
					r := in.(clusterclient.Resource)
					return []string{r.KindAndCluster()}, nil
				},
			}),
		queue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		retryMap: make(map[string]struct{}),
		log:      log,
	}
}

// Start starts listening on and processing events from all member clusters.
func (m *federationManager) Start(stop <-chan struct{}) error {
	m.log.Info("Starting federation manager")
	// Stopper.
	stopTicker := make(chan struct{})
	go func() {
		<-stop
		m.queue.ShutDown()
	}()

	// Timer.
	ticker := time.NewTicker(time.Second)
	go func(stop <-chan struct{}) {
		for {
			select {
			case t := <-ticker.C:
				m.queue.Add(t)
			case <-stop:
				return
			}
		}
	}(stopTicker)

	for _, cl := range m.clusters {
		if err := cl.start(m); err != nil {
			return err
		}
	}
	for {
		item, shutdown := m.queue.Get()
		if shutdown {
			m.log.Info("Federation manager shutting down", "Federation", m.federationID)
			for _, c := range m.clusters {
				c.stop(m)
			}
			return nil
		}
		var err error
		m.queue.Done(item)
		if retry, ok := item.(*retryItem); ok {
			if _, ok := m.retryMap[retry.Key()]; !ok {
				m.log.V(1).Info("Ignore stale retry item", "Item", retry)
				continue
			}
			item = retry.keyedItem
		}
		if _, ok := item.(time.Time); !ok {
			delete(m.retryMap, item.(keyedItem).Key())
		}
		switch v := item.(type) {
		case clusterclient.Resource:
			err = m.processResource(v)
		case clusterclient.LeaderEvent:
			err = m.processLeaderEvent(v)
		case clientEvent:
			err = m.processClientEvent(v)
		case time.Time:
			m.processNetworkPolicyChange()
		default:
			m.log.Error(nil, "Unknown work queue item", "Item", item)
		}
		if err != nil {
			m.log.Error(err, "Process event has error")
			m.retryMap[item.(keyedItem).Key()] = struct{}{}
			m.queue.AddAfter(&retryItem{item.(keyedItem)}, time.Second)
		}
	}
}

// AddClusterClient adds a member cluster to federationManager.
func (m *federationManager) AddClusterClient(client clusterclient.ClusterClient) error {
	m.clusterSyncMap.Store(client.GetClusterID(), client)
	m.queue.Add(clientEvent{
		isAdd:  true,
		client: client,
	})
	return nil
}

// RemoveClusterClient removes a member cluster from federationManager.
func (m *federationManager) RemoveClusterClient(client clusterclient.ClusterClient) error {
	m.queue.Add(clientEvent{
		isAdd:  false,
		client: client,
	})
	return nil
}

// GetMemberClusters returns all member clusters.
func (m *federationManager) GetMemberClusters() map[clusterclient.ClusterID]clusterclient.ClusterClient {
	clusters := make(map[clusterclient.ClusterID]clusterclient.ClusterClient)
	m.clusterSyncMap.Range(func(k, v interface{}) bool {
		id := k.(clusterclient.ClusterID)
		clusters[id] = v.(clusterclient.ClusterClient)
		return true
	})
	return clusters
}

// processClientEvent is the backend processing Client add, remove events.
func (m *federationManager) processClientEvent(evt clientEvent) error {
	m.log.V(1).Info("Received cluster event", "Cluster", evt.client.GetClusterID(), "isAdd", evt.isAdd)
	if !evt.isAdd {
		clusterID := evt.client.GetClusterID()
		if c, ok := m.clusters[clusterID]; !ok {
			m.log.Error(nil, "Remove cluster not in federationID", "Cluster", clusterID)
		} else {
			if err := m.purgeFedResources(c, true); err != nil {
				return err
			}
			if err := m.notifyNetworkPolicyClientRemove(clusterID); err != nil {
				return err
			}
			if err := m.notifyServiceClientRemove(clusterID); err != nil {
				return err
			}
			c.stop(m)
		}
		delete(m.clusters, clusterID)
		m.clusterSyncMap.Delete(clusterID)
		return nil
	}
	clusterID := evt.client.GetClusterID()
	if _, ok := m.clusters[clusterID]; ok {
		m.log.Error(nil, "Add cluster already in federationID", "Cluster", clusterID)
		return nil
	}
	c := federationClient{ClusterClient: evt.client}
	if err := c.start(m); err != nil {
		return err
	}
	m.clusters[clusterID] = &c
	return nil
}

// processResource is root entry point process all resources from all clusters.
func (m *federationManager) processResource(resource clusterclient.Resource) error {
	if !m.isLeader() {
		return nil
	}
	if _, ok := m.clusters[resource.Cluster]; !ok {
		m.log.Error(nil, "Resource received from unknown cluster", "Cluster", resource.Cluster)
		// Unlikely a transient error, no reason to return error and retry.
		return nil
	}
	// Avoid excessive logging to endpoints
	if resource.Kind() != "Endpoints" {
		m.log.Info("Received resource update", "Cluster", resource.Cluster,
			"Resource", resource.Key(), "IsDelete", resource.IsDelete())
	}
	switch resource.Object.(type) {
	case *v1.Node:
		return m.handleNodeResource(resource)
	case *v1.Service:
		return m.handleServiceResource(resource)
	case *v1.Endpoints:
		return m.handleEndpointsResource(resource)
	case *antreanetworking.NetworkPolicy,
		*antreanetworking.ClusterNetworkPolicy:
		return m.handleNetworkPolicyResource(resource)
	case *antreacore.ExternalEntity:
		return m.handleExternalEntityResource(resource)
	case *v1.Namespace:
		return m.handleNamespace(resource)
	default:
		m.log.Error(nil, "Resource received is not supported", "Resource", resource)
	}
	return nil
}

// getResource return resource found in cache.
func (m *federationManager) getResource(r clusterclient.Resource) (clusterclient.Resource, bool) {
	if i, exist, _ := m.fedResources.GetByKey(r.Key()); exist {
		return i.(clusterclient.Resource), true
	} else if i, exist, _ := m.fedResources.GetByKey(r.FedKey()); exist {
		return i.(clusterclient.Resource), true
	} else if i, exist, _ := m.clusterResources.GetByKey(r.Key()); exist {
		return i.(clusterclient.Resource), true
	}
	return clusterclient.Resource{}, false
}

func (m *federationManager) getFederatedResourceObjectKey(r clusterclient.Resource) *client.ObjectKey {
	if key := r.UserDefinedFederatedKey(); key != nil {
		return key
	}
	// If Endpoints, using associated Service to retrieve federated Service.
	eps, ok := r.Object.(*v1.Endpoints)
	if !ok {
		return nil
	}
	r.Object = &v1.Service{
		TypeMeta:   metav1.TypeMeta{Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: eps.Name, Namespace: eps.Namespace},
	}
	if i, exists, _ := m.clusterResources.Get(r); exists {
		return i.(clusterclient.Resource).UserDefinedFederatedKey()
	}
	return nil
}
