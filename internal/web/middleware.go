package web

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("HTTP 处理发生 panic", "panic", recovered, "stack", string(debug.Stack()))
				if writer.status == 0 {
					writeJSON(writer, http.StatusInternalServerError, errorBody{Error: apiError{Code: "internal_error", Message: "服务发生内部错误"}})
				}
			}
			slog.Info("http_access", "method", r.Method, "path", r.URL.Path, "status", writer.status, "bytes", writer.bytes, "duration_ms", time.Since(started).Milliseconds(), "remote", r.RemoteAddr)
		}()
		next.ServeHTTP(writer, r)
	})
}
