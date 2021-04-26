/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	"github.com/vmware-tanzu/antrea/federation/utils"
	"github.com/vmware-tanzu/antrea/federation/utils/templates"
)

func dns1132Compliance(n string) string {
	reg, _ := regexp.Compile(`[^A-Za-z0-9.\-]+`)
	return strings.ToLower(reg.ReplaceAllString(n, "-"))
}

func (c *federationCompleter) Executor(action string, args ...string) {
	if len(args) == 0 {
		fmt.Printf("Federation operations missing argument.\n")
		return
	}
	defer c.root.kubectl.SetContext("")
	switch action {
	case fedActionAdd:
		c.execAddClusters(args[0], args[1:]...)
	case fedActionRemove:
		c.execRemoveClusters(args[0], args[1:]...)
	case fedActionSet:
		c.execSetClusters(args[0], args[1:]...)
	case fedActionGet:
		c.execGetOrDescribeResources(args[0], fedActionGet, args[1:]...)
	case fedActionDescribe:
		c.execGetOrDescribeResources(args[0], fedActionDescribe, args[1:]...)
	}
}

func (c *federationCompleter) execCheckClusters(infed bool, rtype string, args ...string) ([]string, bool) {
	if rtype != fedResourceCluster {
		fmt.Printf("Clusters operation with invalid type: %s\n", rtype)
		return nil, false
	}
	if len(args) == 0 {
		fmt.Printf("Cluster with no clusters provided")
		return nil, false
	}
	clusters := c.getClusters(infed, false)
	ret := make([]string, 0)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if _, ok := clusters[arg]; !ok {
			fmt.Printf("Clusters operation with cluster %s not known or unavailable.\n", arg)
			return nil, false
		}
		ret = append(ret, arg)
	}

	return ret, true
}

func (c *federationCompleter) execAddClusters(rtype string, args ...string) {
	clusters, ok := c.execCheckClusters(false, rtype, args...)
	if !ok {
		return
	}
	for _, cluster := range clusters {
		c.root.kubectl.SetContext(c.root.clusterContextMap[cluster])
		err := utils.ConfigureK8s(c.root.kubectl,
			templates.ClusterClaimParameters{Value: dns1132Compliance(cluster)},
			templates.ClusterIDClaim, false)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		err = utils.ConfigureK8s(c.root.kubectl,
			templates.ClusterClaimParameters{Value: dns1132Compliance(c.name)},
			templates.ClusterSetClaim, false)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}
	// Trigger to recompute Federation membership.
	c.root.initializeFederations()
	smap := c.getClusters(true, true)
	if len(smap) == 0 {
		return
	}
	supervisors := make([]string, 0, len(smap))
	for k := range smap {
		supervisors = append(supervisors, k)
	}
	c.execSetupSupervisorClusters(supervisors)
}

func (c *federationCompleter) execSetupSupervisorClusters(clusters []string) {
	fmt.Printf("Setup federations in supervisor clusters %v\n", clusters)
	secretPrinter := printers.YAMLPrinter{}
	secrets := bytes.NewBuffer(nil)
	fedParams := templates.FederationParameters{ClusterSet: c.name}
	for cl := range c.getClusters(true, false) {
		client := c.root.clusters[cl].client
		secretList := &v1.SecretList{}
		_ = client.List(context.Background(), secretList, client2.InNamespace(antreaNS))
		var secret *v1.Secret
		for i := range secretList.Items {
			s := &secretList.Items[i]
			if strings.HasPrefix(s.Name, antreaSupervisorServiceAccount) {
				secret = s
				break
			}
		}
		if secret == nil {
			fmt.Printf("Unable to find secret in member cluster %s\n", cl)
			continue
		}
		secret.ObjectMeta = v12.ObjectMeta{
			Name:        "supervisor-for-" + dns1132Compliance(cl),
			Namespace:   antreaNS,
			Annotations: map[string]string{userSupervisorSecretAnnotation: cl},
		}
		secret.TypeMeta = v12.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}
		secret.Type = v1.SecretTypeOpaque
		_ = secretPrinter.PrintObj(secret, secrets)

		c.root.kubectl.SetContext("")
		server, _ := c.root.kubectl.Cmd(
			fmt.Sprintf(`config view -o jsonpath='{.clusters[?(@.name=="%s")].cluster.server}'`, cl))
		fedParams.Members = append(fedParams.Members, templates.ClusterParameters{
			Server:    server,
			Secret:    secret.Name,
			ClusterID: dns1132Compliance(cl),
		})
	}

	for _, cluster := range clusters {
		c.root.kubectl.SetContext(c.root.clusterContextMap[cluster])
		err := c.root.kubectl.Apply("", secrets.Bytes())
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		err = utils.ConfigureK8s(c.root.kubectl,
			fedParams,
			templates.Federation, false)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}
}

