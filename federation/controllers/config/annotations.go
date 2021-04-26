/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package config

// User defined annotations.
const (
	UserFedAnnotationPostfix = ".fed.user.antrea.io"
	// UserServiceFedAnnotations are added by users to K98 Service
	// indicating associated federated Services.
	UserFedServiceAnnotation = "svc" + UserFedAnnotationPostfix
	// UserFedANPAnnotations are added by users to ANPs indicating these ANPs
	// are federation scope.
	UserFedANPAnnotation = "anp" + UserFedAnnotationPostfix
)

// Antrea managed resources.
const (
	// FederationIDAnnotation specifies federation ID associated with a federated resource.
	FederationIDAnnotation = "id.fed.antrea.io"
)
