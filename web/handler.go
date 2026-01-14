// Copyright 2018 Thibault Chataigner <thibault.chataigner@gmail.com>
// Copyright 2024-2025 NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package web

import (
	"fmt"
	"html"
	"net/http"
	"sync"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client"
	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/ui"
	"github.com/Netcracker/qubership-graphite-remote-adapter/utils/template"
	"github.com/davecgh/go-spew/spew"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"log/slog"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

const namespace = "remote_adapter"
const apiSubsystem = "api"

var (
	requestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: apiSubsystem,
			Name:      "requests_total",
			Help:      "A counter for requests to the wrapped handler.",
		},
		[]string{"handler", "code", "method"},
	)
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: apiSubsystem,
			Name:      "request_duration_seconds",
			Help:      "A histogram of latencies for requests.",
			Buckets:   []float64{.25, .5, 1, 2.5, 5, 10},
		},
		[]string{"handler", "method"},
	)
	responseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: apiSubsystem,
			Name:      "response_size_bytes",
			Help:      "A histogram of response sizes for requests.",
			Buckets:   []float64{200, 500, 900, 1500},
		},
		[]string{"handler"},
	)
)

// Handler serves various HTTP endpoints of the remote adapter server
type Handler struct {
	logger *slog.Logger

	cfg      *config.Config
	router   *mux.Router
	reloadCh chan chan error

	writers []client.Writer
	readers []client.Reader

	lock sync.RWMutex
}

func instrumentHandler(name string, handlerFunc http.HandlerFunc) http.Handler {
	return promhttp.InstrumentHandlerDuration(
		requestDuration.MustCurryWith(prometheus.Labels{"handler": name}),
		promhttp.InstrumentHandlerCounter(
			requestCounter.MustCurryWith(prometheus.Labels{"handler": name}),
			promhttp.InstrumentHandlerResponseSize(
				responseSize.MustCurryWith(prometheus.Labels{"handler": name}),
				handlerFunc,
			),
		),
	)
}

// New initializes a new web Handler.
func New(logger *slog.Logger, cfg *config.Config) *Handler {
	router := mux.NewRouter()
	h := &Handler{
		cfg:      cfg,
		logger:   logger,
		router:   router,
		reloadCh: make(chan chan error),
	}
	h.buildClients()

	staticFs := http.FileServer(
		&assetfs.AssetFS{Asset: ui.Asset, AssetDir: ui.AssetDir, AssetInfo: ui.AssetInfo, Prefix: ""})

	// Add pprof handler.
	router.PathPrefix("/debug/").Handler(http.DefaultServeMux)

	// Add your routes as needed
	router.Methods(http.MethodGet).PathPrefix("/static/").Handler(staticFs)

	router.Methods(http.MethodGet).Path(h.cfg.Web.TelemetryPath).Handler(promhttp.Handler())
	router.Methods(http.MethodGet).Path("/-/healthy").Handler(instrumentHandler("healthy", h.healthy))
	router.Methods(http.MethodPost).Path("/-/reload").Handler(instrumentHandler("reload", h.reload))
	router.Methods(http.MethodGet).Path("/").Handler(instrumentHandler("home", h.home))
	router.Methods(http.MethodGet).Path("/simulation").Handler(instrumentHandler("home", h.simulation))

	router.Methods(http.MethodPost).Path("/write").Handler(instrumentHandler("write", h.write))
	router.Methods(http.MethodPost).Path("/read").Handler(instrumentHandler("read", h.read))

	return h
}

// Reload returns the receive-only channel that signals configuration reload requests.
func (h *Handler) Reload() <-chan chan error {
	return h.reloadCh
}

// ApplyConfig updates the config field of the Handler struct
func (h *Handler) ApplyConfig(cfg *config.Config) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	for _, w := range h.writers {
		w.Shutdown()
	}
	for _, r := range h.readers {
		r.Shutdown()
	}

	h.cfg = cfg
	h.buildClients()

	return nil
}

func (h *Handler) buildClients() {
	h.logger.Info("Building clients", "cfg", h.cfg)
	h.writers = nil
	h.readers = nil
	if c := graphite.NewClient(h.cfg, h.logger); c != nil {
		h.writers = append(h.writers, c)
		h.readers = append(h.readers, c)
	}
	h.logger.Info("Built clients", "num_writers", len(h.writers), "num_readers", len(h.readers))
}

// Run serves the HTTP endpoints.
func (h *Handler) Run() error {
	h.logger.Info("Listening", "ListenAddress", h.cfg.Web.ListenAddress)
	return http.ListenAndServe(h.cfg.Web.ListenAddress, h.router)
}

func (h *Handler) healthy(w http.ResponseWriter, r *http.Request) {
	h.lock.RLock()
	defer h.lock.RUnlock()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	_, _ = fmt.Fprintf(w, "OK")
}

func (h *Handler) reload(w http.ResponseWriter, r *http.Request) {
	rc := make(chan error)
	h.reloadCh <- rc
	if err := <-rc; err != nil {
		http.Error(w, fmt.Sprintf("failed to reload config: %s", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "Config succesfully reloaded.")
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	status := struct {
		VersionInfo         string
		VersionBuildContext string
		Cfg                 string
		Readers             map[string]string
		Writers             map[string]string
	}{
		VersionInfo:         version.Info(),
		VersionBuildContext: version.BuildContext(),
		Cfg:                 html.EscapeString(spew.Sdump(h.cfg)),
		Readers:             map[string]string{},
		Writers:             map[string]string{},
	}
	for _, r := range h.readers {
		status.Readers[r.Name()] = html.EscapeString(spew.Sdump(r))
	}
	for _, w := range h.writers {
		status.Writers[w.Name()] = html.EscapeString(spew.Sdump(w))
	}

	bytes, err := template.ExecuteTemplate("status.html", status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytes)
}

func (h *Handler) simulation(w http.ResponseWriter, r *http.Request) {
	bytes, err := template.ExecuteTemplate("simulation.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytes)
}
