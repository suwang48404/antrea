/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package clusterclient

import (
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
)

// ClusterClient allows FederationManager to have access to a K8s cluster that it manages.
type ClusterClient interface {
	// Grant FederationManager read/write to the cluster.
	client.Client

	// GetClusterID returns clusterID of the cluster the client connected to.
	GetClusterID() ClusterID

	// GetClusterID returns status of the cluster the client connected to.
	GetClusterStatus() *v1alpha1.MemberClusterStatus

	// StartLeaderElection asks client to start leader election for this cluster and sends LeaderEvent to queue.
	// The client shall return a channel that later notifies the client to stop the leader election.
	// Note resource monitoring and leader change are orthogonal.
	StartLeaderElection(queue workqueue.RateLimitingInterface) (chan<- struct{}, error)

	// StartMonitorResources asks client to start to monitor and send resource changes to queue.
	// The client shall return a channel that notifies client to stop the monitoring resources.
	// Note resource monitoring and leader elections are orthogonal.
	StartMonitorResources(queue workqueue.RateLimitingInterface) (chan<- struct{}, error)
}
