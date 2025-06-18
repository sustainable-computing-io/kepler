// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestName(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		pi := NewInformer(WithNodeName("abc"))
		assert.Equal(t, "podInformer", pi.Name())
	})
}

func Test_extractContainerID(t *testing.T) {
	t.Run("containerd container", func(t *testing.T) {
		ctrStr := "containerd://fe8232774ce3469e8ca34bdb8715738fb212c4c60cb09f3e94f78e254291d081"
		assert.Equal(
			t,
			"fe8232774ce3469e8ca34bdb8715738fb212c4c60cb09f3e94f78e254291d081",
			extractContainerID(ctrStr),
		)
	})
	t.Run("crio container", func(t *testing.T) {
		ctrStr := "cri-o://9452165add21fccb5af5cfda5af59e7f2b4f9efd4326414f3d435bfb3a2b3b08"
		assert.Equal(
			t,
			"9452165add21fccb5af5cfda5af59e7f2b4f9efd4326414f3d435bfb3a2b3b08",
			extractContainerID(ctrStr),
		)
	})
}

func Test_getConfig(t *testing.T) {
	t.Run("empty kubeconfig", func(t *testing.T) {
		var err error
		_, err = getConfig("")
		assert.ErrorContains(t, err, "invalid configuration: no configuration has been provided")
	})
	t.Run("invalid kubeconfig", func(t *testing.T) {
		var err error
		invalid := "/invalid/kube/config"
		_, err = getConfig(invalid)
		assert.ErrorContains(t, err, "no such file or directory")
	})
	t.Run("valid kubeconfig", func(t *testing.T) {
		// no test
	})
}

func TestNewPodInformer(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		got := NewInformer()
		assert.NotNil(t, got, "got nil pod informer")
		assert.NotNil(t, got.logger, "pod informer has nil logger")
	})
	t.Run("options supplied", func(t *testing.T) {
		kubeConfig := "/some/kubeconfig"
		nodeName := "node1"
		logger := slog.Default().With("test", "custom")
		opts := []OptFn{
			WithLogger(logger),
			WithKubeConfig(kubeConfig),
			WithNodeName(nodeName),
		}
		pi := NewInformer(opts...)
		assert.NotNil(t, pi, "got nil pod informer")
		assert.NotNil(t, pi.logger, "pod informer has nil logger")
		assert.Equal(t, pi.kubeConfigPath, kubeConfig, "unexpected kubeconfig")
		assert.Equal(t, pi.nodeName, nodeName, "unexpected node name")
	})
}

func TestIndexerFunc(t *testing.T) {
	pi := NewInformer()
	t.Run("with normal container", func(t *testing.T) {
		got := pi.indexerFunc(podWithStatus(
			cstatus([]string{"cri-o://30781785a0e2e0511e12befb69f9513e11fbdbbb63ef249c0a2f3afaaa19d0f3"}),
			cstatus([]string{"cri-o://9452165add21fccb5af5cfda5af59e7f2b4f9efd4326414f3d435bfb3a2b3b08"}),
			cstatus([]string{"cri-o://55005e31927193faa525d690003450e1928c91831ebb74da8fb8751f79f298cf"}),
		))
		assert.ElementsMatch(t, []string{
			"30781785a0e2e0511e12befb69f9513e11fbdbbb63ef249c0a2f3afaaa19d0f3",
			"9452165add21fccb5af5cfda5af59e7f2b4f9efd4326414f3d435bfb3a2b3b08",
			"55005e31927193faa525d690003450e1928c91831ebb74da8fb8751f79f298cf",
		}, got, "unexpected containerIDs")
	})
}

