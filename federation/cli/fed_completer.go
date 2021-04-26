/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"strings"

	"github.com/c-bata/go-prompt"

	"github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	"github.com/vmware-tanzu/antrea/federation/cli/kube-prompt"
)

type federationCompleter struct {
	name string
	root *rootCompleter
}

func (c *federationCompleter) Complete(d prompt.Document) []prompt.Suggest {
	args := strings.Split(d.TextBeforeCursor(), " ")
	w := d.GetWordBeforeCursor()
	// If PIPE is in text before the cursor, returns empty suggestions.
	for i := range args {
		if args[i] == "|" {
			return []prompt.Suggest{}
		}
	}
	// If word before the cursor starts with "-", returns CLI flag options.
	if strings.HasPrefix(w, "-") {
		return c.optionCompleter(args, strings.HasPrefix(w, "--"))
	}
	namespace := kube.CheckNamespaceArg(d)
	commandArgs, skipNext := c.excludeOptions(args)
	if skipNext {
		// when type 'get pod -o ', we don't want to complete pods. we want to type 'json' or other.
		// So we need to skip argumentCompleter.
		return []prompt.Suggest{}
	}
	return c.argumentsCompleter(namespace, commandArgs)
}

func (c *federationCompleter) getClusters(inFed, supervisorOnly bool) map[string]struct{} {
	ret := make(map[string]struct{})
	_ = c.root.getResource(fedResourceClusterClaim, "", "")
	_ = c.root.getResource(fedResourceFederation, "", "")
	for cl, objs := range c.root.resourceCache[fedResourceClusterClaim] {
		fedid := ""
		isSupervisor := false
		for _, i := range objs {
			claim := i.(*v1alpha1.ClusterClaim)
			if claim.Spec.Name == v1alpha1.WellKnownClusterClaimClusterSet {
				fedid = claim.Spec.Value
				break
			}
		}
		if len(fedid) > 0 {
			if feds, ok := c.root.resourceCache[fedResourceFederation][cl]; ok && len(feds) > 0 {
				isSupervisor = true
			}
		}
		if supervisorOnly {
			if isSupervisor {
				ret[cl] = struct{}{}
			}
			continue
		}
		if len(fedid) == 0 && !inFed {
			ret[cl] = struct{}{}
		} else if inFed && fedid == c.name {
			ret[cl] = struct{}{}
		}
	}
	return ret
}
