/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Identify this cluster.
	WellKnownClusterClaimID = "id.k8s.io"
	// Identify a clusterSet that this cluster is member of.
	WellKnownClusterClaimClusterSet = "clusterset.k8s.io"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterClaimSpec defines the desired state of ClusterClaim.
type ClusterClaimSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name of the ClusterClaim
	Name string `json:"name,omitempty"`

	// Value of the ClusterClaim
	Value string `json:"value,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterClaim is the Schema for the clusterclaims API.
type ClusterClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterClaimSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterClaimList contains a list of ClusterClaim.
type ClusterClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterClaim{}, &ClusterClaimList{})
}
