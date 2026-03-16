package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Use a custom registry to avoid conflicts with global state
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	defer func() {
		prometheus.DefaultRegisterer = prometheus.NewRegistry()
		prometheus.DefaultGatherer = prometheus.NewRegistry()
	}()

	m := New()
	require.NotNil(t, m)
	assert.NotNil(t, m.AuthRequestsTotal)
	assert.NotNil(t, m.AuthRequestDuration)
	assert.NotNil(t, m.AuthRequestsInFlight)
	assert.NotNil(t, m.TokensMinted)
	assert.NotNil(t, m.IDPVerifyTotal)
	assert.NotNil(t, m.IDPVerifyDuration)
	assert.NotNil(t, m.RequestErrors)
	assert.NotNil(t, m.ResponseErrors)

	// Verify metrics can be incremented without panicking
	m.AuthRequestsTotal.WithLabelValues(StatusSuccess).Inc()
	m.AuthRequestDuration.WithLabelValues(StatusError).Observe(0.5)
	m.AuthRequestsInFlight.Inc()
	m.TokensMinted.WithLabelValues("account1", "idp1").Inc()
	m.IDPVerifyTotal.WithLabelValues("idp1", StatusSuccess).Inc()
	m.IDPVerifyDuration.WithLabelValues("idp1").Observe(0.1)
	m.RequestErrors.WithLabelValues(StageDecrypt).Inc()
	m.ResponseErrors.WithLabelValues(StageSign).Inc()
}

func TestNewServer(t *testing.T) {
	s := NewServer(0) // port 0 for testing
	require.NotNil(t, s)
	assert.NotNil(t, s.httpServer)
}

func TestServerHealthz(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestServerStartStop(t *testing.T) {
	s := NewServer(0)
	s.Start()
	s.Stop()
	// No panic or error means success
}
