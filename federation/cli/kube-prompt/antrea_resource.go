/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package kube

import (
	"context"
	"sync"

	"github.com/c-bata/go-prompt"
	v1 "k8s.io/api/rbac/v1"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha12 "github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
)

func init() {
	clusterClaimList = new(sync.Map)
	fedList = new(sync.Map)
	anpList = new(sync.Map)
	clusterRoleList = new(sync.Map)
	clusterRoleBindingList = new(sync.Map)
}

var (
	anpList *sync.Map
)

func fetchANPList(client client2.Client, namespace string) {
	key := "anp" + namespace
	if !shouldFetch(key) {
		return
	}
	updateLastFetchedAt(key)

	// Antrea
	list := &antreanetworking.NetworkPolicyList{}
	_ = client.List(context.Background(), list, client2.InNamespace(namespace))
	anpList.Store(namespace, list)
}

func getANPSuggestions(client client2.Client, namespace string) []prompt.Suggest {
	if client == nil {
		return nil
	}
	go fetchANPList(client, namespace)
	x, ok := anpList.Load(namespace)
	if !ok {
		return []prompt.Suggest{}
	}
	l, ok := x.(*antreanetworking.NetworkPolicyList)
	if !ok || len(l.Items) == 0 {
		return []prompt.Suggest{}
	}
	s := make([]prompt.Suggest, len(l.Items))
	for i := range l.Items {
		s[i] = prompt.Suggest{
			Text: l.Items[i].Name,
		}
	}
	return s
}

var (
	clusterClaimList *sync.Map
)

func fetchCCList(client client2.Client, namespace string) {
	key := "cluster-claim" + namespace
	if !shouldFetch(key) {
		return
	}
	updateLastFetchedAt(key)
	list := &v1alpha12.ClusterClaimList{}
	_ = client.List(context.Background(), list, client2.InNamespace(namespace))
	clusterClaimList.Store(namespace, list)
}

func getCCSuggestions(client client2.Client, namespace string) []prompt.Suggest {
	if client == nil {
		return nil
	}
	go fetchCCList(client, namespace)
	x, ok := clusterClaimList.Load(namespace)
	if !ok {
		return []prompt.Suggest{}
	}
	l, ok := x.(*v1alpha12.ClusterClaimList)
	if !ok || len(l.Items) == 0 {
		return []prompt.Suggest{}
	}
	s := make([]prompt.Suggest, len(l.Items))
	for i := range l.Items {
		s[i] = prompt.Suggest{
			Text: l.Items[i].Name,
		}
	}
	return s
}

var (
	fedList *sync.Map
)

func fetchFedList(client client2.Client, namespace string) {
	key := "federation" + namespace
	if !shouldFetch(key) {
		return
	}
	updateLastFetchedAt(key)
	list := &v1alpha12.FederationList{}
	_ = client.List(context.Background(), list, client2.InNamespace(namespace))
	fedList.Store(namespace, list)
}

func getFedSuggestions(client client2.Client, namespace string) []prompt.Suggest {
	if client == nil {
		return nil
	}
	go fetchFedList(client, namespace)
	x, ok := fedList.Load(namespace)
	if !ok {
		return []prompt.Suggest{}
	}
	l, ok := x.(*v1alpha12.FederationList)
	if !ok || len(l.Items) == 0 {
		return []prompt.Suggest{}
	}
	s := make([]prompt.Suggest, len(l.Items))
	for i := range l.Items {
		s[i] = prompt.Suggest{
			Text: l.Items[i].Name,
		}
	}
	return s
}

var (
	clusterRoleList *sync.Map
)

func fetchClusterRoleList(client client2.Client, namespace string) {
	key := "clusterrole" + namespace
	if !shouldFetch(key) {
		return
	}
	updateLastFetchedAt(key)
	list := &v1.ClusterRoleList{}
	_ = client.List(context.Background(), list, client2.InNamespace(namespace))
	clusterRoleList.Store(namespace, list)
}

func getClusterRoleSuggestions(client client2.Client, _ string) []prompt.Suggest {
	if client == nil {
		return nil
	}
	go fetchClusterRoleList(client, "")
	x, ok := clusterRoleList.Load("")
	if !ok {
		return []prompt.Suggest{}
	}
	l, ok := x.(*v1.ClusterRoleList)
	if !ok || len(l.Items) == 0 {
		return []prompt.Suggest{}
	}
	s := make([]prompt.Suggest, len(l.Items))
	for i := range l.Items {
		s[i] = prompt.Suggest{
			Text: l.Items[i].Name,
		}
	}
	return s
}

var (
	clusterRoleBindingList *sync.Map
)

func fetchClusterRoleBindingList(client client2.Client, namespace string) {
	key := "clusterrolebinding" + namespace
	if !shouldFetch(key) {
		return
	}
	updateLastFetchedAt(key)
	list := &v1.ClusterRoleBindingList{}
	_ = client.List(context.Background(), list, client2.InNamespace(namespace))
	clusterRoleBindingList.Store(namespace, list)
}

func getClusterRoleBindingSuggestions(client client2.Client, _ string) []prompt.Suggest {
	if client == nil {
		return nil
	}
	go fetchClusterRoleBindingList(client, "")
	x, ok := clusterRoleBindingList.Load("")
	if !ok {
		return []prompt.Suggest{}
	}
	l, ok := x.(*v1.ClusterRoleBindingList)
	if !ok || len(l.Items) == 0 {
		return []prompt.Suggest{}
	}
	s := make([]prompt.Suggest, len(l.Items))
	for i := range l.Items {
		s[i] = prompt.Suggest{
			Text: l.Items[i].Name,
		}
	}
	return s
}
