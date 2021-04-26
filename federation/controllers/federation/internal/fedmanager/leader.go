/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package fedmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/mohae/deepcopy"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"

	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
)

func (m *federationManager) handleNamespace(r clusterclient.Resource) error {
	cluster := m.clusters[r.Cluster]
	if !cluster.isManagedBy {
		return nil
	}
	isNew := false
	if r.IsDelete() {
		_ = m.clusterResources.Delete(r)
	} else if _, exists, _ := m.clusterResources.Get(r); exists {
		_ = m.clusterResources.Update(r)
	} else {
		isNew = true
		_ = m.clusterResources.Add(r)
	}
	if err := m.notifyServiceNSChange(cluster, r, isNew); err != nil {
		return err
	}
	if err := m.notifyNetworkPolicyNSChange(cluster, r, isNew); err != nil {
		return err
	}
	return nil
}

// isLeader returns true if this federationManager is the leader of at least one cluster(s).
func (m *federationManager) isLeader() bool {
	for _, c := range m.clusters {
		if c.isManagedBy {
			return true
		}
	}
	return false
}

// processLeaderEvent process a leader event from a cluster,
// When this federationManager becomes the leader of the cluster, it removes all stale federated clusterResources from
// other federations and updates federated clusterResources from this federationID.
func (m *federationManager) processLeaderEvent(evt clusterclient.LeaderEvent) error {
	log := m.log.WithName("Leader")
	cluster, ok := m.clusters[evt.Cluster]
	if !ok {
		log.Error(nil, "Receive leader event from unknown cluster", "Cluster", evt.Cluster)
		return nil
	}
	if evt.IsLeader == cluster.isManagedBy {
		log.V(1).Info("Received redundant leader event", "Cluster", cluster.GetClusterID(),
			"IsLeader", cluster.isManagedBy)
		return nil
	}
	log.V(1).Info("Received leader event", "Cluster", cluster.GetClusterID(), "IsLeader", evt.IsLeader)
	oldIsLeader := m.isLeader()
	cluster.isManagedBy = evt.IsLeader
	if cluster.isManagedBy {
		// Now this federationManager is new leader of the cluster ...
		// Every cluster must be member of a single federation. Remove federated clusterResources from previous/other federations.
		if err := m.purgeFedResources(cluster, false); err != nil {
			return err
		}
		// Get namespaces
		nsList := &v1.NamespaceList{}
		if err := cluster.List(context.Background(), nsList, &client2.ListOptions{}); err != nil {
			return err
		}
		for i := range nsList.Items {
			_ = m.clusterResources.Add(clusterclient.Resource{
				Object:  &nsList.Items[i],
				Cluster: cluster.GetClusterID(),
			})
		}
	}
	if oldIsLeader && !m.isLeader() {
		// This federationManager is no longer a leader of any clusters,
		_ = m.clusterResources.Replace(nil, "")
		_ = m.fedResources.Replace(nil, "")
	} else if !oldIsLeader && m.isLeader() {
		// This federationManager just become the leader of a cluster for the first time.
		if err := m.initializeFedServices(); err != nil {
			cluster.isManagedBy = false
			return err
		}
		if err := m.initializeFedNetworkPolicies(); err != nil {
			cluster.isManagedBy = false
			return err
		}
	} else {
		if err := m.notifyServiceLeaderChange(cluster, ""); err != nil {
			return err
		}
		if err := m.notifyNetworkPolicyLeaderChange(cluster); err != nil {
			return err
		}
	}
	return nil
}

