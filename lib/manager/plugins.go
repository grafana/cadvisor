// Copyright 2024 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"net/http"
	"time"

	"github.com/google/cadvisor/lib/container"
	info "github.com/google/cadvisor/lib/model"
	"github.com/google/cadvisor/lib/stats"
)

// Collector-manager injection seam.
//
// The kubelet-only library does not import the perf_event / resctrl
// implementations (or their dependencies). Instead, the full cAdvisor binary
// registers factories here at init time; when a factory is nil (the kubelet
// case) the manager falls back to a Noop manager, so no stats are collected for
// that controller. This keeps github.com/google/cadvisor/lib lean while letting
// the binary remain fully functional.
//
// These package-level vars are read exactly once, by New(), and their value
// at that instant is captured onto the manager instance — never re-read for
// the life of the manager. That matters for an embedder (e.g. Alloy) that
// reconfigures and re-inits on every reload: reassigning these globals would
// otherwise race with, and eventually clobber, any already-running manager's
// behavior. Such a caller should instead set Config.Factories on the New()
// call for each manager instance and never touch these vars at all; the
// globals remain only as the default the full cAdvisor binary's cmd/plugins.go
// wires once for the process.
var (
	// PerfManagerFactory builds the perf_event stats manager. Set by the root
	// binary to perf.NewManager.
	PerfManagerFactory func(configFile string, topology []info.Node) (stats.Manager, error)

	// ResctrlManagerFactory builds the resctrl stats manager. Set by the root
	// binary to a closure over resctrl/intel.NewManager.
	ResctrlManagerFactory func(interval time.Duration, vendorID string, inHostNamespace bool) (stats.ResctrlManager, error)
)

// SummaryReader computes rolling-window derived/percentile usage stats for a
// single container. The full binary injects a reader backed by the summary
// package; the kubelet leaves SummaryReaderFactory nil and no derived stats are
// produced (GetDerivedStats reports "not enabled").
type SummaryReader interface {
	AddSample(stat info.ContainerStats) error
	DerivedStats() (info.DerivedStats, error)
}

// SummaryReaderFactory builds a per-container summary reader from its spec. Set
// by the root binary (wrapping summary.New).
var SummaryReaderFactory func(spec info.ContainerSpec) (SummaryReader, error)

// ProcessListProvider lists the processes running in a container (the v2 /ps
// endpoint). containerName is the container's cgroup name; isRoot reports
// whether it is the root container. Set by the root binary to a ps-based
// implementation; nil for the kubelet (GetProcessList returns empty).
var ProcessListProvider func(containerName string, isRoot bool, cadvisorContainer string, inHostNamespace bool) ([]info.ProcessInfo, error)

// CollectorManager runs application-metrics collectors for one container and
// aggregates their custom metrics. The full binary injects a manager backed by
// the collector package; the kubelet leaves CollectorManagerFactory nil (no
// custom metrics).
type CollectorManager interface {
	Collect() (time.Time, map[string][]info.MetricVal, error)
	GetSpec() ([]info.MetricSpec, error)
}

// CollectorManagerFactory builds a per-container collector manager. It receives
// the container handler (to discover collector configs from its labels), a
// readFile func (to read those config files from inside the container), and
// the HTTP client to scrape collector endpoints with (built from the
// collectorHTTPClient passed to manager.New, so callers such as Alloy can
// configure it per instance instead of through a process-global setter). Set
// by the root binary.
var CollectorManagerFactory func(handler container.ContainerHandler, readFile func(string) ([]byte, error), httpClient *http.Client) (CollectorManager, error)

// CpuLoadReader reads per-container CPU load over netlink (runnable/uninterruptible
// task counts). The implementation lives in the root binary's utils/cpuload
// (CAP_NET_ADMIN, a persistent netlink connection); the kubelet leaves
// CpuLoadReaderFactory nil so no reader is created and LoadAverage stays zero.
type CpuLoadReader interface {
	Start() error
	Stop()
	GetCpuLoad(name string, path string) (info.LoadStats, error)
}

// CpuLoadReaderFactory builds a CpuLoadReader. Set by the root binary when
// --enable_load_reader is on; nil for the kubelet.
var CpuLoadReaderFactory func() (CpuLoadReader, error)

// Factories overrides, per manager instance, one or more of the seams above
// that are otherwise sourced from the package-level *Factory vars / providers.
// A nil field falls back to the corresponding global. Pass this via
// Config.Factories to New() so different manager instances in the same
// process (or across reloads) never need to mutate the globals themselves.
type Factories struct {
	Perf        func(configFile string, topology []info.Node) (stats.Manager, error)
	Resctrl     func(interval time.Duration, vendorID string, inHostNamespace bool) (stats.ResctrlManager, error)
	Summary     func(spec info.ContainerSpec) (SummaryReader, error)
	ProcessList func(containerName string, isRoot bool, cadvisorContainer string, inHostNamespace bool) ([]info.ProcessInfo, error)
	Collector   func(handler container.ContainerHandler, readFile func(string) ([]byte, error), httpClient *http.Client) (CollectorManager, error)
	CpuLoad     func() (CpuLoadReader, error)
}
