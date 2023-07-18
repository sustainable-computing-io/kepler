/*
Copyright 2023.

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

package kubernetes

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	informerTimeout = time.Minute
	podResourceType = "pods"
)

var (
	regexReplaceContainerIDPrefix = regexp.MustCompile(`.*//`)
	IsWatcherEnabled              = false
)

type ObjListWatcher struct {
	// Lock to syncronize the collector update with the watcher
	Mx *sync.Mutex

	k8sCli       *kubernetes.Clientset
	ResourceKind string
	informer     cache.SharedInformer
	stopChannel  chan struct{}

	// ContainersMetrics holds all container energy and resource usage metrics
	ContainersMetrics *map[string]*collector_metric.ContainerMetrics
}

func newK8sClient() *kubernetes.Clientset {
	var restConf *rest.Config
	var err error
	if config.KubeConfig == "" {
		// creates the in-cluster config
		restConf, err = rest.InClusterConfig()
		klog.Infoln("Using in cluster k8s config")
	} else {
		// use the current context in kubeconfig
		restConf, err = clientcmd.BuildConfigFromFlags("", config.KubeConfig)
		klog.Infoln("Using out cluster k8s config: ", config.KubeConfig)
	}
	if err != nil {
		klog.Infof("failed to get config: %v", err)
		return nil
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(restConf)
	if err != nil {
		klog.Fatalf("%v", err)
	}
	return clientset
}

func NewObjListWatcher() *ObjListWatcher {
	w := &ObjListWatcher{
		stopChannel:  make(chan struct{}),
		k8sCli:       newK8sClient(),
		ResourceKind: podResourceType,
	}
	if w.k8sCli == nil || !config.EnableAPIServer {
		return w
	}

	optionsModifier := func(options *metav1.ListOptions) {
		options.FieldSelector = fmt.Sprintf("spec.nodeName=%s", collector_metric.NodeName) // to filter events per node
	}
	objListWatcher := cache.NewFilteredListWatchFromClient(
		w.k8sCli.CoreV1().RESTClient(),
		w.ResourceKind,
		metav1.NamespaceAll,
		optionsModifier,
	)

	w.informer = cache.NewSharedInformer(objListWatcher, nil, 0)
	w.stopChannel = make(chan struct{})
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			w.handleUpdate(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			w.handleDeleted(obj)
		},
	})
	IsWatcherEnabled = true
	return w
}

func (w *ObjListWatcher) Run() {
	if !IsWatcherEnabled {
		return
	}
	go w.informer.Run(w.stopChannel)
	timeoutCh := make(chan struct{})
	timeoutTimer := time.AfterFunc(informerTimeout, func() {
		close(timeoutCh)
	})
	defer timeoutTimer.Stop()
	if !cache.WaitForCacheSync(timeoutCh, w.informer.HasSynced) {
		klog.Fatalf("watcher timed out waiting for caches to sync")
	}
}

func (w *ObjListWatcher) Stop() {
	close(w.stopChannel)
}

func (w *ObjListWatcher) handleUpdate(obj interface{}) {
	switch w.ResourceKind {
	case podResourceType:
		pod, ok := obj.(*k8sv1.Pod)
		if !ok {
			klog.Infof("Could not convert obj: %v", w.ResourceKind)
			return
		}
		for _, condition := range pod.Status.Conditions {
			klog.V(5).Infof("Pod %s %s status %s %s", pod.Name, pod.Namespace, condition.Type, condition.Status)
			if condition.Type == k8sv1.ContainersReady {
				klog.V(5).Infof("Pod %s %s is ready with %d container statuses, %d init container status, %d ephemeral statues",
					pod.Name, pod.Namespace, len(pod.Status.ContainerStatuses), len(pod.Status.InitContainerStatuses), len(pod.Status.EphemeralContainerStatuses))
				w.Mx.Lock()
				err1 := w.fillInfo(pod, pod.Status.ContainerStatuses)
				err2 := w.fillInfo(pod, pod.Status.InitContainerStatuses)
				err3 := w.fillInfo(pod, pod.Status.EphemeralContainerStatuses)
				w.Mx.Unlock()
				klog.V(5).Infof("parsing pod %s %s status: %v %v %v", pod.Name, pod.Namespace, err1, err2, err3)
			}
		}

	default:
		klog.Infof("Watcher does not support object type %s", w.ResourceKind)
		return
	}
}

func (w *ObjListWatcher) fillInfo(pod *k8sv1.Pod, containers []k8sv1.ContainerStatus) error {
	var err error
	var exist bool
	for j := 0; j < len(containers); j++ {
		containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
		// verify if container ID was already initialized
		if containerID == "" {
			err = fmt.Errorf("container %s did not start yet", containers[j].Name)
			continue
		}
		if _, exist = (*w.ContainersMetrics)[containerID]; !exist {
			(*w.ContainersMetrics)[containerID] = collector_metric.NewContainerMetrics(containers[j].Name, pod.Name, pod.Namespace, containerID)
		}
		klog.V(5).Infof("receiving container %s %s %s %s", containers[j].Name, pod.Name, pod.Namespace, containerID)
		(*w.ContainersMetrics)[containerID].ContainerName = containers[j].Name
		(*w.ContainersMetrics)[containerID].PodName = pod.Name
		(*w.ContainersMetrics)[containerID].Namespace = pod.Namespace
	}
	return err
}

func (w *ObjListWatcher) handleDeleted(obj interface{}) {
	switch w.ResourceKind {
	case podResourceType:
		pod, ok := obj.(*k8sv1.Pod)
		if !ok {
			klog.Fatalf("Could not convert obj: %v", w.ResourceKind)
		}
		w.Mx.Lock()
		w.deleteInfo(pod.Status.ContainerStatuses)
		w.deleteInfo(pod.Status.InitContainerStatuses)
		w.deleteInfo(pod.Status.EphemeralContainerStatuses)
		w.Mx.Unlock()
	default:
		klog.Infof("Watcher does not support object type %s", w.ResourceKind)
		return
	}
}

// TODO: instead of delete, it might be better to mark it to delete since k8s takes time to really delete an object
func (w *ObjListWatcher) deleteInfo(containers []k8sv1.ContainerStatus) {
	for j := 0; j < len(containers); j++ {
		containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
		delete(*w.ContainersMetrics, containerID)
	}
}

func ParseContainerIDFromPodStatus(containerID string) string {
	return regexReplaceContainerIDPrefix.ReplaceAllString(containerID, "")
}
