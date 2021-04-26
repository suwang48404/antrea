/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"context"
	"fmt"

	"github.com/c-bata/go-prompt"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

const (
	antreaNS                           = config.AntreaSystemNamespace
	antreaSupervisorServiceAccount     = config.AntreaSupervisorServiceAccount
	antreaSupervisorClusterRole        = config.AntreaSupervisorRole
	antreaSupervisorClusterRoleBinding = config.AntreaSupervisorRoleBinding
	userSupervisorSecretAnnotation     = "cluster" + config.UserFedAnnotationPostfix
)

func (c *rootCompleter) getResource(rtype, ns, cluster string) []runtime.Object {
	if _, ok := c.resourceCache[rtype]; !ok {
		typeCache := make(map[string][]runtime.Object)
		for name, cl := range c.clusters {
			if !cl.supportFed() {
				continue
			}
			tmp, err := c.getResourceBackend(rtype, ns, cl)
			if err != nil {
				fmt.Printf("%v\n", err)
				continue
			}
			typeCache[name] = tmp
		}
		c.resourceCache[rtype] = typeCache
	}
	ret := make([]runtime.Object, 0)
	for name := range c.clusters {
		if len(cluster) > 0 && cluster != name {
			continue
		}
		ret = append(ret, c.resourceCache[rtype][name]...)
	}
	return ret
}

func (c *rootCompleter) getResourceBackend(rtype, ns string, cluster *clusterCompleter) ([]runtime.Object, error) {
	if !(rtype == fedResourceACNP || rtype == fedResourceANP || rtype == fedResourceService) && len(ns) > 0 && ns != antreaNS {
		return nil, fmt.Errorf("federaton resource %s dods not exist outside of namespace %s", rtype, antreaNS)
	}
	ret := make([]runtime.Object, 0)
	switch rtype {
	case fedResourceClusterClaim:
		claimList := &v1alpha1.ClusterClaimList{}
		if err := cluster.client.List(context.Background(), claimList, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		for i := range claimList.Items {
			ret = append(ret, &claimList.Items[i])
		}
	case fedResourceFederation:
		fedList := &v1alpha1.FederationList{}
		if err := cluster.client.List(context.Background(), fedList, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		for i := range fedList.Items {
			ret = append(ret, &fedList.Items[i])
		}
	case fedResourceServiceAccount:
		sa := &v1.ServiceAccount{}
		if err := cluster.client.Get(context.Background(),
			client.ObjectKey{Name: antreaSupervisorServiceAccount, Namespace: antreaNS}, sa); err != nil {
			return nil, err
		}
		ret = append(ret, sa)
	case fedResourceRole:
		role := &v12.ClusterRole{}
		if err := cluster.client.Get(context.Background(),
			client.ObjectKey{Name: antreaSupervisorClusterRole}, role); err != nil {
			return nil, err
		}
		ret = append(ret, role)
	case fedResourceRoleBinding:
		rb := &v12.ClusterRoleBinding{}
		if err := cluster.client.Get(context.Background(),
			client.ObjectKey{Name: antreaSupervisorClusterRoleBinding}, rb); err != nil {
			return nil, err
		}
		ret = append(ret, rb)
	case fedResourceSecret:
		list := &v1.SecretList{}
		if err := cluster.client.List(context.Background(), list, client.InNamespace(antreaNS)); err != nil {
			return nil, err
		}
		for i := range list.Items {
			r := clusterclient.Resource{Object: &list.Items[i]}
			if r.UserDefinedFederatedKey() != nil {
				ret = append(ret, &list.Items[i])
			}
		}
	case fedResourceService:
		list := &v1.ServiceList{}
		if err := cluster.client.List(context.Background(), list, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		for i := range list.Items {
			r := clusterclient.Resource{Object: &list.Items[i]}
			if len(r.FederationID()) > 0 || r.UserDefinedFederatedKey() != nil {
				ret = append(ret, &list.Items[i])
			}
		}
	case fedResourceEndpoints:
		list := &v1.EndpointsList{}
		if err := cluster.client.List(context.Background(), list, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		for i := range list.Items {
			r := clusterclient.Resource{Object: &list.Items[i]}
			if len(r.FederationID()) > 0 || r.UserDefinedFederatedKey() != nil {
				ret = append(ret, &list.Items[i])
			}
		}
	case fedResourceANP:
		list := &antreanetworking.NetworkPolicyList{}
		if err := cluster.client.List(context.Background(), list, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		for i := range list.Items {
			r := clusterclient.Resource{Object: &list.Items[i]}
			if r.UserDefinedFederatedKey() != nil {
				ret = append(ret, &list.Items[i])
			}
		}
	case fedResourceACNP:
		list := &antreanetworking.ClusterNetworkPolicyList{}
		if err := cluster.client.List(context.Background(), list, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		for i := range list.Items {
			r := clusterclient.Resource{Object: &list.Items[i]}
			if r.UserDefinedFederatedKey() != nil {
				ret = append(ret, &list.Items[i])
			}
		}
	case fedResourceExternalEntity:
		list := &antreacore.ExternalEntityList{}
		if err := cluster.client.List(context.Background(), list, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		for i := range list.Items {
			r := clusterclient.Resource{Object: &list.Items[i]}
			if len(r.FederationID()) > 0 {
				ret = append(ret, &list.Items[i])
			}
		}

	default:
		return nil, fmt.Errorf("unsupported federeated resource type %s", rtype)
	}
	return ret, nil
}

func (c *rootCompleter) getResourceSuggestion(_ []runtime.Object) []prompt.Suggest {
	// TODO
	return nil
}

func (c *rootCompleter) resetResource() {
	for k := range c.resourceCache {
		if k == fedResourceClusterClaim {
			continue
		}
		delete(c.resourceCache, k)
	}
}
