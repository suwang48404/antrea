/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package fedmanager

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/mohae/deepcopy"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
)

// notifyServiceClientRemove removes all Services, Endpoints from the cluster, and recompute federated Services.
func (m *federationManager) notifyServiceClientRemove(cluster clusterclient.ClusterID) error {
	for _, obj := range []runtime.Object{&v1.Service{}, &v1.Endpoints{}} {
		kindIdx := clusterclient.Resource{
			Object:  obj,
			Cluster: cluster,
		}.KindAndCluster()
		items, _ := m.clusterResources.ByIndex(resourceIndexerByKindCluster, kindIdx)
		for _, i := range items {
			_ = m.clusterResources.Delete(i)
		}
	}
	svcs, err := m.getFedServices("")
	if err != nil {
		return err
	}
	for _, svc := range svcs {
		objKey := client2.ObjectKey{
			Namespace: svc.Namespace(),
			Name:      svc.Name(),
		}
		if err := m.computeFedService(objKey); err != nil {
			return err
		}
	}
	return nil
}

// notifyServiceNSChange adds federated Services to the new Namespace in managed cluster.
func (m *federationManager) notifyServiceNSChange(cluster *federationClient, r clusterclient.Resource, isNew bool) error {
	// No-op to Namespace delete or update
	if r.IsDelete() || !isNew {
		return nil
	}
	// Populate federated Services to new Namespace.
	return m.notifyServiceLeaderChange(cluster, r.Object.(*v1.Namespace).Name)
}

// notifyServiceLeaderChange adds federated Services to managed cluster.
func (m *federationManager) notifyServiceLeaderChange(cluster *federationClient, ns string) error {
	if !cluster.isManagedBy {
		return nil
	}
	objs, err := m.getFedServices(ns)
	if err != nil {
		return err
	}
	for _, r := range objs {
		if err := m.updateFedResource(cluster, r); err != nil {
			return err
		}
	}
	return nil
}

// getFedServices returns federated Services and Endpoints.
func (m *federationManager) getFedServices(ns string) ([]clusterclient.Resource, error) {
	items, err := m.fedResources.ByIndex(resourceIndexerByKind, "Service")
	if err != nil {
		return nil, err
	}
	items1, err := m.fedResources.ByIndex(resourceIndexerByKind, "Endpoints")
	if err != nil {
		return nil, err
	}
	items = append(items, items1...)
	svcResources := make([]clusterclient.Resource, 0, len(items))
	for i := range items {
		access, _ := meta.Accessor(items[i].(clusterclient.Resource).Object)
		if len(ns) == 0 || access.GetNamespace() == ns {
			svcResources = append(svcResources, items[i].(clusterclient.Resource))
		}
	}
	return svcResources, nil
}

// initializeFedServices retrieves Services and Endpoints from all member clusters and computes required federated Services.
func (m *federationManager) initializeFedServices() error {
	log := m.log.WithName("Service")
	log.Info("Initializing ... ")
	fedSvcMap := make(map[string]*client2.ObjectKey)
	for _, cluster := range m.clusters {
		svcList := &v1.ServiceList{}
		if err := cluster.List(context.Background(), svcList, &client2.ListOptions{}); err != nil {
			log.Error(err, "Failed to list Services from member cluster", "Cluster", cluster.GetClusterID())
			return err
		}
		for i := range svcList.Items {
			svc := &svcList.Items[i]
			r := clusterclient.Resource{
				Object:  svc,
				Cluster: cluster.GetClusterID(),
			}
			fedSvcObjKey := r.UserDefinedFederatedKey()
			if fedSvcObjKey == nil {
				continue
			}
			log.V(1).Info("Add federated Service to cache", "Service", r.Key())
			_ = m.clusterResources.Add(r)
			fedSvcMap[fedSvcObjKey.String()] = fedSvcObjKey
		}
	}
	for _, fedSvc := range fedSvcMap {
		if err := m.computeFedService(*fedSvc); err != nil {
			return err
		}
	}
	return nil
}

