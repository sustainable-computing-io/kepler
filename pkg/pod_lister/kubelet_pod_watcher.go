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

/*
This pod_watcher is to cache pod info on creation.
The cache will be deleted after the pod is deleted for a specific period of time. (scrape interval)
*/

package pod_lister

 import (
	 "context"
 
	 v1 "k8s.io/api/core/v1"
	 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	 "k8s.io/apimachinery/pkg/util/wait"
	 "k8s.io/client-go/informers"
	 "k8s.io/client-go/kubernetes"
	 "k8s.io/client-go/rest"
	 "k8s.io/client-go/tools/cache"
	 "k8s.io/client-go/tools/clientcmd"

	 "log"
	 "os"
	 "time"
	 "strings"
 )

 var Keeper *PodCacheKeeper
 var PodCacheTime = 30

 const (
	MAX_QSIZE = 100
 )

 func InitKeeper() error {
	var err error
	Keeper, err = NewPodCacheKeeper()
	if err != nil {
		log.Println("cannot init PodCacheKeeper.")
	}
	return err
}

 func getConfig() (*rest.Config, error) {
	var config *rest.Config
	var err error
	presentKube, ok := os.LookupEnv("KUBECONFIG_FILE")
	if !ok && presentKube != "" {
		log.Println("InCluster Config")
		config, err = rest.InClusterConfig()
	} else {
		log.Printf("Config %s", presentKube)
		config, err = clientcmd.BuildConfigFromFlags("", presentKube)
	}
	if err != nil {
		log.Printf("Config Error: %v", err)
	}
	return config, err
 }
 
 // PodCacheKeeper watches pods and cache pod info
 type PodCacheKeeper struct {
	 *kubernetes.Clientset
	 PodQueue chan *v1.Pod
	 Quit     chan struct{}
	 HostName string
	 PodCgroupIDCache map[string]*ContainerInfo
	 PodCache map[string]v1.Pod
 }
 
func isAllContainerReady(pod v1.Pod) bool {
	totalContainer := len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
	totalCreatedContainer := len(pod.Status.ContainerStatuses) + len(pod.Status.InitContainerStatuses)
	if totalContainer == totalCreatedContainer {
		statuses := pod.Status.InitContainerStatuses
		for _, status := range statuses {
			if status.ContainerID == "" {
				return false
			}
		}
		statuses = pod.Status.ContainerStatuses
		for _, status := range statuses {
			if status.ContainerID == "" {
				return false
			}
		}
		return true
	}
	return false
}

func getPodKey(pod *v1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}

 // NewPodCacheKeeper creates new cache keeper
 func NewPodCacheKeeper() (*PodCacheKeeper, error) {
	 config, err := getConfig()
	 if err != nil {
		return nil, err
	 }
	 clientset, err := kubernetes.NewForConfig(config)
	 if err != nil {
		return nil, err
	 }
	 hostName, err := os.Hostname()
	 if err != nil {
		return nil, err
	 }

	 quit := make(chan struct{})
	 podQueue := make(chan *v1.Pod, MAX_QSIZE)

	 keeper := &PodCacheKeeper{
		 Clientset: clientset,
		 PodQueue:  podQueue,
		 Quit:      quit,
		 HostName:  hostName,
		 PodCgroupIDCache: make(map[string]*ContainerInfo),
		 PodCache : make(map[string]v1.Pod),
	 }

	 factory := informers.NewSharedInformerFactory(clientset, 0)
	 podInformer := factory.Core().V1().Pods()
 
	 keeper.UpdateCurrentList()

	 podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		 UpdateFunc: func(prevObj, obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			prevPod, _ := prevObj.(*v1.Pod)
			 if !ok {
				 return
			 }
			 if pod.Spec.NodeName == hostName {
				if !isAllContainerReady(*prevPod) && isAllContainerReady(*pod) {
					keeper.PodQueue <- pod
				}
			 } 
		 },
		 DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				return
			}
			if pod.Spec.NodeName == hostName {
				keeper.PodQueue <- pod
			}
		},
	 })
	 factory.Start(keeper.Quit)
 
	 return keeper, nil
 }


// UpdateCurrentList puts existing pods to the process queue
func (w *PodCacheKeeper) UpdateCurrentList() error {
	initialList, err := w.GetPods()
	if err != nil {
		return err
	}
	for _, pod := range initialList.Items {
		w.PodQueue <- pod.DeepCopy()
	}
	return nil
}

 // getPods returns all daemon pod
 func (w *PodCacheKeeper) GetPods() (*v1.PodList, error) {
	 listOptions := metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + w.HostName,
	 }
	 return w.Clientset.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), listOptions)
 }

 // Run executes daemon watcher routine until get quit signal
 func (w *PodCacheKeeper) Run() {
	 defer close(w.PodQueue)
	 log.Println("start watching pod")
	 wait.Until(w.ProcessPodQueue, 0, w.Quit)
 }
 
 // ProcessPodQueue handle cache
 func (w *PodCacheKeeper) ProcessPodQueue() {
	 pod := <-w.PodQueue
	 if pod.GetDeletionTimestamp() == nil {
		// add case
		w.addPodInfoToCache(*pod)
		podKey := getPodKey(pod)
		w.PodCache[podKey] = *pod
	 } else {
		// delete case
		// remove pod info from cache after PodCacheTime seconds
		podKey := getPodKey(pod)
		cachedPod := w.PodCache[podKey]
		time.AfterFunc(time.Duration(PodCacheTime) * time.Second, func(){w.removePodInfoFromCache(cachedPod)})
		delete(w.PodCache, podKey)
	 }
 }

 func (w *PodCacheKeeper) addPodInfoToCache(pod v1.Pod) {
	statuses := pod.Status.ContainerStatuses
	for _, status := range statuses {
		info := &ContainerInfo{
			PodName:       pod.Name,
			Namespace:     pod.Namespace,
			ContainerName: status.Name,
		}
		containerID := strings.Trim(status.ContainerID, containerIDPredix)
		w.PodCgroupIDCache[containerID] = info
	}
	statuses = pod.Status.InitContainerStatuses
	for _, status := range statuses {
		info := &ContainerInfo{
			PodName:       pod.Name,
			Namespace:     pod.Namespace,
			ContainerName: status.Name,
		}
		containerID := strings.Trim(status.ContainerID, containerIDPredix)
		w.PodCgroupIDCache[containerID] = info
	}
	log.Printf("Add cache of pod %s\n", pod.Name)
 }

 func (w *PodCacheKeeper) removePodInfoFromCache(pod v1.Pod) {
	statuses := pod.Status.ContainerStatuses
	for _, status := range statuses {
		containerID := strings.Trim(status.ContainerID, containerIDPredix)
		delete(w.PodCgroupIDCache, containerID)
	}
	statuses = pod.Status.InitContainerStatuses
	for _, status := range statuses {
		containerID := strings.Trim(status.ContainerID, containerIDPredix)
		delete(w.PodCgroupIDCache, containerID)
	}
	log.Printf("Remove cache of pod %s\n", pod.Name)
 }

 func Destroy() {
	if Keeper != nil {
		close(Keeper.Quit)
	}	
}