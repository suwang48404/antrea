/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package cli

import (
	"github.com/c-bata/go-prompt"
)

const (
	argCluster         = "cluster"
	argFederation      = "federation"
	argDiscoverCluster = "discover-clusters"
	argListClusters    = "list-clusterContexts"
	argListFederations = "list-federations"
)

var rootArguments = []prompt.Suggest{
	{Text: argCluster, Description: "Cluster scope command"},
	{Text: argFederation, Description: "Federation scope command"},
	{Text: argDiscoverCluster, Description: "Discover clusters in kubeconfig directory"},
	{Text: argListClusters, Description: "List cluster contexts"},
	{Text: argListFederations, Description: "List federations"},
}
