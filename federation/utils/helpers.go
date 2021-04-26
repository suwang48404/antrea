/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package utils

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"text/template"
	"time"

	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RestartDeployment restarts an existing deployment.
func RestartOrWaitDeployment(k8sClient client.Client, name, namespace string, timeout time.Duration, restart bool) error {
	if !restart {
		return StartOrWaitDeployment(k8sClient, name, namespace, 0, timeout)
	}
	replicas, err := StopDeployment(k8sClient, name, namespace, timeout)
	if err != nil {
		return err
	}
	if err := StartOrWaitDeployment(k8sClient, name, namespace, replicas, timeout); err != nil {
		return err
	}
	return nil
}

// StartOrWaitDeployment start a stopped deployment with number of replicas.
// Or wait for the deployment to complete if replicas is 0.
func StartOrWaitDeployment(k8sClient client.Client, name, namespace string, replicas int32, timeout time.Duration) error {
	dep := &v1.Deployment{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := k8sClient.Get(context.TODO(), key, dep); err != nil {
		return err
	}
	if replicas != 0 {
		if !(dep.Spec.Replicas == nil || *dep.Spec.Replicas == 0) {
			return fmt.Errorf("deployment is not in stopped state")
		}
		dep.Spec.Replicas = &replicas
		if err := k8sClient.Update(context.TODO(), dep); err != nil {
			return err
		}
	} else {
		if dep.Spec.Replicas == nil || *dep.Spec.Replicas == 0 {
			return fmt.Errorf("empty replicas in deployment")
		}
	}
	if err := wait.Poll(time.Second, timeout, func() (bool, error) {
		dep := &v1.Deployment{}
		if err := k8sClient.Get(context.TODO(), key, dep); err != nil {
			return false, err
		}
		if dep.Status.ReadyReplicas != *dep.Spec.Replicas {
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	// Give time for deployment to re-discover.
	time.Sleep(time.Second * 2)
	return nil
}

// StopDeployment stops an deployment..
func StopDeployment(k8sClient client.Client, name, namespace string, timeout time.Duration) (int32, error) {
	dep := &v1.Deployment{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := k8sClient.Get(context.TODO(), key, dep); err != nil {
		return -1, err
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas == 0 {
		return -1, fmt.Errorf("deployment is already stopped")
	}
	replicas := *dep.Spec.Replicas
	*dep.Spec.Replicas = 0
	if err := k8sClient.Update(context.TODO(), dep); err != nil {
		return -1, err
	}
	if err := wait.Poll(time.Second, timeout, func() (bool, error) {
		dep := &v1.Deployment{}
		if err := k8sClient.Get(context.TODO(), key, dep); err != nil {
			return false, err
		}
		if dep.Status.ReadyReplicas != 0 {
			return false, nil
		}
		return true, nil
	}); err != nil {
		return -1, err
	}
	return replicas, nil
}

// Create or delete an configuration in yaml.
func ConfigureK8s(kubeCtl *KubeCtl, params interface{}, yaml string, isDelete bool) error {
	confParser, err := template.New("").Parse(yaml)
	if err != nil {
		return err
	}
	conf := bytes.NewBuffer(nil)
	if err := confParser.Execute(conf, params); err != nil {
		return fmt.Errorf("parse template failed: %v", err)
	}
	// logf.Log.V(1).Info("", "yaml", conf.String())
	if isDelete {
		err = kubeCtl.Delete("", conf.Bytes())
	} else {
		err = kubeCtl.Apply("", conf.Bytes())
	}
	if err != nil {
		return fmt.Errorf("kubectl failed with err %v yaml %v", err, conf.String())
	}
	return nil
}

// GetPodsFromDeployment returns Pods of Deployment.
func GetPodsFromDeployment(k8sClient client.Client, name, namespace string) ([]string, error) {
	replicaSetList := &v1.ReplicaSetList{}
	if err := k8sClient.List(context.TODO(), replicaSetList, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}
	var replicaSet *v1.ReplicaSet
	for _, r := range replicaSetList.Items {
		if len(r.OwnerReferences) > 0 &&
			r.OwnerReferences[0].Controller != nil && *r.OwnerReferences[0].Controller &&
			r.OwnerReferences[0].Kind == reflect.TypeOf(v1.Deployment{}).Name() && r.OwnerReferences[0].Name == name {
			replicaSet = r.DeepCopy()
			break
		}
	}
	if replicaSet == nil {
		logf.Log.V(1).Info("Failed to find ReplicaSet", "Deployment", name)
		return nil, nil
	}
	podList := &v12.PodList{}
	pods := make([]string, 0)
	if err := k8sClient.List(context.TODO(), podList, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}
	for _, p := range podList.Items {
		if len(p.OwnerReferences) > 0 &&
			p.OwnerReferences[0].Controller != nil && *p.OwnerReferences[0].Controller &&
			p.OwnerReferences[0].Kind == reflect.TypeOf(*replicaSet).Name() && p.OwnerReferences[0].Name == replicaSet.Name {
			pods = append(pods, p.Name)
		}
	}
	return pods, nil
}

// GetServiceClusterIPPort returns clusterIP and first port of a service.
func GetServiceClusterIPPort(k8sClient client.Client, name, namespace string) (string, int32, error) {
	service := &v12.Service{}
	key := types.NamespacedName{Name: name, Namespace: namespace}
	if err := k8sClient.Get(context.TODO(), key, service); err != nil {
		return "", 0, err
	}
	return service.Spec.ClusterIP, service.Spec.Ports[0].Port, nil
}