// preparePurgeFedServices returns a list of federate Services and Endpoints that need to be purged from cluster.
func (m *federationManager) preparePurgeFedServices(cluster *federationClient, thisFed bool) ([]runtime.Object, error) {
	log := m.log.WithName("Service")
	log.V(1).Info("Purge federated resources", "Cluster", cluster.GetClusterID(),
		"ThisFederation", thisFed)
	purge := make([]runtime.Object, 0)
	updatePurging := func(obj runtime.Object) {
		fedID := clusterclient.Resource{Object: obj}.FederationID()
		if len(fedID) == 0 {
			return
		}
		if (fedID == m.federationID) == thisFed {
			purge = append(purge, obj)
		}
	}
	svcList := &v1.ServiceList{}
	if err := cluster.List(context.Background(), svcList, &client2.ListOptions{}); err != nil {
		log.Error(err, "Failed to list Services in member cluster", "Cluster", cluster.GetClusterID())
		return nil, err
	}
	for i := range svcList.Items {
		svc := &svcList.Items[i]
		updatePurging(svc)
	}
	epsList := &v1.EndpointsList{}
	if err := cluster.List(context.Background(), epsList, &client2.ListOptions{}); err != nil {
		log.Error(err, "Failed to list Services in member cluster", "Cluster", cluster.GetClusterID())
		return nil, err
	}
	for i := range epsList.Items {
		eps := &epsList.Items[i]
		updatePurging(eps)
	}
	return purge, nil
}

// generateClusterFedEndpoints returns a federated Endpoints for a cluster.
// If the corresponding federated Service contains some member Services in the cluster, the federated Endpoints for
// this cluster consists of member Service's Endpoints only.
func (m *federationManager) generateClusterFedEndpoints(cluster *federationClient, src *v1.Endpoints) (*v1.Endpoints, error) {
	eps := &v1.Endpoints{
		TypeMeta: metav1.TypeMeta{Kind: "Endpoints"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      src.Name,
			Namespace: src.Namespace,
		},
	}
	clusterfedSvcKey := clusterclient.GenerateResourceKey(src.Name, src.Namespace,
		(clusterclient.Resource{Object: &v1.Service{}}).Kind(), cluster.GetClusterID())
	items, _ := m.clusterResources.ByIndex(serviceIndexerByClusterFederatedName, clusterfedSvcKey)
	getErr := cluster.Get(context.Background(), client2.ObjectKey{Namespace: src.Namespace, Name: src.Name}, eps)
	if len(items) == 0 {
		// Local service is not found for this federated Service. Use original federated Endpoints to update this cluster.
		return src.DeepCopy(), getErr
	}

	// Process local Services.
	eps.Annotations = deepcopy.Copy(src.Annotations).(map[string]string)
	eps.Labels = deepcopy.Copy(eps.Labels).(map[string]string)
	localEps := make([]*v1.Endpoints, 0)
	for _, i := range items {
		if eps := m.getClusterEndpointsFromService(i.(clusterclient.Resource)); eps != nil {
			localEps = append(localEps, eps)
		}
	}
	// Overwrite federated Endpoints with local Endpoints
	eps.Subsets = nil
	for _, s := range localEps {
		for i := range s.Subsets {
			for k := range s.Subsets[i].Ports {
				// It seems K8s requires Endpoints and Service match port name.
				// We allow Endpoints' ports from different cluster have different names.
				// Therefore we clear out Port names in Service and in Endpoints.Subset.
				s.Subsets[i].Ports[k].Name = ""
			}
			eps.Subsets = append(eps.Subsets, *s.Subsets[i].DeepCopy())
		}
	}
	r := clusterclient.Resource{
		Object:  eps,
		Cluster: cluster.GetClusterID(),
	}
	if _, exists, _ := m.fedResources.Get(r); exists {
		_ = m.fedResources.Update(r)
	} else {
		_ = m.fedResources.Add(r)
	}
	return eps, getErr
}

// handleNodeResource triggers a re-computation of federated Services that contains NodePort Services as members.
func (m *federationManager) handleNodeResource(nodeResource clusterclient.Resource) error {
	log := m.log.WithName("Service")
	log.V(1).Info("Node changes in cluster", "Cluster", nodeResource.Cluster)
	items, err := m.clusterResources.ByIndex(nodePortServiceIndexerByCluster, string(nodeResource.Cluster))
	if err != nil {
		log.Error(err, "Failed to get NodePort Service from cache", "Cluster", nodeResource.Cluster)
		return err
	}
	if len(items) == 0 {
		return nil
	}
	fedSvcMap := make(map[string]*client2.ObjectKey)
	for _, i := range items {
		r := i.(clusterclient.Resource)
		key := r.UserDefinedFederatedKey()
		fedSvcMap[key.String()] = key
	}
	for _, fedSvc := range fedSvcMap {
		if err := m.computeFedService(*fedSvc); err != nil {
			return err
		}
	}
	return nil
}

