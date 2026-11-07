package middleware

import "net/http"

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newResponseWriter(writer http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: writer, status: http.StatusOK}
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	if status >= http.StatusOK || status == http.StatusSwitchingProtocols {
		w.wroteHeader = true
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
