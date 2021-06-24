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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"antrea.io/antrea/pkg/apis/crd/v1alpha2"
	mcs "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceImportSpec defines the desired state of ResourceImport
type ResourceImportSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ClusterID specifies the member cluster this resource to import to.
	// When not specified, import to all member clusters.
	ClusterID string `json:"clusterID,omitempty"`
	// Name of imported resource.
	Name string `json:"name,omitempty"`
	// Namespace of imported resource.
	Namespace string `json:"namespace,omitempty"`
	// Kind of imported resource.
	Kind string `json:"kind,omitempty"`

	// If imported resource is ServiceImport.
	ServiceImport mcs.ServiceImport `json:"serviceImport,omitempty"`
	// If imported resource is EndPoints.
	Endpoints []v1.EndpointSubset `json:"endpoints,omitempty"`
	// If imported resource is ExternalEntity.
	ExternalEntity *v1alpha2.ExternalEntitySpec `json:"externalentity,omitempty"`
	// If imported resource is Node (IPs)
	Node v1.NodeStatus `json:"node,omitempty"`
	// If imported resource is ANP.
	// TODO:
	// ANP uses float64 as priority.  Type float64 is discouraged by k8s, and is not supported by controller-gen tools.
	// NetworkPolicy *v1alpha1.NetworkPolicySpec `json:"networkpolicy,omitempty"`
	// If imported resource Kind is unknown.
	Raw []byte `json:"raw,omitempty"`
}

// MemberClusterImportStatus defines ResourceImport status from one member cluster.
type MemberClusterImportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ClusterID is the member cluster ID.
	ClusterID string `json:"clusterID,omitempty"`
	// Error states the reason if import failed.
	Error string `json:"error,omitempty"`
}

// ResourceImportStatus defines the observed state of ResourceImport
type ResourceImportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// MemberClusterImportStatus reports resource import status of member clusters.
	MemberClusterImportStatus []MemberClusterImportStatus `json:"memberClusterImportStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ResourceImport is the Schema for the resourceimports API
type ResourceImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceImportSpec   `json:"spec,omitempty"`
	Status ResourceImportStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceImportList contains a list of ResourceImport
type ResourceImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceImport{}, &ResourceImportList{})
}
