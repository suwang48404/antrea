/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package templates

type ClusterClaimParameters struct {
	Value string
}

const ClusterIDClaim = `
apiVersion: mcs.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name: {{.Value}}
  namespace: antrea-system
spec:
  name: id.k8s.io
  value: {{.Value}}
`

const ClusterSetClaim = `
apiVersion: mcs.crd.antrea.io/v1alpha1
kind: ClusterClaim
metadata:
  name:  {{.Value}}
  namespace: antrea-system
spec:
  name: clusterset.k8s.io
  value: {{.Value}}
`