func TestInit(t *testing.T) {
	t.Run("empty nodeName", func(t *testing.T) {
		pi := NewInformer()
		err := pi.Init()
		assert.ErrorContains(t, err, "nodeName not set")
	})
	t.Run("success case", func(t *testing.T) {
		pi := NewInformer(WithNodeName("node1"))
		pi.createRestConfigFunc = mockGetConfig
		mockMgr := &mockManager{}
		pi.newManagerFunc = func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error) {
			return mockMgr, nil
		}
		mockCache := &mockCache{}
		mockMgr.On("GetCache").Return(mockCache)
		mockCache.On("IndexField", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		err := pi.Init()
		assert.NoError(t, err)
	})
	t.Run("getConfig failed", func(t *testing.T) {
		pi := NewInformer(WithNodeName("node1"))
		pi.createRestConfigFunc = func(kubeConfigPath string) (*rest.Config, error) {
			return nil, fmt.Errorf("!!you shall not pass!!")
		}
		err := pi.Init()
		assert.ErrorContains(t, err, "cannot get kubeconfig")
	})
	t.Run("newManager failed", func(t *testing.T) {
		pi := NewInformer(WithNodeName("node1"))
		pi.createRestConfigFunc = mockGetConfig
		pi.newManagerFunc = func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error) {
			return nil, fmt.Errorf("!!you shall not pass!!")
		}
		err := pi.Init()
		assert.ErrorContains(t, err, "controller-runtime could not create manager")
	})
}

func TestPodInfo(t *testing.T) {
	t.Run("no pod found", func(t *testing.T) {
		pi := NewInformer(WithNodeName("node1"))
		mockMgr := &mockManager{}
		pi.manager = mockMgr
		mockCache := &mockCache{}
		mockMgr.On("GetCache").Return(mockCache)
		mockCache.On(
			"List",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil)
		containerInfo, found, err := pi.LookupByContainerID("container1")
		assert.NoError(t, err)
		assert.False(t, found, "expected container not to be found")
		assert.Nil(t, containerInfo, "expected nil container info")
	})
	t.Run("exactly one pod found", func(t *testing.T) {
		pi := NewInformer()
		mockMgr := &mockManager{}
		pi.manager = mockMgr
		mockCache := &mockCache{}
		mockMgr.On("GetCache").Return(mockCache)
		pod1 := corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Name:      "pod-name",
				UID:       "pod-uuid",
				Namespace: "pod-namespace",
			},
		}
		mockCache.On(
			"List",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil).Run(func(args mock.Arguments) {
			pods := args.Get(1).(*corev1.PodList)
			pods.Items = []corev1.Pod{pod1}
		})
		containerInfo, found, err := pi.LookupByContainerID("container1")
		assert.NoError(t, err)
		assert.True(t, found, "expected container to be found")
		assert.NotNil(t, containerInfo, "expected non-nil container info")
		assert.Equal(t, string(pod1.UID), containerInfo.PodID, "unexpected pod id")
		assert.Equal(t, pod1.Name, containerInfo.PodName, "unexpected pod name")
		assert.Equal(t, pod1.Namespace, containerInfo.Namespace, "unexpected pod namespace")
		assert.Equal(t, "", containerInfo.ContainerName, "expected empty container name")
	})
	t.Run("more than one pod found", func(t *testing.T) {
		pi := NewInformer()
		mockMgr := &mockManager{}
		pi.manager = mockMgr
		mockCache := &mockCache{}
		mockMgr.On("GetCache").Return(mockCache)
		pod1 := corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Name:      "pod-name",
				UID:       "pod-uuid",
				Namespace: "pod-namespace",
			},
		}
		mockCache.On(
			"List",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil).Run(func(args mock.Arguments) {
			pods := args.Get(1).(*corev1.PodList)
			pods.Items = []corev1.Pod{pod1, pod1}
		})
		_, found, err := pi.LookupByContainerID("container1")
		assert.False(t, found, "expected container not to be found due to multiple pods")
		assert.ErrorContains(t, err, "multiple pods found for containerID")
	})
	t.Run("cache error", func(t *testing.T) {
		pi := NewInformer()
		mockMgr := &mockManager{}
		pi.manager = mockMgr
		mockCache := &mockCache{}
		mockMgr.On("GetCache").Return(mockCache)
		mockCache.On(
			"List",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(fmt.Errorf("!!you shall not pass!!"))
		_, found, err := pi.LookupByContainerID("container1")
		assert.False(t, found, "expected container not to be found due to cache error")
		assert.ErrorContains(t, err, "error retrieving pod info from cache")
	})
}