// purgeFedResources removed federated clusterResources from cluster.
// When thisFed is true, it removes clusterResources associated with this federationID;
// and if false, it removes clusterResources associated with other federations.
func (m *federationManager) purgeFedResources(cluster *federationClient, thisFed bool) error {
	log := m.log.WithName("Leader")
	if !cluster.isManagedBy {
		return nil
	}
	log.V(1).Info("Purge federated resources", "Cluster", cluster.GetClusterID(), "ForThisFederation", thisFed)
	purge, err := m.preparePurgeFedNetworkPolicies(cluster, thisFed)
	if err != nil {
		return err
	}
	purge1, err := m.preparePurgeFedServices(cluster, thisFed)
	if err != nil {
		return err
	}
	purge = append(purge, purge1...)
	for _, p := range purge {
		if err := m.deleteFedResource(cluster, clusterclient.Resource{
			Object:  p,
			Cluster: cluster.GetClusterID(),
		}); err != nil && errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// updateFedResource create/update a federated clusterResources to cluster.
func (m *federationManager) updateFedResource(cluster *federationClient, resource clusterclient.Resource) error {
	log := m.log.WithName("Leader")
	log.V(1).Info("Update federated resource", "Cluster",
		cluster.GetClusterID(), "Resource", resource)
	var err error
	access, _ := meta.Accessor(resource.Object)
	objKey := client2.ObjectKey{
		Name:      access.GetName(),
		Namespace: access.GetNamespace(),
	}
	nsKey := clusterclient.GenerateResourceKey(objKey.Namespace, "", "Namespace", cluster.GetClusterID())
	if _, exists, _ := m.clusterResources.GetByKey(nsKey); !exists {
		log.V(1).Info("Ignore update resource in non-existing namespace", "Namespace", objKey.Namespace,
			"Cluster", cluster.GetClusterID())
		return nil
	}
	// Ensures no changes in passed in resource, it may be cached.
	ans := access.GetAnnotations()
	if ans == nil {
		ans = make(map[string]string)
	} else {
		ans = deepcopy.Copy(ans).(map[string]string)
	}
	ans[config.FederationIDAnnotation] = string(m.federationID)
	labels := deepcopy.Copy(access.GetLabels()).(map[string]string)
	var obj runtime.Object
	switch v := resource.Object.(type) {
	case *v1.Service:
		svc := &v1.Service{}
		err = cluster.Get(context.Background(), objKey, svc)
		clusterIP := ""
		if err == nil {
			if svc.Spec.Type != v1.ServiceTypeClusterIP {
				log.Info("Federated Service detected incorrect type", "Service", svc)
			}
			if len((clusterclient.Resource{Object: svc}).FederationID()) == 0 {
				log.Info("Federated Service has name conflict", "Cluster", cluster.GetClusterID(), "Service", objKey)
			}
			// Preserve immutable clusterIP in update.
			clusterIP = svc.Spec.ClusterIP
		}
		svc.Spec.Reset()
		v.Spec.DeepCopyInto(&svc.Spec)
		svc.Spec.ClusterIP = clusterIP
		obj = svc
	case *v1.Endpoints:
		obj, err = m.generateClusterFedEndpoints(cluster, v)
	case *antreacore.ExternalEntity:
		ee := &antreacore.ExternalEntity{}
		err = cluster.Get(context.Background(), objKey, ee)
		ee.Spec = antreacore.ExternalEntitySpec{}
		v.Spec.DeepCopyInto(&ee.Spec)
		// NetworkPolicy cannot be applied to Federated ExternalEntities.
		v.Spec.ExternalNode = ""
		obj = ee
	default:
		return fmt.Errorf("update unrecognized federated resource %v", resource)
	}
	isUpdate := false
	access1, _ := meta.Accessor(obj)
	if err == nil {
		isUpdate = true
	} else if !errors.IsNotFound(err) {
		log.Error(err, "Failed to get resource  in member cluster", "Cluster", cluster.GetClusterID(), "Resource", obj)
		return err
	} else {
		access1.SetName(objKey.Name)
		access1.SetNamespace(objKey.Namespace)
	}
	access1.SetAnnotations(ans)
	access1.SetLabels(labels)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if isUpdate {
		if err = cluster.Update(ctx, obj, &client2.UpdateOptions{}); err != nil {
			log.Error(err, "Failed to update resource in member cluster", "Cluster", cluster.GetClusterID(), "Resource", obj)
			return err
		}
	} else {
		if err = cluster.Create(ctx, obj, &client2.CreateOptions{}); err != nil {
			log.Error(err, "Failed to create resource in member cluster", "Cluster", cluster.GetClusterID(), "Resource", obj)
			return err
		}
	}
	return nil
}

// deleteFedResource deletes a federated resource from cluster.
func (m *federationManager) deleteFedResource(cluster *federationClient, resource clusterclient.Resource) error {
	log := m.log.WithName("Leader")
	log.V(1).Info("Delete federated resource", "Cluster", cluster.GetClusterID(), "Resource", resource)
	if resource.Kind() == "Endpoints" {
		tmp := clusterclient.Resource{
			Object:  resource.Object,
			Cluster: cluster.GetClusterID(),
		}
		if obj, exists, _ := m.fedResources.GetByKey(tmp.Key()); exists {
			_ = m.fedResources.Delete(obj)
		}
	}
	access, _ := meta.Accessor(resource.Object)
	nsKey := clusterclient.GenerateResourceKey(access.GetNamespace(), "", "Namespace", cluster.GetClusterID())
	if _, exists, _ := m.clusterResources.GetByKey(nsKey); !exists {
		log.V(1).Info("Ignore delete resource in non-existing namespace", "Namespace", access.GetNamespace(),
			"Cluster", cluster.GetClusterID())
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cluster.Delete(ctx, resource.Object, &client2.DeleteOptions{}); err != nil {
		log.Error(err, "Failed to delete resource in member cluster", "Cluster", cluster.GetClusterID(), "Resource", resource)
		return err
	}
	return nil
}

// updateFedResourceToAll create/update a federated clusterResources to all clusters that has elected this
// federationManager as leader.
func (m *federationManager) updateFedResourceToAll(resource clusterclient.Resource) error {
	for _, cluster := range m.clusters {
		if !cluster.isManagedBy {
			continue
		}
		if err := m.updateFedResource(cluster, resource); err != nil {
			return err
		}
	}
	return nil
}

// deleteFedResourceFromAll deletes a federated resource from all clusters that has elected this
// federationManager as leader.
func (m *federationManager) deleteFedResourceFromAll(resource clusterclient.Resource) error {
	for _, cluster := range m.clusters {
		if !cluster.isManagedBy {
			continue
		}
		if err := m.deleteFedResource(cluster, resource); err != nil {
			return err
		}
	}
	return nil
}
