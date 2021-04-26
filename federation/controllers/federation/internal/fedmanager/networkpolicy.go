/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package fedmanager

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"

	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
)

// notifyNetworkPolicyClientRemove removes all ExternalEntities from the cluster, and trigger to recompute
// ExternalEntity propagation.
func (m *federationManager) notifyNetworkPolicyClientRemove(clusterID clusterclient.ClusterID) error {
	eeIdx := clusterclient.Resource{
		Object:  &antreacore.ExternalEntity{},
		Cluster: clusterID,
	}.KindAndCluster()
	items, _ := m.clusterResources.ByIndex(resourceIndexerByKindCluster, eeIdx)
	for _, i := range items {
		_ = m.clusterResources.Delete(i)
	}
	for n, cluster := range m.clusters {
		if !cluster.isManagedBy || n == clusterID {
			continue
		}
		cluster.npPolicyChanged = true
	}
	return nil
}

// notifyNetworkPolicyNSChange triggers ExternalEntity propagation computation because Namespace may have changed
// its labels.  or is added or is removed, which all have impact on if an ExternalEntity should be propagated to this
// cluster.
func (m *federationManager) notifyNetworkPolicyNSChange(cluster *federationClient, _ clusterclient.Resource, _ bool) error {
	// Trigger federated ExternalEntity propagation.
	cluster.npPolicyChanged = true
	return nil
}

// notifyNetworkPolicyLeaderChange remove any federated NetworkPolicies from cache if cluster is no longer managed,.
// If cluster just becomes managed, the function collects federated NetworkPolicies into cache from that cluster.
func (m *federationManager) notifyNetworkPolicyLeaderChange(cluster *federationClient) error {
	log := m.log.WithName("NetworkPolicy")
	if !cluster.isManagedBy {
		npIdx := clusterclient.Resource{
			Object:  &antreanetworking.ClusterNetworkPolicy{},
			Cluster: cluster.GetClusterID(),
		}.KindAndCluster()
		items, err := m.clusterResources.ByIndex(resourceIndexerByKindCluster, npIdx)
		if err != nil {
			log.Error(err, "Failed to list NetworkPolicy from cache")
			return err
		}
		for i := range items {
			_ = m.clusterResources.Delete(items[i])
		}
		npIdx = clusterclient.Resource{
			Object:  &antreanetworking.NetworkPolicy{},
			Cluster: cluster.GetClusterID(),
		}.KindAndCluster()
		items, err = m.clusterResources.ByIndex(resourceIndexerByKindCluster, npIdx)
		if err != nil {
			log.Error(err, "Failed to list ClusterNetworkPolicy from cache")
			return err
		}
		for i := range items {
			_ = m.clusterResources.Delete(items[i])
		}
		return nil
	}
	if err := m.collectNetworkPolicyFromCluster(cluster); err != nil {
		return err
	}
	cluster.npPolicyChanged = true
	return nil
}

// collectNetworkPolicyFromCluster gets and caches federated NetworkPolicies from cluster.
func (m *federationManager) collectNetworkPolicyFromCluster(cluster *federationClient) error {
	log := m.log.WithName("NetworkPolicy")
	anpList := &antreanetworking.NetworkPolicyList{}
	if err := cluster.List(context.Background(), anpList, &client2.ListOptions{}); err != nil {
		log.Error(err, "Failed to list NetworkPolicy from member cluster", "Cluster", cluster.GetClusterID())
		return err
	}
	acnpList := &antreanetworking.ClusterNetworkPolicyList{}
	if err := cluster.List(context.Background(), acnpList, &client2.ListOptions{}); err != nil {
		log.Error(err, "Failed to list ClusterNetworkPolicy from member cluster", "Cluster", cluster.GetClusterID())
		return err
	}

	if len(anpList.Items) == 0 && len(acnpList.Items) == 0 {
		return nil
	}
	for i := range anpList.Items {
		anp := &anpList.Items[i]
		r := clusterclient.Resource{
			Object:  anp,
			Cluster: cluster.GetClusterID(),
		}
		if r.UserDefinedFederatedKey() == nil {
			continue
		}
		_ = m.clusterResources.Add(r)
	}
	for i := range acnpList.Items {
		acnp := &acnpList.Items[i]
		r := clusterclient.Resource{
			Object:  acnp,
			Cluster: cluster.GetClusterID(),
		}
		if r.UserDefinedFederatedKey() == nil {
			continue
		}
		_ = m.clusterResources.Add(r)
	}
	return nil
}

