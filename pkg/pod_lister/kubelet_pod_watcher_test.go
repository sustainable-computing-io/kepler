package pod_lister

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"context"
	"fmt"
)

var testPod = &v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-pod",
		Namespace: "default",
	},
	Spec:  v1.PodSpec{
		Containers: []v1.Container{
			v1.Container{
				Name: "test-container",
				Image: "busybox",
			},
		},
	},
}

func numContainers(podList *v1.PodList) int {
	totalContainer := 0
	for _, pod := range podList.Items {
		totalContainer += len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
	}
	return totalContainer
}
var _ = Describe("Test Pod Watcher", func() {
	It("Properly load init list and update with added/deleted pod", func() {
		podList, err := Keeper.GetPods()
		Expect(err).NotTo(HaveOccurred())
		initialPodNum := len(podList.Items)
		expectedSize := numContainers(podList)
		Expect(expectedSize).Should(BeNumerically(">", 0))
		
		maxWaitCount := 10
		count := 0
		for {
			time.Sleep(2 * time.Second)
			count += 1
			size := len(Keeper.PodCgroupIDCache)
			if size == expectedSize {
				break
			}
			fmt.Printf("%d != %d\n", expectedSize, size)
			Expect(count).Should(BeNumerically("<", maxWaitCount))
		}
		fmt.Printf("Initial cache size: %d\n", expectedSize)
		defer Keeper.Clientset.CoreV1().Pods(testPod.Namespace).Delete(context.Background(), testPod.Name, metav1.DeleteOptions{})
		_, err = Keeper.Clientset.CoreV1().Pods(testPod.Namespace).Create(context.Background(), testPod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		count = 0
		for {
			time.Sleep(2 * time.Second)
			count += 1
			getPod, err := Keeper.Clientset.CoreV1().Pods(testPod.Namespace).Get(context.Background(), testPod.Name, metav1.GetOptions{})
			if err == nil {
				if isAllContainerReady(*getPod) {
					break
				} else {
					fmt.Printf("%v \n", getPod.Status.ContainerStatuses)
				}
			}
			Expect(count).Should(BeNumerically("<", maxWaitCount))
		}
		time.Sleep(5 * time.Second)
		newSize := len(Keeper.PodCgroupIDCache)
		Expect(newSize).To(Equal(expectedSize+1))

		PodCacheTime = 1
		err = Keeper.Clientset.CoreV1().Pods(testPod.Namespace).Delete(context.Background(), testPod.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
		count = 0
		for {
			time.Sleep(2 * time.Second)
			count += 1
			podList, _ := Keeper.GetPods()
			size := len(podList.Items)
			if size == initialPodNum {
				break
			}
			fmt.Printf("%d != %d\n", initialPodNum, size)
			Expect(count).Should(BeNumerically("<", maxWaitCount))
		}
		time.Sleep(5 * time.Second)
		newSize = len(Keeper.PodCgroupIDCache)
		Expect(newSize).To(Equal(expectedSize))
	})

})
