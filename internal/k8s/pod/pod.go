// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sustainable-computing-io/kepler/internal/logger"
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
)

const (
	indexContainerID = "containerID"
)

type (
	Informer interface {
		service.Initializer
		service.Runner
		LookupByContainerID(containerID string) (*ContainerInfo, bool, error)
	}

	ContainerInfo struct {
		PodID         string
		PodName       string
		Namespace     string
		ContainerName string
	}

	podInformer struct {
		logger *slog.Logger

		kubeConfigPath string
		nodeName       string

		cfg     *rest.Config
		manager manager.Manager

		createRestConfigFunc func(kubeConfigPath string) (*rest.Config, error)
		newManagerFunc       func(config *rest.Config, options ctrl.Options) (ctrl.Manager, error)
	}

	Option struct {
		logger         *slog.Logger
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
		kubeConfigPath:       opt.kubeConfigPath,
		nodeName:             opt.nodeName,
		createRestConfigFunc: getConfig,
		newManagerFunc:       ctrl.NewManager,
	}
}

func (pi *podInformer) Init() error {
	var err error
	if pi.nodeName == "" {
		return fmt.Errorf("nodeName not set")
	}
	scheme := k8sruntime.NewScheme()
	err = corev1.AddToScheme(scheme)
	if err != nil {
		return fmt.Errorf("controller-runtime could not add scheme: %w", err)
	}

	cfg, err := pi.createRestConfigFunc(pi.kubeConfigPath)
	if err != nil {
		return fmt.Errorf("cannot get kubeconfig: %w", err)
	}
	pi.cfg = cfg

	mgr, err := pi.setupManager(scheme)
	if err != nil {
		return fmt.Errorf("controller-runtime could not create manager: %w", err)
	}
	pi.manager = mgr

	pi.setControllerRuntimeLogLevel()

	pi.logger.Info("pod informer initialized")

	return nil
}

func (pi *podInformer) setupManager(scheme *k8sruntime.Scheme) (ctrl.Manager, error) {
	cacheOp := cache.Options{}
	cacheOp.ByObject = map[client.Object]cache.ByObject{
		&corev1.Pod{}: {
			Field: fields.SelectorFromSet(fields.Set{
				"spec.nodeName": pi.nodeName,
			}),
		},
	}

	mgr, err := pi.newManagerFunc(pi.cfg, ctrl.Options{
		Scheme: scheme,
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
		indexContainerID, // index Name
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
		// this should not happen as cache uses type specific informers for indexing
		return nil
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
		"pod", pod.Name,
		"containers", strings.Join(containerIDs, ","),
	)
	return containerIDs
}

func extractContainerID(str string) string {
	parts := strings.SplitN(str, "://", 2)
	return parts[len(parts)-1]
}

func (pi *podInformer) Run(ctx context.Context) error {
	pi.logger.Info("Starting pod informer")
	return pi.manager.Start(ctx)
}

// LookupByContainerID retrieves pod details and container name given a containerID
func (pi *podInformer) LookupByContainerID(containerID string) (*ContainerInfo, bool, error) {
	var pods corev1.PodList

	err := pi.manager.GetCache().List(
		context.Background(),
		&pods,
		client.MatchingFields{indexContainerID: containerID},
	)
	if err != nil {
		return nil, false, fmt.Errorf("error retrieving pod info from cache: %w", err)
	}

	switch count := len(pods.Items); {
	case count == 0:
		return nil, false, nil
	case count > 1:
		return nil, false, fmt.Errorf("multiple pods found for containerID: %s", containerID)

	default: // case x == 1:
		pod := pods.Items[0]
		containerName := pi.findContainerName(&pod, containerID)
		pi.logger.Debug("pod found for container", "container", containerID, "pod", pod.Name, "containerName", containerName)

		return &ContainerInfo{
			PodID:         string(pod.UID),
			PodName:       pod.Name,
			Namespace:     pod.Namespace,
			ContainerName: containerName,
		}, true, nil
	}
}

func getConfig(kubeConfigPath string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
}

func (i *podInformer) Name() string {
	return "podInformer"
}

func (pi *podInformer) setControllerRuntimeLogLevel() {
	level := logger.LogLevel()
	opts := zap.Options{
		Level: slogLevelToZapLevel(level),
	}
	if level == slog.LevelDebug {
		opts.Development = true
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)).WithCallDepth(0))
}

func slogLevelToZapLevel(level slog.Level) zapcore.Level {
	switch {
	case level <= slog.LevelDebug:
		return zapcore.DebugLevel
	case level <= slog.LevelInfo:
		return zapcore.InfoLevel
	case level <= slog.LevelWarn:
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}

// findContainerName finds the container name for a given containerID in the pod
func (pi *podInformer) findContainerName(pod *corev1.Pod, containerID string) string {
	// Check regular containers
	for _, status := range pod.Status.ContainerStatuses {
		if status.ContainerID != "" && extractContainerID(status.ContainerID) == containerID {
			return status.Name
		}
	}
	// Check ephemeral containers
	for _, status := range pod.Status.EphemeralContainerStatuses {
		if status.ContainerID != "" && extractContainerID(status.ContainerID) == containerID {
			return status.Name
		}
	}
	// Check init containers
	for _, status := range pod.Status.InitContainerStatuses {
		if status.ContainerID != "" && extractContainerID(status.ContainerID) == containerID {
			return status.Name
		}
	}
	return ""
}
