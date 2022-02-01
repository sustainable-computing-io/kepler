/*
Copyright 2021.

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

package pod_lister

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
)

type KubeletPodLister struct{}

const (
	saPath         = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	nodeEnv        = "NODE_NAME"
	kubeletPortEnv = "KUBELET_PORT"
)

var (
	url string
)

func init() {
	nodeName := os.Getenv(nodeEnv)
	if len(nodeName) == 0 {
		nodeName = "localhost"
	}
	port := os.Getenv(kubeletPortEnv)
	if len(port) == 0 {
		port = "10250"
	}
	url = "https://" + nodeName + ":" + port + "/pods"
}

// ListPods accesses Kubelet's metrics and obtain PodList
func (k *KubeletPodLister) ListPods() (*[]corev1.Pod, error) {
	objToken, err := ioutil.ReadFile(saPath)
	if err != nil {
		log.Fatalf("failed to read from %q: %v", saPath, err)
	}
	token := string(objToken)

	var bearer = "Bearer " + token
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", bearer)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to get response from %q: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read response body: %v", err)
	}
	podList := corev1.PodList{}
	err = json.Unmarshal(body, &podList)
	if err != nil {
		log.Fatalf("failed to parse response body: %v", err)
	}

	pods := &podList.Items

	return pods, nil
}
