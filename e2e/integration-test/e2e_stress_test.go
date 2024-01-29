/*
Copyright 2022.

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

package integrationtest

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

const stressNGDeploymentYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stress-ng-cpu
spec:
  selector:
    matchLabels:
      app: stress-ng-cpu
  replicas: 2
  template:
    metadata:
      labels:
        app: stress-ng-cpu
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534 # nobody
      containers:
        - name: stressn-ng
          image: quay.io/sustainable_computing_io/stress-ng:latest
          resources:
            requests:
              cpu: 1
              memory: 500Mi
          command: ["/bin/sh"]
          args:
            - "-c"
            - "stress-ng --cpu 1 --temp-path /tmp --cpu-load 20"
`

func (kmc *TestKeplerMetric) createdStressNGDeployment() error {
	var deployment appsv1.Deployment
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err := decode([]byte(stressNGDeploymentYaml), nil, &deployment)
	if err != nil {
		return fmt.Errorf("Failed to decode deployment manifest: %v\n", err)
	}

	namespace := "default"
	_, err = kmc.Client.AppsV1().Deployments(namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("Deployment %s already exists, skipping creation\n", deployment.Name)
		return nil
	}
	createdDeployment, err := kmc.Client.AppsV1().Deployments(namespace).Create(context.TODO(), &deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to create deployment: %v\n", err)
	}

	fmt.Printf("Deployment created: %s\n", createdDeployment.Name)

	// Wait and watch the deployment status till it is running
	watch, err := kmc.Client.AppsV1().Deployments(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", createdDeployment.Name),
	})
	if err != nil {
		return fmt.Errorf("Failed to watch deployment: %v\n", err)
	}

	// Define a channel to receive events from the watch
	eventCh := watch.ResultChan()

	for event := range eventCh {
		deployment, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			return fmt.Errorf("Failed to convert object to Deployment")
		}

		if deployment.Status.ReadyReplicas == deployment.Status.Replicas &&
			deployment.Status.UpdatedReplicas == deployment.Status.Replicas &&
			deployment.Status.AvailableReplicas == deployment.Status.Replicas {
			// Deployment is ready and all replicas are running
			break
		}

		// Wait for a short duration before checking the status again
		time.Sleep(5 * time.Second)
	}

	fmt.Printf("Deployment is ready and all replicas are running\n")
	// sleep to allow the stress-ng to generate load and get the metrics
	time.Sleep(30 * time.Second)

	return nil
}