// handleServiceResourceCommon provides common processing to Endpoints and Services.
func (m *federationManager) handleServiceResourceCommon(r clusterclient.Resource,
	fedResourceEqual func(r1, r2 clusterclient.Resource) bool) error {
	log := m.log.WithName("Service")
	cached, found := m.getResource(r)
	isDelete := r.IsDelete()
	if !found && isDelete {
		// Non-cached resource is deleted, no-op.
		return nil
	}
	fedID := r.FederationID()
	if len(fedID) == 0 && found {
		fedID = cached.FederationID()
	}
	var err error
	if len(fedID) == 0 {
		// is cluster resource.
		fedResourceKey := m.getFederatedResourceObjectKey(r)
		if isDelete {
			fedResourceKey = m.getFederatedResourceObjectKey(cached)
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
		if fedResourceKey == nil {
			log.V(1).Info("Cannot derive federated resource name, possibly an orphan Endpoints", "Resource", r)
			return nil
		} else if errs := validation.IsQualifiedName(fedResourceKey.Name); len(errs) > 0 {
			log.V(1).Info("Invalid federated resource name", "Name", fedResourceKey,
				"Errors", errs)
			return nil
		}
		return m.computeFedService(*fedResourceKey)
	}

	// Some other federationManager is the leader of this cluster, no-op.
	cluster := m.clusters[r.Cluster]
	if !cluster.isManagedBy {
		return nil
	}
	if fedID != m.federationID {
		log.Error(nil, "Federated resource from another federationID", "Resource", r)
		return nil
	}
	if !found {
		log.Error(nil, "Federated resource cannot be found, perhaps stale resource deleting", "Resource", r)
		return m.deleteFedResource(cluster, r)
	}
	if isDelete && found {
		log.Info("Federated resource is deleted by other entity, recovering", "Resource", r, "Cached", cached)
		return m.updateFedResource(cluster, cached)
	} else if !fedResourceEqual(cached, r) {
		log.Info("Federated resource is modified by other entity, recovering", "Resource", r, "Cached", cached)
		return m.updateFedResource(cluster, cached)
	}
	return nil
}

// computeFedService computes federated Service.
func (m *federationManager) computeFedService(svcKey client2.ObjectKey) error {
	log := m.log.WithName("Service")
	log.V(1).Info("Compute federated Service", "Service", svcKey)
	fedResource := clusterclient.Resource{
		Object: &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        svcKey.Name,
				Namespace:   svcKey.Namespace,
				Annotations: map[string]string{config.FederationIDAnnotation: string(m.federationID)},
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "Service",
			},
		},
	}
	i, fedResourceExists, _ := m.fedResources.Get(fedResource)
	if fedResourceExists {
		fedResource = i.(clusterclient.Resource)
	}
	items, err := m.clusterResources.ByIndex(serviceIndexerByFederatedName, svcKey.String())
	if err != nil {
		log.Error(err, "Failed to list Services by indexer", "Key", svcKey.String())
		return err
	}
	clusterResources := make([]clusterclient.Resource, 0, len(items))
	for _, i := range items {
		clusterResources = append(clusterResources, i.(clusterclient.Resource))
	}
	if !fedResourceExists && len(clusterResources) == 0 {
		return nil
	}
	if !fedResourceExists && len(clusterResources) > 0 {
		// Add new federated Service
		fedSvc := fedResource.Object.(*v1.Service)
		fedSvc.Spec.Type = v1.ServiceTypeClusterIP
		svcPort := int32(0)
		protocol := v1.ProtocolTCP
		for _, p := range clusterResources[0].Object.(*v1.Service).Spec.Ports {
			if p.Port > 0 {
				svcPort = p.Port
				protocol = p.Protocol
				break
			}
		}
		fedSvc.Spec.Ports = []v1.ServicePort{{
			Protocol: protocol,
			Port:     svcPort,
		}}
		if err = m.updateFedResourceToAll(fedResource); err != nil {
			return err
		}
		if err = m.fedResources.Add(fedResource); err != nil {
			log.Error(err, "Failed to add federated Service to cache", "Service", fedResource)
			// Cannot be a transient error.
			return nil
		}
	} else if fedResourceExists && len(clusterResources) == 0 {
		// Delete stale federated Service.
		if err = m.deleteFedResourceFromAll(fedResource); err != nil {
			return err
		}
		if err = m.fedResources.Delete(fedResource); err != nil {
			log.Error(err, "Failed to delete federated Service to cache", "Service", fedResource)
			return nil
		}
	}
	// Update federated Endpoints.
	return m.computeFedEndpoints(svcKey, clusterResources)
}

