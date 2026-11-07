package middleware

import "net/http"

type responseWriter struct {
	http.ResponseWriter
	status int
}

func newResponseWriter(writer http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: writer, status: http.StatusOK}
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	return w.ResponseWriter.Write(body)
}
