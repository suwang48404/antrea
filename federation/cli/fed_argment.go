/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"github.com/c-bata/go-prompt"
)

const (
	fedActionAdd      = "add"
	fedActionRemove   = "remove"
	fedActionSet      = "set"
	fedActionGet      = "get"
	fedActionDescribe = "describe"
)

const (
	fedResourceCluster = "cluster"
	// Resources used in federation configuration.
	fedResourceSecret         = "secret"
	fedResourceServiceAccount = "serviceAccount"
	fedResourceRole           = "clusterrole"
	fedResourceRoleBinding    = "clusterroleBinding"
	fedResourceClusterClaim   = "clusterClaim"
	fedResourceFederation     = "federation"

	// Other resources.
	fedResourceService        = "service"
	fedResourceEndpoints      = "endpoints"
	fedResourceANP            = "networkpolicy.crd.antrea.io"
	fedResourceACNP           = "clusternetworkpolicy.crd.antrea.io"
	fedResourceExternalEntity = "externalentity"
)

var (
	allowedFedResourceTypes = []string{
		fedResourceSecret,
		fedResourceServiceAccount,
		fedResourceRole,
		fedResourceRoleBinding,
		fedResourceClusterClaim,
		fedResourceFederation,
		fedResourceService,
		fedResourceEndpoints,
		fedResourceANP,
		fedResourceACNP,
		fedResourceExternalEntity,
	}
)

func getResourceTypeSuggestions() []prompt.Suggest {
	sugg := []prompt.Suggest{
		{Text: fedResourceCluster, Description: "All resources to configure federation"},
	}
	for _, t := range allowedFedResourceTypes {
		sugg = append(sugg, prompt.Suggest{Text: t})
	}
	return sugg
}

type federationArgument struct {
	// Each federation commands follows, excluding options,
	// action resourceType resourceInstances ...
	action prompt.Suggest
	// Resource types supported by this action.
	types []prompt.Suggest
	// Suggest resource instances to complete command.
	instanceSuggest bool
}

var (
	federationArguments = map[string]federationArgument{
		fedActionAdd: {
			action: prompt.Suggest{Text: fedActionAdd, Description: "Add clusters to the federation"},
			types: []prompt.Suggest{
				{Text: fedResourceCluster, Description: ""},
			},
			instanceSuggest: true,
		},
		fedActionRemove: {
			action: prompt.Suggest{Text: fedActionRemove, Description: "Remove clusters from the federation"},
			types: []prompt.Suggest{
				{Text: fedResourceCluster, Description: ""},
			},
			instanceSuggest: true,
		},
		fedActionSet: {
			action: prompt.Suggest{Text: fedActionSet, Description: "Set clusters attribute in the federation"},
			types: []prompt.Suggest{
				{Text: fedResourceCluster, Description: ""},
			},
			instanceSuggest: true,
		},
		fedActionGet: {
			action: prompt.Suggest{Text: fedActionGet, Description: "Get federated resources from the federation"},
			types:  getResourceTypeSuggestions(),
		},
		fedActionDescribe: {
			action: prompt.Suggest{Text: fedActionDescribe, Description: "Describe federated resources from the federation"},
			types:  getResourceTypeSuggestions(),
		},
	}
)

func (c *federationCompleter) argumentsCompleter(ns string, args []string) []prompt.Suggest {
	fedArgs := federationArguments
	if len(args) <= 1 {
		actions := make([]prompt.Suggest, 0, len(fedArgs))
		for _, arg := range fedArgs {
			actions = append(actions, arg.action)
		}
		prefix := ""
		if len(args) == 1 {
			prefix = args[0]
		}
		return prompt.FilterHasPrefix(actions, prefix, true)
	}
	action, ok := fedArgs[args[0]]
	if !ok {
		return nil
	}
	if len(args) == 2 {
		return prompt.FilterHasPrefix(action.types, args[1], true)
	}
	if !action.instanceSuggest {
		return nil
	}
	rtype := ""
	for _, t := range action.types {
		if t.Text == args[1] {
			rtype = t.Text
			break
		}
	}
	if len(rtype) == 0 {
		return nil
	}
	lastInst := args[len(args)-1]
	prevInsts := args[2 : len(args)-1]
	suggestions := make([]prompt.Suggest, 0)
	if rtype == fedResourceCluster {
		for cl := range c.getClusters(action.action.Text != fedActionAdd, false) {
			suggestions = append(suggestions, prompt.Suggest{Text: cl})
		}
	} else {
		for cl := range c.getClusters(true, false) {
			suggestions = append(suggestions,
				c.root.getResourceSuggestion(c.root.getResource(rtype, ns, cl))...)
		}
	}
	// Filtering out suggestions that is already in the command line.
	for i := 0; i < len(prevInsts); i++ {
		for j := 0; j < len(suggestions); j++ {
			if prevInsts[i] == suggestions[j].Text {
				suggestions[j] = suggestions[len(suggestions)-1]
				suggestions = suggestions[:len(suggestions)-1]
				break
			}
		}
	}
	return prompt.FilterHasPrefix(suggestions, lastInst, true)
}