// initializeFedNetworkPolicies retrieves ExternalEntities from all clusters when this fedManager become a leader of
// a member cluster. And propagate federated ExternalEntities to managed cluster.
func (m *federationManager) initializeFedNetworkPolicies() error {
	log := m.log.WithName("NetworkPolicy")
	log.Info("Initializing ... ")

	// Collect all ExternalEntities from all clusters.
	for _, cluster := range m.clusters {
		eeList := &antreacore.ExternalEntityList{}
		if err := cluster.List(context.Background(), eeList, &client2.ListOptions{}); err != nil {
			log.Error(err, "Failed to list ExternalEntity from member cluster", "Cluster", cluster.GetClusterID())
			return err
		}
		for i := range eeList.Items {
			ee := &eeList.Items[i]
			r := clusterclient.Resource{
				Object:  ee,
				Cluster: cluster.GetClusterID(),
			}
			if len(r.FederationID()) > 0 {
				// Ignore federated/generated ExternalEntities
				continue
			}
			_ = m.clusterResources.Add(r)
		}
	}
	// Collect NetworkPolicies.,
	for _, cluster := range m.clusters {
		if !cluster.isManagedBy {
			continue
		}
		cluster.npPolicyChanged = true
		if err := m.collectNetworkPolicyFromCluster(cluster); err != nil {
			return err
		}
	}
	m.processNetworkPolicyChange()
	return nil
}

// preparePurgeFedServices returns a list of federated ExternalEntities that need to be purged from cluster.
func (m *federationManager) preparePurgeFedNetworkPolicies(cluster *federationClient, thisFed bool) ([]runtime.Object, error) {
	purge := make([]runtime.Object, 0)
	updatePurging := func(obj runtime.Object, an map[string]string) {
		fedID, ok := an[config.FederationIDAnnotation]
		if !ok || len(fedID) == 0 {
			return
		}
		if (clusterclient.FederationID(fedID) == m.federationID) == thisFed {
			purge = append(purge, obj)
		}
	}
	eeList := &antreacore.ExternalEntityList{}
	if err := cluster.List(context.Background(), eeList, &client2.ListOptions{}); err != nil {
		m.log.Error(err, "Failed to list ExternalEntity in member cluster", "Cluster", cluster.GetClusterID())
		return nil, err
	}
	for i := range eeList.Items {
		ee := &eeList.Items[i]
		updatePurging(ee, ee.Annotations)
	}
	return purge, nil
}

// handleNetworkPolicyResource triggers ExternalEntity propagation because the label selectors may have changed.
func (m *federationManager) handleNetworkPolicyResource(r clusterclient.Resource) error {
	log := m.log.WithName("NetworkPolicy")
	cached, found := m.getResource(r)
	isDelete := r.IsDelete()
	if !found && isDelete {
		// Non-cached resource is deleted, no-op.
		return nil
	}
	var err error
	if isDelete {
		err = m.clusterResources.Delete(cached)
	} else if found {
		err = m.clusterResources.Update(r)
	} else {
		err = m.clusterResources.Add(r)
	}
	if err != nil {
		log.Error(err, "Failed resource cache operation", "Resource", r)
		return err
	}
	m.clusters[r.Cluster].npPolicyChanged = true
	return nil
}

