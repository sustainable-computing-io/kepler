package pod

import (
	"log/slog"

	"github.com/sustainable-computing-io/kepler/internal/service"
)

type (
	Informer interface {
		service.Service
		PodInfo(containerID string) (*PodInfo, error)
	}

	PodInfo struct {
		ID        string
		Name      string
		Namespace string
		// Node  string ??
	}
)

type informer struct {
	service.Service
}

func (i *informer) PodInfo(containerID string) (*PodInfo, error) {
	return nil, nil
}

type (
	Option struct{}

	OptFn func(*informer)
)

func WithLogger(logger *slog.Logger) OptFn {
	return func(i *informer) {
	}
}

func WithKubeConfig(kubeConfig string) OptFn {
	return func(i *informer) {
	}
}

func NewInformer(opts ...OptFn) Informer {
	return &informer{}
}
