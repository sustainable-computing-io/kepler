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
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestKeplerMetric represents the structure to test kepler metrics
type TestKeplerMetric struct {
	Metric    map[string][]float64
	Client    *kubernetes.Clientset
	Config    *rest.Config
	Namespace string
	Port      string
	PodLists  []string
}

// NewTestKeplerMetric creates a new TestKeplerMetric instance.
func NewTestKeplerMetric(kubeconfigPath, namespace, port string) (*TestKeplerMetric, error) {
	config, err := getConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &TestKeplerMetric{
		Metric:    make(map[string][]float64),
		PodLists:  make([]string, 0),
		Client:    clientset,
		Config:    config,
		Namespace: namespace,
		Port:      port,
	}, nil
}

// getConfig create and returns a rest client configuration based on the provided kubeconfig path
func getConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	return rest.InClusterConfig()
}

// UpdateMetricMap updates the metric map with a new value.
func (kmc *TestKeplerMetric) UpdateMetricMap(key string, value float64) {
	kmc.Metric[key] = append(kmc.Metric[key], value)
}

// RetrievePodNames retrives names of all pods in all namespaces.
func (kmc *TestKeplerMetric) RetrievePodNames(ctx context.Context) error {
	namespaces, err := kmc.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		pods, err := kmc.Client.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Errorf("error retrieving pods in namespace %s: %v", ns.Name, err)
			return err
		}
		for j := range pods.Items {
			pod := &pods.Items[j]
			kmc.PodLists = append(kmc.PodLists, pod.Name)
		}
	}
	return nil
}

// getEnvOrDefault returns the value of an environment vaiable or a default value if not set.
func getEnvOrDefault(envName, defaultValue string) string {
	if value, exists := os.LookupEnv(envName); exists {
		return value
	}
	return defaultValue
}

// GetMetrics reterives metrics from all pods in the "kepler" namespace.
func (kmc *TestKeplerMetric) GetMetrics(ctx context.Context) error {
	pods, err := kmc.Client.CoreV1().Pods(kmc.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Errorf("error retrieving pods in namespace %s : %v", kmc.Namespace, err)
		return err
	}

	for j := range pods.Items {
		pod := &pods.Items[j]
		if err := kmc.retrieveMetrics(pod); err != nil {
			log.Errorf("error retrieving metrics from pod %s: %v", pod.Name, err)
			return err
		}
	}
	return nil
}

// retrieveMetrics executes command on the given pod and retrieve metrics data.
func (kmc *TestKeplerMetric) retrieveMetrics(pod *v1.Pod) error {
	req := kmc.createExecRequest(pod)
	exec, err := remotecommand.NewSPDYExecutor(kmc.Config, "GET", req.URL())
	if err != nil {
		return err
	}
	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return err
	}
	if stdout.Len() == 0 {
		log.Warnf("received empty response from pod %s", pod.Name)
		return nil
	}
	kmc.PromParse(stdout.Bytes())
	return nil
}

// createExecRequest constructs a request for executing command inside a container in the pod.
func (kmc *TestKeplerMetric) createExecRequest(pod *v1.Pod) *rest.Request {
	return kmc.Client.CoreV1().RESTClient().Get().
		Resource("pods").Name(pod.Name).
		Namespace(kmc.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command:   []string{"curl", "http://localhost:" + kmc.Port + "/metrics"},
			Container: pod.Spec.Containers[0].Name,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)
}

// PromParse parses Prometheus metrics and updates the map.
func (kmc *TestKeplerMetric) PromParse(b []byte) {
	p := textparse.NewPromParser(b)
	log.Info("Parsing Metrics...")
	for {
		et, err := p.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Errorf("error parsing metric: %v", err)
			continue
		}
		if et == textparse.EntrySeries {
			kmc.processSeries(p)
		}
	}
}

// processSeries processes a single series of prometheus metrics data.
func (kmc *TestKeplerMetric) processSeries(p textparse.Parser) {
	var res labels.Labels
	_, _, v := p.Series()
	p.Metric(&res)
	metricName := kmc.constructMetricName(res)
	kmc.UpdateMetricMap(metricName, v)
	res = res[:0]
}

// constructMetricName constructs a unique name for a metric based on its label.
func (kmc *TestKeplerMetric) constructMetricName(res labels.Labels) string {
	if res.Has("pod_name") {
		return res.Get("__name__") + " @ " + res.Get("pod_name")
	}
	return res.Get("__name__")
}

