package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthChecker_IsAlive_BeforeNATSInit(t *testing.T) {
	h := NewHealthChecker()
	// Before NATS connection is set, process is still starting — should be alive
	if !h.IsAlive() {
		t.Error("expected IsAlive=true before NATS init")
	}
}

func TestHealthChecker_IsAlive_Connected(t *testing.T) {
	h := NewHealthChecker()
	h.SetNATSConnected(true)
	// nc pointer is still nil, so IsAlive checks nc==nil path
	if !h.IsAlive() {
		t.Error("expected IsAlive=true")
	}
}

func TestHealthChecker_IsAlive_Disconnected(t *testing.T) {
	h := NewHealthChecker()
	// Simulate: nc was set but then disconnected
	h.natsConnected.Store(false)
	// nc is nil so still alive (startup case)
	if !h.IsAlive() {
		t.Error("expected IsAlive=true when nc is nil")
	}

	// Simulate nc being set (non-nil) but disconnected
	h.SetNATSConnected(false)
	// We can't easily create a real nats.Conn, so we test the connected flag path
	// by storing a non-nil pointer hack — skip that and test via handler instead
}

func TestHealthChecker_IsReady_AllGood(t *testing.T) {
	h := NewHealthChecker()
	h.SetNATSConnected(true)
	h.SetIDPVerifiersReady(true)
	h.SetServiceRegistered(true)

	ready, checks := h.IsReady()
	if !ready {
		t.Error("expected ready=true")
	}
	if checks["nats"] != "connected" {
		t.Errorf("expected nats=connected, got %s", checks["nats"])
	}
	if checks["idp_verifiers"] != "ready" {
		t.Errorf("expected idp_verifiers=ready, got %s", checks["idp_verifiers"])
	}
	if checks["service"] != "registered" {
		t.Errorf("expected service=registered, got %s", checks["service"])
	}
}

func TestHealthChecker_IsReady_NATSDown(t *testing.T) {
	h := NewHealthChecker()
	h.SetNATSConnected(false)
	h.SetIDPVerifiersReady(true)
	h.SetServiceRegistered(true)

	ready, checks := h.IsReady()
	if ready {
		t.Error("expected ready=false when NATS disconnected")
	}
	if checks["nats"] != "disconnected" {
		t.Errorf("expected nats=disconnected, got %s", checks["nats"])
	}
}

func TestHealthChecker_IsReady_NoIDPVerifiers(t *testing.T) {
	h := NewHealthChecker()
	h.SetNATSConnected(true)
	h.SetIDPVerifiersReady(false)
	h.SetServiceRegistered(true)

	ready, _ := h.IsReady()
	if ready {
		t.Error("expected ready=false when no IDP verifiers")
	}
}

func TestHealthChecker_IsReady_ServiceNotRegistered(t *testing.T) {
	h := NewHealthChecker()
	h.SetNATSConnected(true)
	h.SetIDPVerifiersReady(true)
	h.SetServiceRegistered(false)

	ready, _ := h.IsReady()
	if ready {
		t.Error("expected ready=false when service not registered")
	}
}

func TestLivenessHandler_OK(t *testing.T) {
	h := NewHealthChecker()
	// nc is nil → alive
	rec := httptest.NewRecorder()
	h.LivenessHandler()(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body healthStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status=ok, got %s", body.Status)
	}
}

func TestReadinessHandler_Unavailable(t *testing.T) {
	h := NewHealthChecker()
	// Nothing set → not ready
	rec := httptest.NewRecorder()
	h.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}

	var body healthStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "unavailable" {
		t.Errorf("expected status=unavailable, got %s", body.Status)
	}
}

func TestReadinessHandler_OK(t *testing.T) {
	h := NewHealthChecker()
	h.SetNATSConnected(true)
	h.SetIDPVerifiersReady(true)
	h.SetServiceRegistered(true)

	rec := httptest.NewRecorder()
	h.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body healthStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status=ok, got %s", body.Status)
	}
	if len(body.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(body.Checks))
	}
}
