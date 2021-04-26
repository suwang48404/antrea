/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package externalentityowners

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	networking "github.com/vmware-tanzu/antrea/federation/controllers/externalentity"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

// VirtualMachineOwnerOf says VirtualMachine is owner of ExternalEntity.
type EndpointsOwnerOf struct {
	v1.Endpoints
}

func init() {
	networking.RegisterExternalEntityOwner("EndpointsOwnerOf", &EndpointsOwnerOf{})
}

// GetEndPointAddresses returns all EndpointSubnet addresses.
func (e *EndpointsOwnerOf) GetEndPointAddresses(_ client.Client) ([]string, error) {
	if len((clusterclient.Resource{Object: &e.Endpoints}).FederationID()) == 0 {
		return nil, nil
	}
	ips := make([]string, 0)
	for _, subset := range e.Subsets {
		for _, ip := range subset.Addresses {
			if len(ip.IP) == 0 {
				continue
			}
			ips = append(ips, ip.IP)
		}
	}
	return ips, nil
}

// GetEndPointPort returns all EndpointSubset Ports.
func (e *EndpointsOwnerOf) GetEndPointPort(_ client.Client) []antreatypes.NamedPort {
	if len((clusterclient.Resource{Object: &e.Endpoints}).FederationID()) == 0 {
		return nil
	}
	ports := make([]antreatypes.NamedPort, 0)
	for _, subset := range e.Subsets {
		for _, port := range subset.Ports {
			if port.Port == 0 {
				continue
			}
			ports = append(ports, antreatypes.NamedPort{
				Protocol: port.Protocol,
				Port:     port.Port,
				Name:     port.Name,
			})
		}
	}
	return ports
}

// GetTags returns nil for Endpoints.
func (e *EndpointsOwnerOf) GetTags() map[string]string {
	return nil
}

// GetLabels returns Kind label that over-write general kind label.
func (e *EndpointsOwnerOf) GetLabels(_ client.Client) map[string]string {
	return map[string]string{config.ExternalEntityLabelKeyKind: config.ExternalEntityLabelKeyFederatedServiceKind}
}

// GetExternalNode returns empty string as NetworkPolicies cannot be
// applied to it.
func (e *EndpointsOwnerOf) GetExternalNode(_ client.Client) string {
	return ""
}

// Copy returns a duplicate of EndpointsOwnerOf.
func (e *EndpointsOwnerOf) Copy() networking.ExternalEntityOwner {
	newEps := &EndpointsOwnerOf{}
	e.Endpoints.DeepCopyInto(&newEps.Endpoints)
	return newEps
}

// EmbedType returns Endpoints resource.
func (e *EndpointsOwnerOf) EmbedType() runtime.Object {
	return &e.Endpoints
}

func (e *EndpointsOwnerOf) IsFedResource() bool {
	return true
}

var (
	_ networking.ExternalEntityOwner = &EndpointsOwnerOf{}
	_ runtime.Object                 = &EndpointsOwnerOf{}
)
