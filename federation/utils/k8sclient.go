/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package utils

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// NewK8sClient returns K8s client interface.
func NewK8sClient(scheme *runtime.Scheme, cluster string) (client.Client, error) {
	var conf *rest.Config
	var err error
	if len(cluster) > 0 {
		conf, err = config.GetConfigWithContext(cluster)
		if err != nil {
			return nil, err
		}
	} else {
		conf = config.GetConfigOrDie()
	}
	k8sClient, err := client.New(conf, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return k8sClient, nil
}
