package metrics

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/nats-io/nats.go"
)

// HealthChecker tracks the health and readiness state of the broker.
// All methods are safe for concurrent use.
type HealthChecker struct {
	nc                atomic.Pointer[nats.Conn]
	natsConnected     atomic.Bool
	idpVerifiersReady atomic.Bool
	serviceRegistered atomic.Bool
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{}
}

// SetNATSConn stores the NATS connection for status checks.
func (h *HealthChecker) SetNATSConn(nc *nats.Conn) {
	h.nc.Store(nc)
	h.natsConnected.Store(nc != nil && nc.IsConnected())
}

// SetNATSConnected updates the NATS connection state.
// Called from disconnect/reconnect handlers.
func (h *HealthChecker) SetNATSConnected(connected bool) {
	h.natsConnected.Store(connected)
}

// SetIDPVerifiersReady marks whether at least one IDP verifier is available.
func (h *HealthChecker) SetIDPVerifiersReady(ready bool) {
	h.idpVerifiersReady.Store(ready)
}

// SetServiceRegistered marks whether the micro service endpoint is registered.
func (h *HealthChecker) SetServiceRegistered(registered bool) {
	h.serviceRegistered.Store(registered)
}

// healthStatus is the JSON response for health endpoints.
type healthStatus struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// IsAlive returns true if the broker process is alive and the NATS
// connection is active (or was never set, e.g. during startup).
func (h *HealthChecker) IsAlive() bool {
	nc := h.nc.Load()
	if nc == nil {
		// NATS not yet initialised — process is still starting up, consider alive
		return true
	}
	return h.natsConnected.Load()
}

// IsReady returns true when the broker can serve auth requests.
func (h *HealthChecker) IsReady() (bool, map[string]string) {
	checks := map[string]string{}

	natsOK := h.natsConnected.Load()
	if natsOK {
		checks["nats"] = "connected"
	} else {
		checks["nats"] = "disconnected"
	}

	idpOK := h.idpVerifiersReady.Load()
	if idpOK {
		checks["idp_verifiers"] = "ready"
	} else {
		checks["idp_verifiers"] = "not_ready"
	}

	svcOK := h.serviceRegistered.Load()
	if svcOK {
		checks["service"] = "registered"
	} else {
		checks["service"] = "not_registered"
	}

	return natsOK && idpOK && svcOK, checks
}

// LivenessHandler returns an http.HandlerFunc for /healthz.
func (h *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if h.IsAlive() {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(healthStatus{Status: "ok"})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(healthStatus{Status: "unavailable"})
		}
	}
}

// ReadinessHandler returns an http.HandlerFunc for /readyz.
func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready, checks := h.IsReady()
		if ready {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(healthStatus{Status: "ok", Checks: checks})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(healthStatus{Status: "unavailable", Checks: checks})
		}
	}
}
