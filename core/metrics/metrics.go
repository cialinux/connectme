package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type Metrics struct {
	requests atomic.Uint64
	inFlight atomic.Int64
}

func New() *Metrics { return &Metrics{} }
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.requests.Add(1)
		m.inFlight.Add(1)
		defer m.inFlight.Add(-1)
		next.ServeHTTP(w, r)
	})
}
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# TYPE connectme_http_requests_total counter\nconnectme_http_requests_total %d\n# TYPE connectme_http_requests_in_flight gauge\nconnectme_http_requests_in_flight %d\n", m.requests.Load(), m.inFlight.Load())
	})
}
