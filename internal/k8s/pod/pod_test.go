// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Test_pod_informer_common(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		pi := NewInformer()
		assert.Equal(t, "podInformer", pi.Name())
	})
	t.Run("Reconcile", func(t *testing.T) {
		d := dummyReconciler{}
		res, err := d.Reconcile(context.Background(), reconcile.Request{})
		assert.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)
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
		kubeEnabled := true
		nodeName := "node1"
		logger := slog.Default().With("test", "custom")
		opts := []OptFn{
			WithLogger(logger),
			WithKubeConfig(kubeConfig),
			WithKubeEnabled(kubeEnabled),
			WithNodeName(nodeName),
		}
		pi := NewInformer(opts...)
		assert.NotNil(t, pi, "got nil pod informer")
		assert.NotNil(t, pi.logger, "pod informer has nil logger")
		assert.Equal(t, pi.kubeConfigPath, kubeConfig, "unexpected kubeconfig")
		assert.Equal(t, pi.kubeEnabled, kubeEnabled, "unexpected kube enabled")
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
	t.Run("success case", func(t *testing.T) {
		pi := NewInformer(WithNodeName("node1"))
		pi.getConfigFunc = mockGetConfig
		mockMgr := &mockManager{}
		pi.newManagerFunc = func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error) {
			return mockMgr, nil
		}
		pi.createControllerFunc = func(m manager.Manager) error {
			return nil
		}
		mockCache := &mockCache{}
		mockMgr.On("GetCache").Return(mockCache)
		mockCache.On("IndexField", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		err := pi.Init()
		assert.NoError(t, err)
	})
	t.Run("getConfig failed", func(t *testing.T) {
		pi := NewInformer()
		pi.getConfigFunc = func(kubeConfigPath string) (*rest.Config, error) {
			return nil, fmt.Errorf("!!you shall not pass!!")
		}
		err := pi.Init()
		assert.ErrorContains(t, err, "cannot get kubeconfig")
	})
	t.Run("newManager failed", func(t *testing.T) {
		pi := NewInformer()
		pi.getConfigFunc = mockGetConfig
		pi.newManagerFunc = func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error) {
			return nil, fmt.Errorf("!!you shall not pass!!")
		}
		err := pi.Init()
		assert.ErrorContains(t, err, "controller-runtime could not create manager")
	})
	t.Run("createController failed", func(t *testing.T) {
		pi := NewInformer()
		pi.getConfigFunc = mockGetConfig
		mockMgr := &mockManager{}
		mockCache := &mockCache{}
		mockMgr.On("GetCache").Return(mockCache)
		mockCache.On("IndexField", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		pi.newManagerFunc = func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error) {
			return mockMgr, nil
		}
		pi.createControllerFunc = func(m manager.Manager) error {
			return fmt.Errorf("!!you shall not pass!!")
		}
		err := pi.Init()
		assert.ErrorContains(t, err, "controller managed by manager could not be created")
	})
}

func TestPodInfo(t *testing.T) {
	t.Run("no pod found", func(t *testing.T) {
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
		).Return(nil)
		_, err := pi.PodInfo("container1")
		assert.ErrorIs(t, err, ErrNotFound, "unexpected error returned")
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
		retPod, err := pi.PodInfo("container1")
		assert.NoError(t, err)
		assert.Equal(t, string(pod1.UID), retPod.ID, "unexpected pod id")
		assert.Equal(t, pod1.Name, retPod.Name, "unexpected pod name")
		assert.Equal(t, pod1.Namespace, retPod.Namespace, "unexpected pod namespace")
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
		_, err := pi.PodInfo("container1")
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
		_, err := pi.PodInfo("container1")
		assert.ErrorContains(t, err, "error retrieving pod info from cache")
	})
}
