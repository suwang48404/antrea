/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package fedmanager

import (
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
)

type federationClient struct {
	clusterclient.ClusterClient
	// Channel to stop Client from monitor resources.
	resourceStop chan<- struct{}
	// Channel to stop Client from joining leader election.
	leaderStop chan<- struct{}
	// Is FederationManager leader of this cluster.
	isManagedBy     bool
	npPolicyChanged bool
}

func (c *federationClient) start(m *federationManager) (err error) {
	log := m.log.WithName("Client")
	log.V(1).Info("Starting cluster", "Cluster", c.GetClusterID())
	if c.leaderStop, err = c.StartLeaderElection(m.queue); err != nil {
		return
	}
	if c.resourceStop, err = c.StartMonitorResources(m.queue); err != nil {
		return
	}
	return
}

func (c *federationClient) stop(m *federationManager) {
	log := m.log.WithName("Client")
	log.V(1).Info("Stopping  cluster", "Cluster", c.GetClusterID())
	c.leaderStop <- struct{}{}
	close(c.leaderStop)
	c.resourceStop <- struct{}{}
	close(c.resourceStop)
}
