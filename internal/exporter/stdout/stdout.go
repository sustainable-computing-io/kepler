// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package stdout

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

type (
	Initializer = service.Initializer
	Runner      = service.Runner
	Shutdowner  = service.Shutdowner
	Monitor     = monitor.Service
)

// Exporter exports power data to stdout
type Exporter struct {
	logger   *slog.Logger
	monitor  Monitor
	out      io.WriteCloser
	ticker   time.Ticker
	interval time.Duration
}

var (
	_ Initializer = (*Exporter)(nil)
	_ Runner      = (*Exporter)(nil)
	_ Shutdowner  = (*Exporter)(nil)
)

type Opts struct {
	logger   *slog.Logger
	out      io.WriteCloser
	interval time.Duration
}

// DefaultOpts() returns a new Opts with defaults set
func DefaultOpts() Opts {
	return Opts{
		logger:   slog.Default().With("service", "stdout"),
		out:      os.Stdout,
		interval: 2 * time.Second,
	}
}

// OptionFn is a function sets one more more options in Opts struct
type OptionFn func(*Opts)

// WithLogger sets the logger for the PowerMonitor
func WithLogger(logger *slog.Logger) OptionFn {
	return func(o *Opts) {
		o.logger = logger
	}
}

func WithOutput(out io.WriteCloser) OptionFn {
	return func(o *Opts) {
		o.out = out
	}
}

func WithInterval(interval time.Duration) OptionFn {
	return func(o *Opts) {
		o.interval = interval
	}
}

func NewExporter(pm Monitor, applyOpts ...OptionFn) *Exporter {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	exporter := &Exporter{
		logger:   opts.logger.With("service", "stdout"),
		monitor:  pm,
		out:      opts.out,
		interval: opts.interval,
	}

	return exporter
}

func (e *Exporter) Init() error {
	e.ticker = *time.NewTicker(e.interval)
	return nil
}

func (e *Exporter) Run(ctx context.Context) error {
	for {
		select {
		case now := <-e.ticker.C:
			snapshot, err := e.monitor.Snapshot()
			if err != nil {
				e.logger.Error("Failed to collect power data", "error", err)
				return nil
			}
			write(e.out, now, snapshot)
		case <-ctx.Done():
			e.logger.Info("Exiting ticker")
			return nil
		}
	}
}

func write(out io.Writer, now time.Time, snapshot *monitor.Snapshot) {
	writeNode(out, snapshot.Node)
}

func writeNode(out io.Writer, node *monitor.Node) {
	rows := [][]string{}
	// copying to a slice, to sort based on zone name
	for zone, usage := range node.Zones {
		rows = append(rows, []string{
			zone.Name(),
			usage.Power.String(),
			usage.EnergyTotal.String(),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})
	table := tablewriter.NewWriter(out)
	table.Configure(func(cfg *tablewriter.Config) {
		cfg.Row.Formatting.Alignment = tw.AlignRight
	})
	table.Header([]string{"Zone", "Power(W)", "Absolute(J)"})
	_ = table.Bulk(rows)
	// removed because testcase gets a trailing whitespace which fails CI
	// table.Caption(tw.Caption{
	// 	Text: "Kepler Node Power",
	// 	Spot: tw.SpotTopLeft,
	// })
	_ = table.Render()
}

func (e *Exporter) Shutdown() error {
	return e.out.Close()
}

// Name implements service.Name
func (e *Exporter) Name() string {
	return "stdout"
}