// getClusterEndpointsFromService updates cache and returns corresponding Endpoints of a Service.
// It returns nil if Endpoints cannot be found, and Endpoints contains no subsets.
func (m *federationManager) getClusterEndpointsFromService(svcResource clusterclient.Resource) *v1.Endpoints {
	log := m.log.WithName("Service")
	cluster := m.clusters[svcResource.Cluster]
	svc := svcResource.Object.(*v1.Service)
	er := clusterclient.Resource{
		Object: &v1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svc.Name,
				Namespace: svc.Namespace,
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "Endpoints",
			},
		},
		Cluster: svcResource.Cluster,
	}
	var eps *v1.Endpoints
	if i, exist, _ := m.clusterResources.Get(er); exist {
		eps = i.(clusterclient.Resource).Object.(*v1.Endpoints)
	} else {
		eps = er.Object.(*v1.Endpoints)
		_ = cluster.Get(context.Background(), client2.ObjectKey{
			Namespace: svc.Namespace,
			Name:      svc.Name,
		}, eps)
		// Regardless if Get status, always add to cache because Endpoints may come later.
		log.V(1).Info("Add new Endpoints to cache", "Endpoints", er)
		_ = m.clusterResources.Add(er)
	}
	if len(eps.Subsets) == 0 {
		return nil
	}
	return eps.DeepCopy()
}

// computeClusterIPServiceEndpointSubsets returns EndpointSubsets associated with a ClusterIP Service.
func (m *federationManager) computeClusterIPServiceEndpointSubsets(svcResource clusterclient.Resource) []v1.EndpointSubset {
	eps := m.getClusterEndpointsFromService(svcResource)
	if eps == nil {
		return nil
	}
	return eps.Subsets
}

// computeNodePortServiceEndpointSubsets returns EndpointSubsets associated with a NodePort Service.
func (m *federationManager) computeNodePortServiceEndpointSubsets(svcResource clusterclient.Resource) ([]v1.EndpointSubset, error) {
	if eps := m.getClusterEndpointsFromService(svcResource); eps == nil {
		return nil, nil
	}
	cluster := m.clusters[svcResource.Cluster]
	svc := svcResource.Object.(*v1.Service)
	nodePort := int32(0)
	for _, p := range svc.Spec.Ports {
		if p.NodePort > 0 {
			nodePort = p.NodePort
			break
		}
	}
	if nodePort == 0 {
		// Service is not ready,
		return nil, nil
	}
	nodeList := &v1.NodeList{}
	if err := cluster.List(context.Background(), nodeList, &client2.ListOptions{}); err != nil {
		return nil, err
	}
	subset := v1.EndpointSubset{}
	for i := range nodeList.Items {
		// Prefer external node IP over internal.
		externIPs := make([]string, 0)
		internalIPs := make([]string, 0)
		node := &nodeList.Items[i]
		if len(node.Status.Addresses) == 0 {
			continue
		}
		ready := false
		for _, c := range node.Status.Conditions {
			if c.Type == v1.NodeReady {
				ready = c.Status == v1.ConditionTrue
				break
			}
		}
		if !ready {
			continue
		}
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				internalIPs = append(internalIPs, addr.Address)
			} else if addr.Type == v1.NodeExternalIP {
				externIPs = append(externIPs, addr.Address)
			} // TODO, what about other Type ??
		}
		if len(externIPs) > 0 {
			for _, ip := range externIPs {
				subset.Addresses = append(subset.Addresses, v1.EndpointAddress{
					IP:       ip,
					NodeName: &node.Name,
				})
			}
		} else if len(internalIPs) > 0 {
			for _, ip := range internalIPs {
				subset.Addresses = append(subset.Addresses, v1.EndpointAddress{
					IP:       ip,
					NodeName: &node.Name,
				})
			}
		}
	}
	if len(subset.Addresses) == 0 {
		return nil, nil
	}
	subset.Ports = []v1.EndpointPort{{Port: nodePort}}
	return []v1.EndpointSubset{subset}, nil
}

// computeLoadBalancerServiceEndpointSubsets returns EndpointSubsets associated with a LoadBalancer Service.
func (m *federationManager) computeLoadBalancerServiceEndpointSubsets(r clusterclient.Resource) ([]v1.EndpointSubset, error) {
	log := m.log.WithName("Service")
	if er := m.getClusterEndpointsFromService(r); er == nil {
		return nil, nil
	}
	svc := r.Object.(*v1.Service)
	svcPort := int32(0)
	for _, p := range svc.Spec.Ports {
		if p.Port > 0 {
			svcPort = p.Port
			break
		}
	}
	subset := v1.EndpointSubset{}
	for _, addr := range svc.Status.LoadBalancer.Ingress {
		if len(addr.IP) > 0 {
			subset.Addresses = append(subset.Addresses, v1.EndpointAddress{
				IP: addr.IP,
			})
		} else if len(addr.Hostname) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, addr.Hostname)
			cancel()
			if err != nil {
				log.Error(err, "Unable to resolve LB for Service", "Service", svc)
				return nil, err
			}
			// TODO, not very robust, LB may change IPs later
			for _, ip := range ips {
				subset.Addresses = append(subset.Addresses, v1.EndpointAddress{
					IP: ip.IP.To4().String(),
				})
			}
		}
	}
	if len(subset.Addresses) == 0 {
		return nil, nil
	}
	subset.Ports = []v1.EndpointPort{{Port: svcPort}}
	return []v1.EndpointSubset{subset}, nil
}