// handleExternalEntityResource potentially propagates ExternalEntity changes to managed clusters.
func (m *federationManager) handleExternalEntityResource(r clusterclient.Resource) error {
	log := m.log.WithName("NetworkPolicy")
	if len(r.FederationID()) > 0 {
		// Ignore federated/generated ExternalEntity.
		return nil
	}
	cached, found := m.getResource(r)
	isDelete := r.IsDelete()
	if !found && isDelete {
		// Non-cached resource is deleted, no-op.
		return nil
	}
	var err error
	if isDelete {
		err = m.clusterResources.Delete(cached)
	} else if found {
		err = m.clusterResources.Update(r)
	} else {
		err = m.clusterResources.Add(r)
	}
	if err != nil {
		log.Error(err, "Failed resource cache operation", "Resource", r)
		return err
	}

	for n, cluster := range m.clusters {
		if !cluster.isManagedBy || n == r.Cluster ||
			!m.needPropagateExternalEntity(r, cluster.GetClusterID()) {
			continue
		}
		if isDelete {
			err = m.deleteFedResource(cluster, r)
		} else {
			err = m.updateFedResource(cluster, r)
		}
		if err != nil {
			return nil
		}
	}
	return err
}

// processNetworkPolicyChange periodically check and (re-)computes ExternalEntity propagation to managed clusters.
func (m *federationManager) processNetworkPolicyChange() {
	log := m.log.WithName("NetworkPolicy")
	for n, cluster := range m.clusters {
		// TODO handle stale propagated ExternalEntity.
		if !cluster.npPolicyChanged {
			continue
		}

		log.V(1).Info("Processing NetworkPolicy Changes", "Cluster", n)
		staleee := make(map[string]clusterclient.Resource)
		eeList := &antreacore.ExternalEntityList{}
		if err := cluster.List(context.Background(), eeList, &client2.ListOptions{}); err != nil {
			log.Error(err, "Failed to list ExternalEntity from member cluster", "Cluster", cluster.GetClusterID())
		} else {
			for i := range eeList.Items {
				r := clusterclient.Resource{
					Object:  &eeList.Items[i],
					Cluster: n,
				}
				if len(r.FederationID()) == 0 {
					// Ignore non-federated Endpoints
					continue
				}
				staleee[r.Key()] = r
			}
		}

		m.computeExternalEntityFilter(cluster.GetClusterID())
		items, err := m.clusterResources.ByIndex(resourceIndexerByKind, "ExternalEntity")
		if err != nil {
			log.Error(err, "Unable to get ExternalEntities from cache")
			return
		}
		for _, item := range items {
			r := item.(clusterclient.Resource)
			if !m.needPropagateExternalEntity(r, cluster.GetClusterID()) {
				continue
			}
			if _, ok := staleee[r.Key()]; !ok || reflect.DeepEqual(staleee[r.Key()].Object.(*antreacore.ExternalEntity).Spec,
				r.Object.(*antreacore.ExternalEntity).Spec) {
				if err = m.updateFedResource(cluster, r); err != nil {
					log.Error(err, "Unable to update ExternalEntity", "EE", r)
				}
			}
			delete(staleee, r.Key())
		}
		if err != nil {
			continue
		}
		cluster.npPolicyChanged = false
		for _, r := range staleee {
			if err = m.deleteFedResource(cluster, r); err != nil {
				log.Error(err, "Unable to delete stale  ExternalEntity", "EE", r)
			}
		}
	}
}

