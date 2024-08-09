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
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	informerTimeout = time.Minute
	podResourceType = "pods"
	// Number of retries to process an event
	maxRetries = 5
	// Number of workers to process events
	// NOTE: Given that the ContainerStats map is protected under a shared mutex,
	// the number of workers should be kept at 1. Otherwise, we might starve
	// the collector.
	workers = 1
)

var (
	regexReplaceContainerIDPrefix = regexp.MustCompile(`.*//`)
	IsWatcherEnabled              = false
)

type ObjListWatcher struct {
	// Lock to synchronize the collector update with the watcher
	// NOTE: This lock is shared with the Collector
	Mx *sync.Mutex

	k8sCli              *kubernetes.Clientset
	ResourceKind        string
	informer            cache.SharedIndexInformer
	workqueue           workqueue.RateLimitingInterface
	stopChannel         chan struct{}
	bpfSupportedMetrics bpf.SupportedMetrics

	// NOTE: This map is shared with the Collector
	// ContainerStats holds all container energy and resource usage metrics
	ContainerStats map[string]*stats.ContainerStats
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

func NewObjListWatcher(bpfSupportedMetrics bpf.SupportedMetrics) *ObjListWatcher {
	w := &ObjListWatcher{
		stopChannel:         make(chan struct{}),
		k8sCli:              newK8sClient(),
		ResourceKind:        podResourceType,
		bpfSupportedMetrics: bpfSupportedMetrics,
		workqueue:           workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
	if w.k8sCli == nil || !config.EnableAPIServer {
		return w
	}
	optionsModifier := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.Set{"spec.nodeName": stats.GetNodeName()}.AsSelector().String() // to filter events per node
	}
	objListWatcher := cache.NewFilteredListWatchFromClient(
		w.k8sCli.CoreV1().RESTClient(),
		w.ResourceKind,
		metav1.NamespaceAll,
		optionsModifier,
	)
	w.informer = cache.NewSharedIndexInformer(objListWatcher, &corev1.Pod{}, 0, cache.Indexers{})
	w.stopChannel = make(chan struct{})
	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				w.workqueue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				w.workqueue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				w.workqueue.Add(key)
			}
		},
	})
	if err != nil {
		klog.Errorf("%v", err)
		return nil
	}
	IsWatcherEnabled = true
	return w
}

func (w *ObjListWatcher) processNextItem() bool {
	key, quit := w.workqueue.Get()
	if quit {
		return false
	}
	defer w.workqueue.Done(key)

	err := w.handleEvent(key.(string))
	w.handleErr(err, key)
	return true
}

func (w *ObjListWatcher) handleErr(err error, key interface{}) {
	// No error!
	if err == nil {
		w.workqueue.Forget(key)
		return
	}

	// Retry
	if w.workqueue.NumRequeues(key) < maxRetries {
		klog.Errorf("Error syncing pod %v: %v", key, err)
		w.workqueue.AddRateLimited(key)
		return
	}

	// Give up
	w.workqueue.Forget(key)
	klog.Infof("Dropping pod %q out of the queue: %v", key, err)
}

func (w *ObjListWatcher) handleEvent(key string) error {
	obj, exists, err := w.informer.GetIndexer().GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}
	if !exists {
		w.handleDeleted(obj)
	} else {
		if err := w.handleAdd(obj); err != nil {
			return err
		}
	}
	return nil
}

func (w *ObjListWatcher) Run() {
	if !IsWatcherEnabled {
		klog.Infoln("k8s APIserver watcher was not enabled")
		return
	}
	defer w.workqueue.ShutDown()

	go w.informer.Run(w.stopChannel)

	timeoutCh := make(chan struct{})
	timeoutTimer := time.AfterFunc(informerTimeout, func() {
		close(timeoutCh)
	})
	defer timeoutTimer.Stop()
	if !cache.WaitForCacheSync(timeoutCh, w.informer.HasSynced) {
		klog.Fatalf("watcher timed out waiting for caches to sync")
	}

	// launch workers to handle events
	for i := 0; i < workers; i++ {
		go wait.Until(w.runWorker, time.Second, w.stopChannel)
	}

	klog.Infoln("k8s APIserver watcher was started")
}

func (w *ObjListWatcher) runWorker() {
	for w.processNextItem() {
	}
}

func (w *ObjListWatcher) Stop() {
	klog.Infoln("k8s APIserver watcher was stopped")
	close(w.stopChannel)
}

func (w *ObjListWatcher) handleAdd(obj interface{}) error {
	switch w.ResourceKind {
	case podResourceType:
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			err := fmt.Errorf("could not convert obj: %v", w.ResourceKind)
			return err
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.ContainersReady {
				continue
			}
			klog.V(5).Infof("Pod %s %s is ready with %d container statuses, %d init container status, %d ephemeral statues",
				pod.Name, pod.Namespace, len(pod.Status.ContainerStatuses), len(pod.Status.InitContainerStatuses), len(pod.Status.EphemeralContainerStatuses))
			if err := w.fillInfo(pod, pod.Status.ContainerStatuses, w.Mx); err != nil {
				return err
			}
			if err := w.fillInfo(pod, pod.Status.InitContainerStatuses, w.Mx); err != nil {
				return err
			}
			if err := w.fillInfo(pod, pod.Status.EphemeralContainerStatuses, w.Mx); err != nil {
				return err
			}
			klog.V(5).Infof("parsing pod %s %s status: %v %v %v", pod.Name, pod.Namespace, pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses, pod.Status.EphemeralContainerStatuses)
			return nil
		}
	default:
		err := fmt.Errorf("watcher does not support object type %s", w.ResourceKind)
		return err
	}
	return errors.New("pod not ready")
}

func (w *ObjListWatcher) fillInfo(pod *corev1.Pod, containers []corev1.ContainerStatus, mx *sync.Mutex) error {
	var exist bool
	var err error
	mx.Lock()
	defer mx.Unlock()
	for j := 0; j < len(containers); j++ {
		containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
		if containerID == "" {
			err = fmt.Errorf("container %s did not start yet", containers[j].Name)
			continue
		}
		if _, exist = w.ContainerStats[containerID]; !exist {
			w.ContainerStats[containerID] = stats.NewContainerStats(containers[j].Name, pod.Name, pod.Namespace, containerID, w.bpfSupportedMetrics)
		}
		klog.V(5).Infof("receiving container %s %s %s %s", containers[j].Name, pod.Name, pod.Namespace, containerID)
		w.ContainerStats[containerID].ContainerName = containers[j].Name
		w.ContainerStats[containerID].PodName = pod.Name
		w.ContainerStats[containerID].Namespace = pod.Namespace
	}
	return err
}

func (w *ObjListWatcher) handleDeleted(obj interface{}) {
	switch w.ResourceKind {
	case podResourceType:
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			klog.Errorf("Could not convert obj: %v", w.ResourceKind)
			return
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
func (w *ObjListWatcher) deleteInfo(containers []corev1.ContainerStatus) {
	for j := 0; j < len(containers); j++ {
		containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
		delete(w.ContainerStats, containerID)
	}
}

func ParseContainerIDFromPodStatus(containerID string) string {
	return regexReplaceContainerIDPrefix.ReplaceAllString(containerID, "")
}