// computeFedEndpoints computes federated Endpoints of federated Service of epsKey,
// svcResources are member Services of federated Service epsKey.
func (m *federationManager) computeFedEndpoints(epsKey client2.ObjectKey, svcResources []clusterclient.Resource) error {
	log := m.log.WithName("Service")
	log.V(1).Info("Compute federated Endpoints", "Endpoints", epsKey,
		"MemberServices", svcResources)
	fedResource := clusterclient.Resource{
		Object: &v1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:        epsKey.Name,
				Namespace:   epsKey.Namespace,
				Annotations: map[string]string{config.FederationIDAnnotation: string(m.federationID)},
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "Endpoints",
			},
		},
		Cluster: "",
	}
	i, fedResourceExists, _ := m.fedResources.Get(fedResource)
	if fedResourceExists {
		fedResource = i.(clusterclient.Resource)
	}
	var err error
	if !fedResourceExists && len(svcResources) == 0 {
		return nil
	}
	fedEps := fedResource.Object.(*v1.Endpoints)
	fedEps.Subsets = nil
	if fedResourceExists || len(svcResources) > 0 {
		// Re-compute subsets from cluster endpoints
		for _, r := range svcResources {
			svc := r.Object.(*v1.Service)
			var subsets []v1.EndpointSubset
			var err error
			switch svc.Spec.Type {
			case v1.ServiceTypeClusterIP:
				subsets = m.computeClusterIPServiceEndpointSubsets(r)
			case v1.ServiceTypeNodePort:
				subsets, err = m.computeNodePortServiceEndpointSubsets(r)
			case v1.ServiceTypeLoadBalancer:
				subsets, err = m.computeLoadBalancerServiceEndpointSubsets(r)
			default:
				return fmt.Errorf("unknown Service Type %s", svc.Spec.Type)
			}
			if err != nil {
				return err
			}
			fedEps.Subsets = append(fedEps.Subsets, subsets...)
		}
	}
	// Delete existing federated Endpoints if there is no valid EndpointSubset.
	if len(fedEps.Subsets) == 0 {
		if fedResourceExists {
			// Delete stale federated Service.
			if err = m.deleteFedResourceFromAll(fedResource); err != nil {
				return err
			}
			if err = m.fedResources.Delete(fedResource); err != nil {
				log.Error(err, "Failed to delete federated Endpoints to cache", "Endpoints", fedResource)
				return err
			}
		} else {
			log.V(1).Info("Federated Endpoints finds no valid subsets", "Endpoints", fedResource)
		}
		return nil
	}
	if err = m.updateFedResourceToAll(fedResource); err != nil {
		return err
	}
	if !fedResourceExists {
		if err = m.fedResources.Add(fedResource); err != nil {
			log.Error(err, "Failed to add federated Service to cache", "Service", fedResource)
			return err
		}
	} else {
		if err = m.fedResources.Update(fedResource); err != nil {
			log.Error(err, "Failed to update federated Service to cache", "Service", fedResource)
			return nil
		}
	}
	return nil
}

// handleServiceResource handles federated and cluster Service.
func (m *federationManager) handleServiceResource(r clusterclient.Resource) error {
	return m.handleServiceResourceCommon(r,
		func(r1, r2 clusterclient.Resource) bool {
			svc1 := r1.Object.(*v1.Service)
			svc2 := r1.Object.(*v1.Service)
			// Ignore received Service's Target because it is assigned by k8s.
			for _, p := range svc2.Spec.Ports {
				p.TargetPort = intstr.IntOrString{}
			}
			return reflect.DeepEqual(svc1.Spec, svc2.Spec)
		})
}

//  handleEndpointsResource handles federated and cluster Endpoints.
func (m *federationManager) handleEndpointsResource(r clusterclient.Resource) error {
	if _, exists, _ := m.clusterResources.Get(r); !exists {
		return nil
	}
	return m.handleServiceResourceCommon(r,
		func(r1, r2 clusterclient.Resource) bool {
			return reflect.DeepEqual(r1.Object.(*v1.Endpoints).Subsets, r2.Object.(*v1.Endpoints).Subsets)
		})
}
