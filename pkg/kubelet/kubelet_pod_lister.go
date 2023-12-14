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

package kubelet

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
)

type KubeletPodLister struct{}

const (
	saPath         = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	nodeEnv        = "NODE_IP"
	kubeletPortEnv = "KUBELET_PORT"
)

var (
	podURL string
)

func init() {
	nodeName := os.Getenv(nodeEnv)
	if nodeName == "" {
		nodeName = "localhost"
	}
	port := os.Getenv(kubeletPortEnv)
	if port == "" {
		port = "10250"
	}
	podURL = "https://" + nodeName + ":" + port + "/pods"
}

func httpGet(url string) (*http.Response, error) {
	objToken, err := os.ReadFile(saPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read from %q: %v", saPath, err)
	}
	token := string(objToken)

	var bearer = "Bearer " + token
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", bearer)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from %q: %v", url, err)
	}
	return resp, err
}

// ListPods obtains PodList
func (k *KubeletPodLister) ListPods() (*[]corev1.Pod, error) {
	resp, err := httpGet(podURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	podList := corev1.PodList{}
	err = json.Unmarshal(body, &podList)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %v", err)
	}

	pods := &podList.Items

	return pods, nil
}
