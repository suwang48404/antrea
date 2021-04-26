/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"github.com/c-bata/go-prompt"

	"github.com/vmware-tanzu/antrea/federation/cli/kube-prompt"
)

var (
	federationOptions = map[string]map[string][]prompt.Suggest{
		fedActionSet: {
			fedResourceCluster: {{Text: "--supervisor", Description: "Set clusters to supervisory role"}},
		},
		fedActionRemove: {
			fedResourceCluster: {},
		}, // Remove has no options
		fedActionAdd: {
			fedResourceCluster: {},
		}, // Add has no options

	}
)

func (c *federationCompleter) optionCompleter(args []string, long bool) []prompt.Suggest {
	if len(args) < 2 {
		return nil
	}
	// option do not support parameters.
	options, ok := federationOptions[args[0]][args[1]]
	if !ok {
		return kube.OptionCompleter(args, long)
	}
	return prompt.FilterContains(options, args[len(args)-1], true)
}

func (c *federationCompleter) getOptions(action, rtype string, args []string) []interface{} {
	ret := make([]interface{}, 0)
	// Return option values in order that is declared.
	options := federationOptions[action][rtype]
	for _, opt := range options {
		found := false
		for _, arg := range args {
			// HACK,only true/false options
			if opt.Text == arg {
				ret = append(ret, true)
				found = true
				break
			}
		}
		if !found {
			ret = append(ret, false)
		}
	}
	return ret
}

func (c *federationCompleter) excludeOptions(args []string) ([]string, bool) {
	_, ok := federationOptions[args[0]]
	if !ok {
		return kube.ExcludeOptions(args)
	}
	cmdArgs := make([]string, 0)
	for _, arg := range args {
		if len(arg) >= 1 && arg[0:1] == "-" {
			continue
		}
		cmdArgs = append(cmdArgs, arg)
	}
	return cmdArgs, false
}