// checkMetricValues verifies that specified metric exists and has nonzero values.
func checkMetricValues(keplerMetric *TestKeplerMetric, metricName string, zeroAllowed bool) {
	v, ok := keplerMetric.Metric[metricName]
	Expect(ok).To(BeTrue(), "Metric %s should exist", metricName)

	for _, val := range v {
		if val == 0 {
			if !zeroAllowed {
				Expect(val).To(BeNumerically(">", float64(0)), "Value for metric %s should be greater than 0", metricName)
			} else {
				Skip("Skipping test as values for " + metricName + " are zero")
			}
		}
	}
}

// noneZeroMetricValues verifies that specified metric exists and has nonzero values.
// It returns true if no zero value is found.
func noneZeroMetricValues(keplerMetric *TestKeplerMetric, metricName string) {
	checkMetricValues(keplerMetric, metricName, false)
}

// allowZeroMetricValues verifies that specified metric exists and has nonzero values.
// It returns true even if a zero value is found.
func allowZeroMetricValues(keplerMetric *TestKeplerMetric, metricName string) {
	checkMetricValues(keplerMetric, metricName, true)
}

// checkPodMetricValues iterates through the list of pods and checks for the presence and value of specified pod-level metric.
func checkPodMetricValues(keplerMetric *TestKeplerMetric, metricName string, zeroAllowed bool) {
	// retry 3 times to get the metrics, with exponential backoff
	retry := 0
	sum := 0.0
	for retry < 5 {
		for _, podName := range keplerMetric.PodLists {
			metricKey := metricName + " @ " + podName
			v, ok := keplerMetric.Metric[metricKey]
			Expect(ok).To(BeTrue(), "Metric %s should exists for pod %s", metricName, podName)
			for _, val := range v {
				sum += val
			}
		}
		if sum > 0 || zeroAllowed {
			break
		} else {
			retry += 1
			log.Infof("Retrying to get metrics for %s, retry count: %d", metricName, retry)
			// exponential backoff
			time.Sleep(time.Duration(math.Pow(2, float64(retry))) * time.Second)
		}
	}
	if sum == 0 {
		if !zeroAllowed {
			Expect(sum).To(BeNumerically(">", float64(0)), "Pod value for metric %s should be greater than 0", metricName)
		} else {
			Skip("Skipping test as values for " + metricName + " are zero")
		}
	}
}

// noneZeroPodMetricValues iterates through the list of pods and checks for the presence and value of specified pod-level metric.
// It returns true if no zero value is found.
func noneZeroPodMetricValues(keplerMetric *TestKeplerMetric, metricName string) {
	checkPodMetricValues(keplerMetric, metricName, false)
}

// allowZeroPodMetricValues iterates through the list of pods and checks for the presence and value of specified pod-level metric.
// It returns true even if a zero value is found.
func allowZeroPodMetricValues(keplerMetric *TestKeplerMetric, metricName string) {
	checkPodMetricValues(keplerMetric, metricName, true)
}

var _ = Describe("Metrics check should pass", Ordered, func() {
	_ = BeforeAll(func() {
		err := keplerMetric.GetMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	_ = DescribeTable("Check node level metrics for details, non zero value metric should be found",
		func(metricName string) {
			noneZeroMetricValues(keplerMetric, metricName)
		},
		Entry(nil, "kepler_exporter_build_info"),       // only one
		Entry(nil, "kepler_node_core_joules_total"),    // node level
		Entry(nil, "kepler_node_dram_joules_total"),    // node level
		Entry(nil, "kepler_node_info"),                 // node level
		Entry(nil, "kepler_node_package_joules_total"), // node level
	)

	_ = DescribeTable("Check node level metrics for details, zero value metric can be found",
		func(metricName string) {
			allowZeroMetricValues(keplerMetric, metricName)
		},
		Entry(nil, "kepler_node_platform_joules_total"), // node level
		Entry(nil, "kepler_node_uncore_joules_total"),   // node level
	)

	_ = DescribeTable("Check pod level metrics for details, no zero value metric should be found",
		func(metricName string) {
			noneZeroPodMetricValues(keplerMetric, metricName)
		},
		Entry(nil, "kepler_container_core_joules_total"),    // pod level
		Entry(nil, "kepler_container_dram_joules_total"),    // pod level
		Entry(nil, "kepler_container_joules_total"),         // pod level
		Entry(nil, "kepler_container_package_joules_total"), // pod level
	)

	_ = DescribeTable("Check pod level metrics for details, zero value metric can be found",
		func(metricName string) {
			allowZeroPodMetricValues(keplerMetric, metricName)
		},
		Entry(nil, "kepler_container_uncore_joules_total"), // pod level
	)
})
