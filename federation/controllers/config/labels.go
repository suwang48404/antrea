/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package config

const (
	// Well known labels on ExternalEntities so that they can be selected by Antrea NetworkPolicies.
	ExternalEntityLabelKeyPostfix              = "antrea.io"
	ExternalEntityLabelKeyNamespace            = "namespace." + ExternalEntityLabelKeyPostfix // TODO, useless, remove later??
	ExternalEntityLabelKeyKind                 = "kind." + ExternalEntityLabelKeyPostfix
	ExternalEntityLabelKeyName                 = "name." + ExternalEntityLabelKeyPostfix
	ExternalEntityLabelKeyTagPostfix           = ".tag." + ExternalEntityLabelKeyPostfix
	ExternalEntityLabelKeyFederatedServiceKind = "federated-service"
)