func (c *federationCompleter) execUnsetSupervisorClusters(clusters []string) {
	for _, cluster := range clusters {
		c.root.kubectl.SetContext(c.root.clusterContextMap[cluster])
		resources := make([]string, 0)
		for _, i := range c.root.getResource(fedResourceFederation, antreaNS, cluster) {
			resources = append(resources, i.(*v1alpha1.Federation).Name)
		}
		if len(resources) > 0 {
			_, err := c.root.kubectl.Cmd(fmt.Sprintf("delete %s %s -n %s",
				fedResourceFederation, strings.Join(resources, " "), antreaNS))
			if err != nil {
				fmt.Println(err)
			}
		}
		resources = nil
		for _, i := range c.root.getResource(fedResourceSecret, antreaNS, cluster) {
			resources = append(resources, i.(*v1.Secret).Name)
		}
		if len(resources) == 0 {
			continue
		}
		_, err := c.root.kubectl.Cmd(fmt.Sprintf("delete %s %s -n %s",
			fedResourceSecret, strings.Join(resources, " "), antreaNS))
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (c *federationCompleter) execSetClusters(rtype string, args ...string) {
	clusters, ok := c.execCheckClusters(true, rtype, args...)
	if !ok {
		return
	}
	options := c.getOptions(fedActionSet, rtype, args)
	setSupervisor := options[0].(bool)
	if !setSupervisor {
		c.execUnsetSupervisorClusters(clusters)
		return
	}
	c.execSetupSupervisorClusters(clusters)
}

func (c *federationCompleter) execRemoveClusters(rtype string, args ...string) {
	clusters, ok := c.execCheckClusters(true, rtype, args...)
	if ok {
		c.execUnsetSupervisorClusters(clusters)
		for _, cluster := range clusters {
			c.root.kubectl.SetContext(c.root.clusterContextMap[cluster])
			resources := make([]string, 0)
			for _, i := range c.root.getResource(fedResourceClusterClaim, antreaNS, cluster) {
				resources = append(resources, i.(*v1alpha1.ClusterClaim).Name)
			}
			if len(resources) == 0 {
				continue
			}
			_, err := c.root.kubectl.Cmd(fmt.Sprintf("delete %s %s -n %s",
				fedResourceClusterClaim, strings.Join(resources, " "), antreaNS))
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	// Trigger to recompute Federation membership.
	c.root.initializeFederations()
	smap := c.getClusters(true, true)
	if len(smap) == 0 {
		return
	}
	supervisors := make([]string, 0, len(smap))
	for k := range smap {
		supervisors = append(supervisors, k)
	}
	c.execSetupSupervisorClusters(supervisors)
}

func (c *federationCompleter) execGetOrDescribeResources(rtype, action string, options ...string) {
	supported := false
	for _, s := range getResourceTypeSuggestions() {
		if s.Text == rtype {
			supported = true
			break
		}
	}
	if !supported {
		fmt.Printf("Unsupported resource type %s\n", rtype)
		return
	}

	clusters := c.getClusters(true, false)
	for cluster := range clusters {
		c.root.kubectl.SetContext(cluster)
		fmt.Println(cluster + ":")
		if rtype == fedResourceCluster {
			c.execPrintResources(c.root.getResource(fedResourceServiceAccount, antreaNS, cluster),
				action, fedResourceServiceAccount, options...)
			c.execPrintResources(c.root.getResource(fedResourceSecret, antreaNS, cluster),
				action, fedResourceSecret, options...)
			c.execPrintResources(c.root.getResource(fedResourceRole, antreaNS, cluster),
				action, fedResourceRole, options...)
			c.execPrintResources(c.root.getResource(fedResourceRoleBinding, antreaNS, cluster),
				action, fedResourceRoleBinding, options...)
			c.execPrintResources(c.root.getResource(fedResourceClusterClaim, antreaNS, cluster),
				action, fedResourceClusterClaim, options...)
			c.execPrintResources(c.root.getResource(fedResourceFederation, antreaNS, cluster),
				action, fedResourceFederation, options...)
		} else {
			c.execPrintResources(c.root.getResource(rtype, "", cluster),
				action, rtype, options...)
		}
	}
}

func (c *federationCompleter) execPrintResources(objs []runtime.Object, action, rtype string, options ...string) {
	if len(objs) == 0 {
		return
	}
	objMap := make(map[string][]string)
	for _, obj := range objs {
		accessor, _ := meta.Accessor(obj)
		ns := accessor.GetNamespace()
		n := accessor.GetName()
		objMap[ns] = append(objMap[ns], n)
	}
	for ns, resources := range objMap {
		args := []string{action, rtype}
		if len(ns) > 0 {
			args = append(args, "-n", ns)
		}
		args = append(args, resources...)
		args = append(args, options...)
		fmt.Println(rtype + ":")
		out, _ := c.root.kubectl.Cmd(strings.Join(args, " "))
		fmt.Println(out)
	}
}
