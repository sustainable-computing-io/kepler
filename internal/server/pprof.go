// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/http/pprof"

	"github.com/sustainable-computing-io/kepler/internal/service"
)

type pp struct {
	api APIService
}

var (
	_ service.Service     = (*pp)(nil)
	_ service.Initializer = (*pp)(nil)
)

func NewPprof(api APIService) *pp {
	return &pp{
		api: api,
	}
}

func (p *pp) Name() string {
	return "pprof"
}

func (p *pp) Init() error {
	return p.api.Register("/debug/pprof/", "pprof", "Profiling Data", handlers())
}

func handlers() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return mux
}
