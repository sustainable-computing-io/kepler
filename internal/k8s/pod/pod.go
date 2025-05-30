// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sustainable-computing-io/kepler/internal/service"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	scheme = k8sruntime.NewScheme()

	ErrNotFound = errors.New("no pod found for container")
)

const (
	KubeconfigFlagName = "kubeconfig"
	IndexContainerID   = "containerID"
)

type (
	Informer interface {
		service.Service
		service.Initializer
		service.Runner
		service.Shutdowner
		PodInfo(containerID string) (*PodInfo, error)
	}

	PodInfo struct {
		ID        string
		Name      string
		Namespace string
	}
)

type podInformer struct {
	logger *slog.Logger

	kubeEnabled bool
	kubeconfig  string
	nodeName    string

	manager manager.Manager
	cli     client.Client

	scheme *k8sruntime.Scheme
}

type (
	Option struct {
		logger      *slog.Logger
		kubeEnabled bool
		kubeconfig  string
		nodeName    string
	}

	OptFn func(*Option)
)

func WithLogger(logger *slog.Logger) OptFn {
	return func(o *Option) {
		o.logger = logger
	}
}

func WithKubeEnabled(enabled bool) OptFn {
	return func(o *Option) {
		o.kubeEnabled = enabled
	}
}

func WithKubeConfig(config string) OptFn {
	return func(o *Option) {
		o.kubeconfig = config
	}
}

func WithNodeName(nodeName string) OptFn {
	return func(o *Option) {
		o.nodeName = nodeName
	}
}

func NewInformer(opts ...OptFn) Informer {
	opt := &Option{}
	for _, fn := range opts {
		fn(opt)
	}
	return &podInformer{
		logger:      opt.logger.With("service", "podInformer"),
		kubeEnabled: opt.kubeEnabled,
		kubeconfig:  opt.kubeconfig,
		nodeName:    opt.nodeName,
		scheme:      k8sruntime.NewScheme(),
	}
}

func (pi *podInformer) Init() error {
	var err error
	err = corev1.AddToScheme(pi.scheme)
	if err != nil {
		return fmt.Errorf("controller-runtime could not add scheme: %w", err)
	}

	err = initControllerRuntime(pi.kubeconfig)
	if err != nil {
		return fmt.Errorf("controller-runtime could not be initialized with kubeconfig %w", err)
	}

	mgr, err := pi.setupManager()
	if err != nil {
		return fmt.Errorf("controller-runtime could not create manager: %w", err)
	}
	pi.manager = mgr

	_, err = ctrl.NewControllerManagedBy(pi.manager).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.ResourceVersionChangedPredicate{}).
		Build(pi)
	if err != nil {
		return fmt.Errorf("controller managed by manager could not be created: %w", err)
	}

	opts := zap.Options{
		Development: true, // enables DebugLevel by default
		Level:       zapcore.DebugLevel,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)).WithCallDepth(0))

	pi.logger.Info("pod informer initialized")

	return nil
}

func (pi *podInformer) setupManager() (ctrl.Manager, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get config from controller-runtime: %w", err)
	}

	cacheOp := cache.Options{}
	if pi.nodeName != "" {
		cacheOp.ByObject = map[client.Object]cache.ByObject{
			&corev1.Pod{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"spec.nodeName": pi.nodeName,
				}),
			},
		}
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: pi.scheme,
		Cache:  cacheOp,
		// TODO: fix this
		Metrics: server.Options{
			BindAddress: "0.0.0.0:8081",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("controller-runtime could not create manager: %w", err)
	}
	pi.logger.Debug("setupManager with cache")
	// Add an index on containerID
	err = mgr.GetCache().IndexField(
		context.Background(),
		&corev1.Pod{},    // cache object
		IndexContainerID, // index Name
		func(obj client.Object) []string { // keys
			pod := obj.(*corev1.Pod)
			var containerIDs []string
			// TODO: check for status.State.Running != nil ?
			for _, status := range pod.Status.ContainerStatuses {
				if status.ContainerID != "" {
					containerIDs = append(containerIDs, extractContainerID(status.ContainerID))
				}
			}
			for _, status := range pod.Status.EphemeralContainerStatuses {
				if status.ContainerID != "" {
					containerIDs = append(containerIDs, extractContainerID(status.ContainerID))
				}
			}
			for _, status := range pod.Status.InitContainerStatuses {
				if status.ContainerID != "" {
					containerIDs = append(containerIDs, extractContainerID(status.ContainerID))
				}
			}
			pi.logger.Debug(
				"containers for pod",
				"pod",
				pod.Name,
				"containers",
				strings.Join(containerIDs, ","),
			)
			return containerIDs
		})
	if err != nil {
		return nil, fmt.Errorf("failed to add an index on containerID: %w", err)
	}
	return mgr, nil
}

func extractContainerID(str string) string {
	parts := strings.SplitN(str, "://", 2)
	return parts[len(parts)-1]
}

func (pi *podInformer) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (pi *podInformer) Run(ctx context.Context) error {
	pi.logger.Info("Starting pod informer")
	return pi.manager.Start(ctx)
}

// PodInfo retrieves pod details given a containerID
func (pi *podInformer) PodInfo(containerID string) (*PodInfo, error) {
	var pods corev1.PodList

	err := pi.manager.GetCache().List(
		context.Background(),
		&pods,
		client.MatchingFields{IndexContainerID: containerID},
	)
	if err != nil {
		return nil, fmt.Errorf("error retrieving pod info from cache: %w", err)
	}

	if len(pods.Items) == 0 {
		// pi.logger.Debug("no pod found for container", "container", containerID)
		return nil, ErrNotFound
	}
	if len(pods.Items) > 1 {
		return nil, fmt.Errorf("multiple pods found for containerID: %s", containerID)
	}

	pod := pods.Items[0]
	pi.logger.Debug("pod found for container", "container", containerID, "pod", pod.Name)
	return &PodInfo{
		ID:        string(pod.UID),
		Name:      pod.Name,
		Namespace: pod.Namespace,
	}, nil
}

func initControllerRuntime(kubeconfig string) error {
	ctrlConfigFlagSet := flag.NewFlagSet("ctrlFlags", flag.ContinueOnError)
	ctrl.RegisterFlags(ctrlConfigFlagSet)
	ctrlConfigFlagSet.Set(KubeconfigFlagName, kubeconfig)
	return nil
}

func (i *podInformer) Name() string {
	return "podInformer"
}

func (i *podInformer) Shutdown() error {
	return nil
}
