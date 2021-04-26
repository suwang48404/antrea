/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterStatus string

const (
	ClusterStatusReachable   ClusterStatus = "Reachable"
	ClusterStatusUnreachable ClusterStatus = "Unreachable"
)

type MemberCluster struct {
	// Identify member cluster in federation.
	ClusterID string `json:"clusterID,omitempty"`
	// API server of the member cluster.
	Server string `json:"server,omitempty"`
	// Secret name to access API server of the member cluster.
	Secret string `json:"secret,omitempty"`
}

type MemberClusterStatus struct {
	// Identify member cluster in federation.
	ClusterID string `json:"clusterID,omitempty"`
	// Identify if leader.
	IsLeader bool `json:"isLeader,omitempty"`
	// Status indicates connection status
	Status ClusterStatus `json:"status,omitempty"`
	// Error is current error, if any.
	Error string `json:"error,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FederationSpec defines the desired state of Federation.
type FederationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Member clusters.
	Members []MemberCluster `json:"members,omitempty"`
}

// FederationStatus defines the observed state of Federation.
type FederationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Member cluster status.
	MemberStatus []MemberClusterStatus `json:"memberStatus,omitempty"`
}

// +kubebuilder:object:root=true

// +kubebuilder:subresource:status
// Federation is the Schema for the federations API.
type Federation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FederationSpec   `json:"spec,omitempty"`
	Status FederationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FederationList contains a list of Federation.
type FederationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Federation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Federation{}, &FederationList{})
}
