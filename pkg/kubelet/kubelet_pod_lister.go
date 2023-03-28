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

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

type KubeletPodLister struct{}

const (
	saPath         = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	nodeEnv        = "NODE_IP"
	kubeletPortEnv = "KUBELET_PORT"
)

var (
	podURL, metricsURL string

	nodeCPUUsageMetricName      = config.KubeletNodeCPU
	nodeMemUsageMetricName      = config.KubeletNodeMemory
	containerCPUUsageMetricName = config.KubeletContainerCPU
	containerMemUsageMetricName = config.KubeletContainerMemory

	podNameTag       = "pod"
	containerNameTag = "container"
	namespaceTag     = "namespace"
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
	metricsURL = "https://" + nodeName + ":" + port + "/metrics/resource"
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

// ListPods accesses Kubelet's metrics and obtain PodList
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

// ListMetrics accesses Kubelet's metrics and obtain pods and node metrics
func (k *KubeletPodLister) ListMetrics() (containerCPU, containerMem map[string]float64, nodeCPU, nodeMem float64, retErr error) {
	resp, err := httpGet(metricsURL)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to get response: %v", err)
	}
	defer resp.Body.Close()

	return parseMetrics(resp.Body)
}

func parseMetrics(r io.ReadCloser) (containerCPU, containerMem map[string]float64, nodeCPU, nodeMem float64, retErr error) {
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to parse: %v", err)
	}
	containerCPU = make(map[string]float64)
	containerMem = make(map[string]float64)
	totalContainerMem := float64(0)
	totalContainerCPU := float64(0)
	for k, family := range mf {
		for _, v := range family.Metric {
			value := float64(0)
			switch family.GetType() {
			case dto.MetricType_COUNTER:
				value = v.GetCounter().GetValue()
			case dto.MetricType_GAUGE:
				value = v.GetGauge().GetValue()
			}
			switch k {
			case nodeCPUUsageMetricName:
				nodeCPU = value
			case nodeMemUsageMetricName:
				nodeMem = value
			case containerCPUUsageMetricName:
				namespace, pod, container := parseLabels(v.GetLabel())
				containerCPU[namespace+"/"+pod+"/"+container] = value
				totalContainerCPU += value
			case containerMemUsageMetricName:
				namespace, pod, container := parseLabels(v.GetLabel())
				containerMem[namespace+"/"+pod+"/"+container] = value
				totalContainerMem += value
			default:
				continue
			}
		}
	}
	systemContainerMem := nodeMem - totalContainerMem
	systemContainerCPU := nodeCPU - totalContainerCPU
	systemContainerName := utils.SystemProcessNamespace + "/" + utils.SystemProcessName
	containerCPU[systemContainerName] = systemContainerCPU
	containerMem[systemContainerName] = systemContainerMem
	return containerCPU, containerMem, nodeCPU, nodeMem, retErr
}

// GetAvailableMetrics returns containerCPUUsageMetricName and containerMemUsageMetricName if kubelet is connected
func (k *KubeletPodLister) GetAvailableMetrics() []string {
	_, _, _, _, retErr := k.ListMetrics()
	if retErr != nil {
		return []string{}
	}
	return []string{containerCPUUsageMetricName, containerMemUsageMetricName}
}

func parseLabels(labels []*dto.LabelPair) (namespace, pod, container string) {
	for _, v := range labels {
		if v.GetName() == podNameTag {
			pod = v.GetValue()
		}
		if v.GetName() == namespaceTag {
			namespace = v.GetValue()
		}
		if v.GetName() == containerNameTag {
			container = v.GetValue()
		}
	}
	return
}
