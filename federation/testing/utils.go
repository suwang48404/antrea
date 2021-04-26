/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package testing

import (
	v1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	networking "github.com/vmware-tanzu/antrea/federation/controllers/externalentity"
	externalentityowners "github.com/vmware-tanzu/antrea/federation/controllers/externalentity/externalentity_owners"
	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

func SetupEndpoints(eps *v1.Endpoints, name, namespace string, ips []string, ports []antreatypes.NamedPort) {
	eps.Name = name
	eps.Namespace = namespace
	eps.Annotations = map[string]string{config.FederationIDAnnotation: "test-fed"}
	subset := v1.EndpointSubset{}
	for _, ip := range ips {
		subset.Addresses = append(subset.Addresses, v1.EndpointAddress{IP: ip})
	}
	for _, port := range ports {
		subset.Ports = append(subset.Ports, v1.EndpointPort{Port: port.Port, Protocol: port.Protocol, Name: port.Name})
	}
	eps.Subsets = []v1.EndpointSubset{subset}
}

func SetupEndpointsOwnerOf(owner *externalentityowners.EndpointsOwnerOf, name, namespace string,
	ips []string, ports []antreatypes.NamedPort) {
	SetupEndpoints(&owner.Endpoints, name, namespace, ips, ports)
}

// SetupExternalEntityOwners returns externalEntityOwner resources for testing.
func SetupExternalEntityOwners(ips []string, ports []antreatypes.NamedPort, namespace string) map[string]networking.ExternalEntityOwner {
	owners := make(map[string]networking.ExternalEntityOwner)
	endpoints := &externalentityowners.EndpointsOwnerOf{}
	owners["Endpoints"] = endpoints
	SetupEndpointsOwnerOf(endpoints, "test-endpoints", namespace, ips, ports)
	return owners
}
