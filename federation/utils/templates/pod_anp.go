/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package templates

type EntitySelectorParameters struct {
	Kind string
	Name string
}

type NamespaceParameters struct {
	Labels map[string]string
}

type ToFromParameters struct {
	Entity    *EntitySelectorParameters
	Namespace *NamespaceParameters
	IPBlock   string
	Ports     []*PortParameters
	DenyAll   bool
}

type PortParameters struct {
	Protocol string
	Port     string
}

type PodANPParameters struct {
	Name        string
	Namespace   string
	PodSelector string
	To          *ToFromParameters
	Port        *PortParameters
	Action      string
}

const PodAntreaNetworkPolicy = `
apiVersion: crd.antrea.io/v1alpha1
kind: NetworkPolicy
metadata:
  name: {{.Name}}
  namespace : {{.Namespace}}
spec:
  priority: 1
  appliedTo:
  - podSelector:
      matchLabels:
        app: {{.PodSelector}}
  egress:
{{ if .To }}
  - action: {{ .Action }}
    to:
{{ if .To.IPBlock }}
    - ipBlock:
        cidr: {{.To.IPBlock}}
{{ end }}
{{ if .To.Entity }}
    - externalEntitySelector:
        matchLabels:
{{ if .To.Entity.Kind }}
          kind.antrea.io: {{.To.Entity.Kind}}
{{ end }}
{{ if .To.Entity.Name }}
          name.antrea.io: {{ .To.Entity.Name }}
{{ end }}
{{ end }}{{/* .To.Entity */}}
{{ if .To.Namespace }}
      namespaceSelector:
          matchLabels:
{{ range $k, $v := .To.Namespace.Labels }}
          {{$k}}: {{$v}}
{{ end }}
{{ end }}{{/* .To.Namespace */}}
{{ end }} {{/* .To */}}
{{ if .Port }}
    ports:
      - protocol: {{.Port.Protocol}}
{{ if .Port.Port }}
        port: {{.Port.Port}}
{{ end }}
{{ end }} {{/* .Port */}}
  - action: Allow
    to:
    - podSelector: {}
  - action: Drop
    to:
    - ipBlock:
        cidr: 0.0.0.0/0

`
