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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"
)

type KubeletPodLister struct{}

const (
	saPath         = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	nodeEnv        = "NODE_NAME"
	kubeletPortEnv = "KUBELET_PORT"
)

var (
	podUrl, metricsUrl string

	nodeCpuUsageMetricName       = "node_cpu_usage_seconds_total"
	nodeMemUsageMetricName       = "node_memory_working_set_bytes"
	containerCpuUsageMetricName  = "container_cpu_usage_seconds_total"
	containerMemUsageMetricName  = "container_memory_working_set_bytes"
	containerStartTimeMetricName = "container_start_time_seconds"

	containerNameTag = "container"
	podNameTag       = "pod"
	namespaceTag     = "namespace"
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
	podUrl = "https://" + nodeName + ":" + port + "/pods"
	metricsUrl = "https://" + nodeName + ":" + port + "/metrics/resource"
}

func httpGet(url string) (*http.Response, error) {
	objToken, err := ioutil.ReadFile(saPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read from %q: %v", saPath, err)
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
		return nil, fmt.Errorf("failed to get response from %q: %v", url, err)
	}
	return resp, err
}

// ListPods accesses Kubelet's metrics and obtain PodList
func (k *KubeletPodLister) ListPods() (*[]corev1.Pod, error) {
	resp, err := httpGet(podUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
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
func (k *KubeletPodLister) ListMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error) {
	resp, err := httpGet(metricsUrl)
	if err != nil {
		retErr = fmt.Errorf("failed to get response: %v", err)
		return
	}
	defer resp.Body.Close()
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		retErr = fmt.Errorf("failed to parse: %v", err)
		return
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
				value = float64(v.GetCounter().GetValue())
			case dto.MetricType_GAUGE:
				value = float64(v.GetGauge().GetValue())
			}
			switch k {
			case nodeCpuUsageMetricName:
				nodeCPU = value
			case nodeMemUsageMetricName:
				nodeMem = value
			case containerCpuUsageMetricName:
				namespace, pod := parseLabels(v.GetLabel())
				containerCPU[namespace+"/"+pod] = value
				totalContainerCPU += value
			case containerMemUsageMetricName:
				namespace, pod := parseLabels(v.GetLabel())
				containerMem[namespace+"/"+pod] = value
				totalContainerMem += value
			default:
				continue
			}
		}
	}
	systemContainerMem := nodeMem - totalContainerMem
	systemContainerCPU := nodeCPU - totalContainerCPU
	systemContainerName := systemProcessNamespace + "/" + systemProcessName
	containerCPU[systemContainerName] = systemContainerCPU
	containerMem[systemContainerName] = systemContainerMem
	return
}

// GetAvailableMetrics returns containerCpuUsageMetricName and containerMemUsageMetricName if kubelet is connected
func (k *KubeletPodLister) GetAvailableMetrics() []string {
	_, _, _, _, retErr := k.ListMetrics()
	if retErr != nil {
		return []string{}
	}
	return []string{containerCpuUsageMetricName, containerMemUsageMetricName}
}

func parseLabels(labels []*dto.LabelPair) (namespace, pod string) {
	for _, v := range labels {
		if v.GetName() == podNameTag {
			pod = v.GetValue()
		}
		if v.GetName() == namespaceTag {
			namespace = v.GetValue()
		}
	}
	return
}
