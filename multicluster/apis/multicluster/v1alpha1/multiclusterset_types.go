/*
Copyright 2021 Antrea Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ClusterStatus string

const (
	ClusterStatusReachable   ClusterStatus = "Reachable"
	ClusterStatusUnreachable ClusterStatus = "Unreachable"
)

// MemberCluster defines member cluster information.
type MemberCluster struct {
	// Identify member cluster in multi-cluster set.
	ClusterID string `json:"clusterID,omitempty"`
	// API server of the member or the leader cluster.
	Server string `json:"server,omitempty"`
	// Secret name to access API server of the member from the leader cluster.
	Secret string `json:"secret,omitempty"`
	// ServiceAccount used by the member cluster to access into leader cluster.
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// MemberClusterStatus defines member cluster status in the leader clusters.
type MemberClusterStatus struct {
	// ClusterID identifies member cluster in the set.
	ClusterID string `json:"clusterID,omitempty"`
	// IsLeader is true if the member cluster has selected this cluster as leader.
	IsLeader bool `json:"isLeader,omitempty"`
	// Status indicates connection status between leader and member cluster.
	Status ClusterStatus `json:"status,omitempty"`
	// Error reports of the member cluster.
	Error string `json:"error,omitempty"`
}

// LeaderClusterStatus defines leader cluster status in the member clusters.
type LeaderClusterStatus struct {
	// ClusterID identifies leader cluster in the set.
	ClusterID string `json:"clusterID,omitempty"`
	// Status indicates connection status between leader and member cluster.
	Status ClusterStatus `json:"status,omitempty"`
	// Error reports of the leader cluster.
	Error string `json:"error,omitempty"`
}

// MultiClusterSetSpec defines the desired state of MultiClusterSet
type MultiClusterSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Members include member clusters known to the leader clusters.
	// Used in leader cluster.
	Members []MemberCluster `json:"members,omitempty"`
	// Leaders include leader clusters known to the member clusters.
	Leaders []MemberCluster `json:"leaders,omitempty"`
	// Namespace to connect to in leader clusters,.
	// Used in member cluster.
	Namespace string `json:"namespace,omitempty"`
}

// MultiClusterSetStatus defines the observed state of MultiClusterSet
type MultiClusterSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	MemberStatus []MemberClusterStatus `json:"memberStatus,omitempty"`
	LeaderStatus []LeaderClusterStatus `json:"leaderStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MultiClusterSet is the Schema for the multiclustersets API
type MultiClusterSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterSetSpec   `json:"spec,omitempty"`
	Status MultiClusterSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MultiClusterSetList contains a list of MultiClusterSet
type MultiClusterSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiClusterSet{}, &MultiClusterSetList{})
}
