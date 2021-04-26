/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/c-bata/go-prompt"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"

	federationv1alpha1 "github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	"github.com/vmware-tanzu/antrea/federation/cli/kube-prompt"
	"github.com/vmware-tanzu/antrea/federation/utils"
)

type Completer interface {
	Complete(d prompt.Document) []prompt.Suggest
}

type Executor interface {
	Executor(string)
}

var (
	_ Completer = &clusterCompleter{}
	_ Completer = &federationCompleter{}
	_ Executor  = &rootCompleter{}
	_ Completer = &rootCompleter{}
)

type clusterCompleter struct {
	kube.Completer
	client client.Client
}

func (c *clusterCompleter) Complete(d prompt.Document) []prompt.Suggest {
	return c.Completer.Complete(d)
}

func (c *clusterCompleter) supportFed() bool {
	return c.client != nil
}

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = antreatypes.AddToScheme(scheme)
	_ = federationv1alpha1.AddToScheme(scheme)
	_ = antreanetworking.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

type rootCompleter struct {
	kubectl     *utils.KubeCtl
	clusters    map[string]*clusterCompleter
	federations map[string]*federationCompleter
	// resource type map of cluster map of resource instances list.
	resourceCache     map[string]map[string][]runtime.Object
	clusterContextMap map[string]string
	tmpFederation     *federationCompleter
}

func (c *rootCompleter) initializeFederations() {
	c.federations = make(map[string]*federationCompleter)
	c.tmpFederation = nil
	delete(c.resourceCache, fedResourceClusterClaim)
	_ = c.getResource(fedResourceClusterClaim, antreaNS, "")
	for _, objs := range c.resourceCache[fedResourceClusterClaim] {
		for _, i := range objs {
			fed := i.(*federationv1alpha1.ClusterClaim)
			if fed.Spec.Name != federationv1alpha1.WellKnownClusterClaimClusterSet {
				continue
			}
			if _, ok := c.federations[fed.Spec.Value]; !ok {
				c.federations[fed.Spec.Value] = &federationCompleter{name: fed.Spec.Value, root: c}
			}
		}
	}
	// fmt.Printf("Discovered federations %v\n", c.federations)
}

func (c *rootCompleter) initialize() error {
	defer c.kubectl.SetContext("")
	cmd := exec.Command("/bin/bash", "-c", "kubectl config get-contexts")
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get clusters: %v, %s", err, string(bytes))
	}
	lines := strings.Split(string(bytes), "\n")
	for i := 1; i < len(lines); i++ {
		l := lines[i]
		l = strings.Replace(l, "*", "", 1)
		fields := strings.Fields(l)
		if len(fields) == 0 {
			continue
		}
		clusterCompleter := &clusterCompleter{}
		supportFed := true
		c.kubectl.SetContext(fields[0])
		if _, err := c.kubectl.Cmd(fmt.Sprintf("get ns %s", antreaNS)); err != nil {
			supportFed = false
		}
		if supportFed {
			conf, err := config.GetConfigWithContext(fields[0])
			if err != nil {
				fmt.Printf("Cannot retrieve kube config for cluster context %s: %v", fields[0], err)
				os.Exit(1)
			}
			cl, err := client.New(conf, client.Options{Scheme: scheme})
			if err != nil {
				fmt.Printf("Cannot create client for cluster context %s: %v", fields[0], err)
				os.Exit(1)
			}
			clusterCompleter.client = cl
		}
		k, err := kube.NewCompleter(fields[0], clusterCompleter.client)
		if err != nil {
			fmt.Printf("Ignore misconfigured or unreachable cluster %s\n", fields[0])
			continue
		}
		clusterCompleter.Completer = *k
		c.clusters[fields[1]] = clusterCompleter
		c.clusterContextMap[fields[1]] = fields[0]
		fmt.Printf("Discovered cluster %s in context %s support federation %v \n", fields[1], fields[0], clusterCompleter.supportFed())
	}
	c.resetResource()
	c.initializeFederations()
	return nil
}

func NewCompleter() (Completer, error) {
	c := &rootCompleter{
		clusters:          make(map[string]*clusterCompleter),
		federations:       make(map[string]*federationCompleter),
		resourceCache:     make(map[string]map[string][]runtime.Object),
		clusterContextMap: make(map[string]string),
	}
	c.kubectl, _ = utils.NewKubeCtl("")
	if err := c.initialize(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *rootCompleter) Complete(d prompt.Document) []prompt.Suggest {
	// command start with
	// "cluster CLUSTER_NAME" or
	// "federation FED_NAME"
	// then delegate to specific completer.
	suggs := make([]prompt.Suggest, 0)
	if d.TextBeforeCursor() == "" {
		if d.LastKeyStroke() == prompt.Tab {
			return rootArguments
		}
		return suggs
	}
	args := strings.Split(d.TextBeforeCursor(), " ")
	if len(args) <= 1 {
		return prompt.FilterHasPrefix(rootArguments, args[0], true)
	}
	var completer Completer
	switch args[0] {
	case argFederation:
		if len(args) > 2 {
			var ok bool
			completer, ok = c.federations[args[1]]
			if !ok {
				c.tmpFederation = &federationCompleter{name: args[1], root: c}
				completer = c.tmpFederation
			}
			break
		}
		for k := range c.federations {
			suggs = append(suggs, prompt.Suggest{Text: k})
		}
		suggs = prompt.FilterHasPrefix(suggs, args[1], true)
	case argCluster:
		if len(args) > 2 {
			if tmp, ok := c.clusters[args[1]]; ok {
				completer = tmp
			}
			break
		}
		for k := range c.clusters {
			suggs = append(suggs, prompt.Suggest{Text: k})
		}
		if len(args) == 2 {
			suggs = prompt.FilterHasPrefix(suggs, args[1], true)
		}
	default:
		return suggs
	}
	if completer == nil {
		return suggs
	}
	sd := generateSubDocument(&d, 2)
	return completer.Complete(*sd)
}

func generateSubDocument(d *prompt.Document, fieldOffset int) *prompt.Document {
	curPos := utils.GetUnexportedField(reflect.ValueOf(d).Elem().FieldByName("cursorPosition")).(int)
	fields := strings.Fields(d.TextBeforeCursor())
	rd := prompt.NewDocument()
	if len(fields) > fieldOffset {
		// We want to cut off fieldOffset number of fields and white spaces after them.
		// But preserve everything else.
		nonWSCnt := 0
		for i := 0; i < fieldOffset; i++ {
			nonWSCnt += len(fields[i])
		}
		byteOffSet := 0
		for ; byteOffSet < len(d.Text); byteOffSet++ {
			sub := d.Text[byteOffSet : byteOffSet+1]
			if nonWSCnt == 0 && sub != " " {
				break
			}
			if sub != " " {
				nonWSCnt--
			}
		}
		rd.Text = d.Text[byteOffSet:]
		curPos -= byteOffSet
		utils.SetUnexportedField(reflect.ValueOf(rd).Elem().FieldByName("cursorPosition"), curPos)
		utils.SetUnexportedField(reflect.ValueOf(rd).Elem().FieldByName("lastKey"), d.LastKeyStroke())
	}
	// curPos = utils.GetUnexportedField(reflect.ValueOf(rd).Elem().FieldByName("cursorPosition")).(int)
	// fmt.Printf("New doc generated with Text %s, curPos %d, key %d\n", rd.Text, curPos, rd.LastKeyStroke())
	return rd
}
