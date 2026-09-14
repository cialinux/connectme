package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/connectme/connectme/core/config"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	server *http.Server
	log    *slog.Logger
}

func New(cfg config.HTTP, handler http.Handler, log *slog.Logger) *Server {
	return &Server{server: &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: cfg.ReadTimeout, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout, MaxHeaderBytes: cfg.MaxHeaderBytes}, log: log}
}
func (s *Server) Run() error {
	s.log.Info("http server listening", "address", s.server.Addr)
	err := s.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
func (s *Server) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func Middleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Correlation-ID")
		if id == "" {
			var b [16]byte
			if _, err := rand.Read(b[:]); err == nil {
				id = hex.EncodeToString(b[:])
			}
		}
		w.Header().Set("X-Correlation-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Info("http request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration_ms", time.Since(start).Milliseconds(), "correlation_id", id)
	})
}
