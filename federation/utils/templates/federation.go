/*
 *  Copyright  (c) 2021 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package templates

type ClusterParameters struct {
	Server    string
	Secret    string
	ClusterID string
}

type FederationParameters struct {
	ClusterSet string
	Members    []ClusterParameters
}

const Federation = `
apiVersion: mcs.crd.antrea.io/v1alpha1
kind: Federation
metadata:
  name: {{.ClusterSet}}
  namespace: antrea-system
spec:
   members:
{{- range $m := .Members }}
     - clusterID: {{$m.ClusterID}}
       server: {{$m.Server}}
       secret: {{$m.Secret}}
{{ end }}
`
