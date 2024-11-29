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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	"github.com/sustainable-computing-io/kepler/pkg/node"
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
	if config.KubeConfig() == "" {
		// creates the in-cluster config
		restConf, err = rest.InClusterConfig()
		klog.Infoln("Using in cluster k8s config")
	} else {
		// use the current context in kubeconfig
		restConf, err = clientcmd.BuildConfigFromFlags("", config.KubeConfig())
		klog.Infoln("Using out cluster k8s config: ", config.KubeConfig())
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

func NewObjListWatcher(bpfSupportedMetrics bpf.SupportedMetrics) (*ObjListWatcher, error) {
	w := &ObjListWatcher{
		stopChannel:         make(chan struct{}),
		k8sCli:              newK8sClient(),
		ResourceKind:        podResourceType,
		bpfSupportedMetrics: bpfSupportedMetrics,
		workqueue:           workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
	if w.k8sCli == nil || !config.IsAPIServerEnabled() {
		return w, nil
	}
	optionsModifier := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.Set{"spec.nodeName": node.Name()}.AsSelector().String() // to filter events per node
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
				w.workqueue.AddRateLimited(key)
			}
			utilruntime.HandleError(err)
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				w.workqueue.AddRateLimited(key)
			}
			utilruntime.HandleError(err)
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				w.workqueue.AddRateLimited(key)
			}
			utilruntime.HandleError(err)
		},
	})

	if err != nil {
		klog.Errorf("%v", err)
		return nil, err
	}
	IsWatcherEnabled = true
	return w, nil
}

func (w *ObjListWatcher) processNextItem() bool {
	key, quit := w.workqueue.Get()
	if quit {
		klog.V(5).Info("quitting processNextItem")
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
		klog.V(5).Infof("Successfully synced '%s'", key)
		w.workqueue.Forget(key)
		return
	}

	// Put the item back on the workqueue to handle any transient errors
	// if it hasn't already been requeued more times than our maxRetries
	if w.workqueue.NumRequeues(key) < maxRetries {
		klog.V(5).Infof("failed to sync pod %v: %v ... requeuing, retries %v", key, err, w.workqueue.NumRequeues(key))
		w.workqueue.AddRateLimited(key)
		return
	}
	// Give up if we've exceeded MaxRetries, remove the item from the queue
	klog.V(5).Infof("Dropping pod %q out of the queue: %v", key, err)
	w.workqueue.Forget(key)

	// handle any errors that occurred
	utilruntime.HandleError(err)
}

func (w *ObjListWatcher) handleEvent(key string) error {
	obj, exists, err := w.informer.GetIndexer().GetByKey(key)
	if err != nil {
		klog.Errorf("fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		return w.handleDeleted(obj)
	}

	return w.handleAdd(obj)
}

func (w *ObjListWatcher) Run() error {
	if !IsWatcherEnabled {
		return fmt.Errorf("k8s APIserver watcher was not enabled")
	}
	defer utilruntime.HandleCrash()
	go w.informer.Run(w.stopChannel)

	timeoutCh := make(chan struct{})
	timeoutTimer := time.AfterFunc(informerTimeout, func() {
		close(timeoutCh)
	})
	defer timeoutTimer.Stop()

	klog.V(5).Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(timeoutCh, w.informer.HasSynced) {
		return fmt.Errorf("watcher timed out waiting for caches to sync")
	}

	// launch workers to handle events
	for i := 0; i < workers; i++ {
		go wait.Until(w.runWorker, time.Second, w.stopChannel)
	}

	klog.Infoln("k8s APIserver watcher was started")
	return nil
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
	var err error
	switch w.ResourceKind {
	case podResourceType:
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return fmt.Errorf("could not convert obj: %v", w.ResourceKind)
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.ContainersReady {
				// set the error in case we reach the end of the loop and no ContainersReady condition is found
				err = fmt.Errorf("containers not ready in pod: %v", pod.Name)
				continue
			}
			klog.V(5).Infof("Pod %s %s is ready with %d container statuses, %d init container status, %d ephemeral statues",
				pod.Name, pod.Namespace, len(pod.Status.ContainerStatuses), len(pod.Status.InitContainerStatuses), len(pod.Status.EphemeralContainerStatuses))
			w.Mx.Lock()
			err1 := w.fillInfo(pod, pod.Status.ContainerStatuses)
			err2 := w.fillInfo(pod, pod.Status.InitContainerStatuses)
			err3 := w.fillInfo(pod, pod.Status.EphemeralContainerStatuses)
			w.Mx.Unlock()
			if err1 != nil || err2 != nil || err3 != nil {
				err = fmt.Errorf("parsing pod %s %s ContainerStatuses issue : %v, InitContainerStatuses issue :%v, EphemeralContainerStatuses issue :%v", pod.Name, pod.Namespace, err1, err2, err3)
				return err
			}
			klog.V(5).Infof("parsing pod %s %s status: %v %v %v", pod.Name, pod.Namespace, pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses, pod.Status.EphemeralContainerStatuses)
			return nil
		}
	default:
		err = fmt.Errorf("watcher does not support object type %s", w.ResourceKind)
	}
	return err
}

func (w *ObjListWatcher) fillInfo(pod *corev1.Pod, containers []corev1.ContainerStatus) error {
	var err error
	var exist bool
	for j := 0; j < len(containers); j++ {
		containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
		// verify if container ID was already initialized
		if containerID == "" {
			// mark the error to requeue to the workqueue
			err = fmt.Errorf("container %s did not start yet status", containers[j].Name)
			continue
		}
		if _, exist = w.ContainerStats[containerID]; !exist {
			w.ContainerStats[containerID] = stats.NewContainerStats(containers[j].Name, pod.Name, pod.Namespace, containerID)
		}
		klog.V(5).Infof("receiving container %s %s %s %s", containers[j].Name, pod.Name, pod.Namespace, containerID)
		w.ContainerStats[containerID].ContainerName = containers[j].Name
		w.ContainerStats[containerID].PodName = pod.Name
		w.ContainerStats[containerID].Namespace = pod.Namespace
	}
	return err
}

func (w *ObjListWatcher) handleDeleted(obj interface{}) error {
	switch w.ResourceKind {
	case podResourceType:
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return fmt.Errorf("could not convert obj: %v", w.ResourceKind)
		}
		w.Mx.Lock()
		w.deleteInfo(pod.Status.ContainerStatuses)
		w.deleteInfo(pod.Status.InitContainerStatuses)
		w.deleteInfo(pod.Status.EphemeralContainerStatuses)
		w.Mx.Unlock()
		klog.V(5).Infof("deleting pod %s %s", pod.Name, pod.Namespace)
	default:
		return fmt.Errorf("watcher does not support object type %s", w.ResourceKind)
	}
	return nil
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

func (w *ObjListWatcher) ShutDownWithDrain() {
	done := make(chan struct{})

	// ShutDownWithDrain waits for all in-flight work to complete and thus could block indefinitely so put a deadline on it.
	go func() {
		w.workqueue.ShutDownWithDrain()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		klog.Warningf("timed out draining the queue on shut down")
	}
}