// computeExternalEntityFilter computes ExternalEntity filter based on federated NetworkPolicies in dstCluster.
func (m *federationManager) computeExternalEntityFilter(dstCluster clusterclient.ClusterID) {
	filters := make(map[string][]*antreanetworking.NetworkPolicyPeer)
	npIdx := clusterclient.Resource{
		Object:  &antreanetworking.NetworkPolicy{},
		Cluster: dstCluster,
	}.KindAndCluster()
	items, _ := m.clusterResources.ByIndex(resourceIndexerByKindCluster, npIdx)

	addFilter := func(filters []*antreanetworking.NetworkPolicyPeer,
		filter *antreanetworking.NetworkPolicyPeer) []*antreanetworking.NetworkPolicyPeer {
		if filter.ExternalEntitySelector != nil {
			filters = append(filters, filter)
		}
		return filters
	}
	for _, item := range items {
		anp := item.(clusterclient.Resource).Object.(*antreanetworking.NetworkPolicy)
		nsKey := clusterclient.GenerateResourceKey(anp.Namespace, "", "Namespace", dstCluster)
		if _, exists, _ := m.clusterResources.GetByKey(nsKey); !exists {
			continue
		}
		for i := range anp.Spec.Ingress {
			for k := range anp.Spec.Ingress[i].From {
				filters[nsKey] = addFilter(filters[nsKey], &anp.Spec.Ingress[i].From[k])
			}
		}
		for i := range anp.Spec.Egress {
			for k := range anp.Spec.Egress[i].To {
				filters[nsKey] = addFilter(filters[nsKey], &anp.Spec.Egress[i].To[k])
			}
		}
	}
	npIdx = clusterclient.Resource{
		Object:  &antreanetworking.ClusterNetworkPolicy{},
		Cluster: dstCluster,
	}.KindAndCluster()
	items, _ = m.clusterResources.ByIndex(resourceIndexerByKindCluster, npIdx)
	for _, item := range items {
		acnp := item.(clusterclient.Resource).Object.(*antreanetworking.ClusterNetworkPolicy)
		for i := range acnp.Spec.Ingress {
			for k := range acnp.Spec.Ingress[i].From {
				filters[""] = addFilter(filters[""], &acnp.Spec.Ingress[i].From[k])
			}
		}
		for i := range acnp.Spec.Egress {
			for k := range acnp.Spec.Egress[i].To {
				filters[""] = addFilter(filters[""], &acnp.Spec.Egress[i].To[k])
			}
		}
	}
	m.eeFilters[dstCluster] = filters
}

// needPropagateExternalEntity returns true if ExternalEntity ee should be sent to dstCluster as federated resources.
func (m *federationManager) needPropagateExternalEntity(ee clusterclient.Resource, dstCluster clusterclient.ClusterID) bool {
	// Skip generating federated ExternalEntity if the source is Service based or is from the same dstCluster.
	if ee.Cluster == dstCluster || ee.Labels()[config.ExternalEntityLabelKeyKind] == config.ExternalEntityLabelKeyFederatedServiceKind {
		return false
	}
	// Find ExternalEntity's Namespace in dstCluster.
	eeNSKey := clusterclient.GenerateResourceKey(ee.Namespace(), "", "Namespace", dstCluster)
	item, exists, _ := m.clusterResources.GetByKey(eeNSKey)
	if !exists {
		return false
	}
	eeNS := item.(clusterclient.Resource)
	for nsKey, filters := range m.eeFilters[dstCluster] {
		// ns is ANP's Namespace in dstCluster.
		var ns *clusterclient.Resource
		if len(nsKey) > 0 {
			// Check ANP Namespace
			i, exists, _ := m.clusterResources.GetByKey(nsKey)
			if !exists {
				continue
			}
			nsv := i.(clusterclient.Resource)
			ns = &nsv
		}
		for _, filter := range filters {
			nsMatch := false
			if ns != nil {
				nsSelectorMatch := false
				if selector := filter.NamespaceSelector; selector != nil {
					nsSelector := labels.SelectorFromSet(selector.MatchLabels)
					nsSelectorMatch = nsSelector.Matches(labels.Set(eeNS.Labels()))
				}
				nsMatch = filter.NamespaceSelector == nil && ee.Namespace() == ns.Name()
				if !(nsSelectorMatch || nsMatch) {
					// ExternalEntity's namespace should either be specified by ANP NS Selector and inherited from
					// the ANP.
					continue
				}
				nsMatch = true
			} else {
				// acnp filter.
				nsMatch = true
			}
			if nsMatch {
				if selector := filter.ExternalEntitySelector; selector != nil {
					eeSelector := labels.SelectorFromSet(selector.MatchLabels)
					if eeSelector.Matches(labels.Set(ee.Labels())) {
						return true
					}
				}
			}
		}
	}
	return false
}
