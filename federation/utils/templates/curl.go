/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package templates

type DeploymentParameters struct {
	Name      string
	Namespace string
	Replicas  int
}

const CurlDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    name: {{.Name}}
spec:
  selector:
    matchLabels:
      app: curl
  replicas: {{.Replicas}}
  template:
    metadata:
      labels:
        app: curl
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - curl
              topologyKey: "kubernetes.io/hostname"
      containers:
        - name: {{.Name}}
          image: byrnedo/alpine-curl
          imagePullPolicy: Never
          command:
            - "sh"
            - "-c"
            - >
              while true; do
                sleep 1;
              done
      terminationGracePeriodSeconds: 0
`
