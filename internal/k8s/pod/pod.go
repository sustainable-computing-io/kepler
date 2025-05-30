// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sustainable-computing-io/kepler/internal/service"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
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

	kubeEnabled    bool
	kubeConfigPath string
	nodeName       string

	cfg           *rest.Config
	manager       manager.Manager
	mgrCtx        context.Context
	mgrCancelFunc context.CancelFunc

	scheme *k8sruntime.Scheme

	getConfigFunc        func(kubeConfigPath string) (*rest.Config, error)
	newManagerFunc       func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error)
	createControllerFunc func(manager.Manager) error
}

type (
	Option struct {
		logger         *slog.Logger
		kubeEnabled    bool
		kubeConfigPath string
		nodeName       string
	}

	OptFn func(*Option)
)

// DefaultOpts() returns a new Opts with defaults set
func DefaultOpts() Option {
	return Option{
		logger: slog.Default(),
	}
}

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

func WithKubeConfig(path string) OptFn {
	return func(o *Option) {
		o.kubeConfigPath = path
	}
}

func WithNodeName(nodeName string) OptFn {
	return func(o *Option) {
		o.nodeName = nodeName
	}
}

func NewInformer(opts ...OptFn) *podInformer {
	opt := DefaultOpts()
	for _, fn := range opts {
		fn(&opt)
	}
	return &podInformer{
		logger:               opt.logger.With("service", "podInformer"),
		kubeEnabled:          opt.kubeEnabled,
		kubeConfigPath:       opt.kubeConfigPath,
		nodeName:             opt.nodeName,
		scheme:               k8sruntime.NewScheme(),
		getConfigFunc:        getConfig,
		newManagerFunc:       ctrl.NewManager,
		createControllerFunc: createController,
	}
}

func (pi *podInformer) Init() error {
	var err error
	err = corev1.AddToScheme(pi.scheme)
	if err != nil {
		return fmt.Errorf("controller-runtime could not add scheme: %w", err)
	}

	cfg, err := pi.getConfigFunc(pi.kubeConfigPath)
	if err != nil {
		return fmt.Errorf("cannot get kubeconfig: %w", err)
	}
	pi.cfg = cfg

	mgr, err := pi.setupManager()
	if err != nil {
		return fmt.Errorf("controller-runtime could not create manager: %w", err)
	}
	pi.manager = mgr

	err = pi.createControllerFunc(pi.manager)
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

	mgr, err := pi.newManagerFunc(pi.cfg, ctrl.Options{
		Scheme: pi.scheme,
		Cache:  cacheOp,
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
		pi.indexerFunc,   // keys
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add an index on containerID: %w", err)
	}
	return mgr, nil
}

func (pi *podInformer) indexerFunc(obj client.Object) []string {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return []string{"invalidContainerID"}
	}
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
}

func extractContainerID(str string) string {
	parts := strings.SplitN(str, "://", 2)
	return parts[len(parts)-1]
}

func (pi *podInformer) Run(ctx context.Context) error {
	pi.logger.Info("Starting pod informer")
	mgrCtx, cancel := context.WithCancel(context.Background())
	pi.mgrCtx = mgrCtx
	pi.mgrCancelFunc = cancel
	return pi.manager.Start(mgrCtx)
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

func getConfig(kubeConfigPath string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
}

func (i *podInformer) Name() string {
	return "podInformer"
}

func (pi *podInformer) Shutdown() error {
	pi.logger.Info("stopping pod informer manager")
	pi.mgrCancelFunc()
	return nil
}

type dummyReconciler struct{}

func (pi *dummyReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func createController(mgr manager.Manager) error {
	d := &dummyReconciler{}
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.ResourceVersionChangedPredicate{}).
		Build(d)
	return err
}