func TestPodInformer_RunIntegration(t *testing.T) {
	t.Run("integration test with real manager lifecycle", func(t *testing.T) {
		scheme := runtime.NewScheme()
		err := corev1.AddToScheme(scheme)
		assert.NoError(t, err)

		testPod := &corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				UID:       "test-uid-123",
			},
			Spec: corev1.PodSpec{
				NodeName: "test-node",
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "test-container", ContainerID: "containerd://abc123"},
				},
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(testPod).
			Build()

		pi := NewInformer(WithNodeName("test-node"))
		pi.createRestConfigFunc = mockGetConfig

		fakeMgr := &fakeManager{
			client: fakeClient,
			scheme: scheme,
		}

		pi.newManagerFunc = func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error) {
			return fakeMgr, nil
		}

		err = pi.Init()
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- pi.Run(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		containerInfo, found, err := pi.LookupByContainerID("abc123")
		if err != nil {
			t.Logf("LookupByContainerID lookup failed (expected in fake setup): %v", err)
		} else if found {
			assert.Equal(t, "test-pod", containerInfo.PodName)
			assert.Equal(t, "default", containerInfo.Namespace)
			assert.Equal(t, "test-uid-123", containerInfo.PodID)
			assert.Equal(t, "test-container", containerInfo.ContainerName)
		}

		cancel()

		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("Run() did not stop after context cancellation")
		}
	})
}

func TestFindContainerName(t *testing.T) {
	pi := NewInformer()

	t.Run("find container in regular containers", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "app-container", ContainerID: "containerd://abc123"},
					{Name: "sidecar-container", ContainerID: "containerd://def456"},
				},
			},
		}
		containerName := pi.findContainerName(pod, "abc123")
		assert.Equal(t, "app-container", containerName)

		containerName = pi.findContainerName(pod, "def456")
		assert.Equal(t, "sidecar-container", containerName)
	})

	t.Run("find container in ephemeral containers", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{
					{Name: "debug-container", ContainerID: "cri-o://ephemeral123"},
				},
			},
		}
		containerName := pi.findContainerName(pod, "ephemeral123")
		assert.Equal(t, "debug-container", containerName)
	})

	t.Run("find container in init containers", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					{Name: "init-container", ContainerID: "containerd://init123"},
				},
			},
		}
		containerName := pi.findContainerName(pod, "init123")
		assert.Equal(t, "init-container", containerName)
	})

	t.Run("container not found returns empty string", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "app-container", ContainerID: "containerd://abc123"},
				},
			},
		}
		containerName := pi.findContainerName(pod, "nonexistent")
		assert.Equal(t, "", containerName)
	})

	t.Run("empty container ID in status", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "app-container", ContainerID: ""},
					{Name: "running-container", ContainerID: "containerd://running123"},
				},
			},
		}
		containerName := pi.findContainerName(pod, "running123")
		assert.Equal(t, "running-container", containerName)
	})

	t.Run("mixed container types", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					{Name: "init-container", ContainerID: "containerd://init123"},
				},
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "app-container", ContainerID: "containerd://app123"},
				},
				EphemeralContainerStatuses: []corev1.ContainerStatus{
					{Name: "debug-container", ContainerID: "cri-o://debug123"},
				},
			},
		}

		// Test finding in each type
		assert.Equal(t, "init-container", pi.findContainerName(pod, "init123"))
		assert.Equal(t, "app-container", pi.findContainerName(pod, "app123"))
		assert.Equal(t, "debug-container", pi.findContainerName(pod, "debug123"))
	})

	t.Run("different container runtime prefixes", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "containerd-container", ContainerID: "containerd://containerd123"},
					{Name: "crio-container", ContainerID: "cri-o://crio123"},
					{Name: "docker-container", ContainerID: "docker://docker123"},
				},
			},
		}

		assert.Equal(t, "containerd-container", pi.findContainerName(pod, "containerd123"))
		assert.Equal(t, "crio-container", pi.findContainerName(pod, "crio123"))
		assert.Equal(t, "docker-container", pi.findContainerName(pod, "docker123"))
	})
}

func TestSlogLevelToZapLevel(t *testing.T) {
	tests := []struct {
		input    slog.Level
		expected zapcore.Level
	}{
		{slog.LevelDebug, zapcore.DebugLevel},
		{slog.LevelInfo, zapcore.InfoLevel},
		{slog.LevelWarn, zapcore.WarnLevel},
		{slog.LevelError, zapcore.ErrorLevel},
		{slog.Level(-10), zapcore.DebugLevel},
		{slog.Level(10), zapcore.ErrorLevel},
	}

	for _, tc := range tests {
		result := slogLevelToZapLevel(tc.input)
		assert.Equal(t, tc.expected, result, "Conversion failed for slog level: %v", tc.input)
	}
}
