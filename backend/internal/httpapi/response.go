// Package httpapi exposes the read-only AudioMuse knowledge API.
//
// Handlers read only from the service layer. No handler touches the filesystem, and the
// router rejects every mutating HTTP method before routing runs.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Stable error codes. Clients match on these rather than on message prose.
const (
	CodeNotFound        = "not_found"
	CodeNodeNotFound    = "node_not_found"
	CodeSessionNotFound = "session_not_found"
	CodeInvalidQuery    = "invalid_query"
	CodeMethodNotAllow  = "method_not_allowed"
	CodeInternal        = "internal_error"
)

// errorBody is the stable error envelope.
//
// It carries a code and a human-readable message and nothing else. Go errors, stack traces
// and filesystem paths stay in the local log: an API consumer cannot act on them and
// exposing them would describe the operator's machine to anything that can reach the port.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serialises a successful response.
//
// The payload is marshalled before any header is written so that a marshalling failure can
// still produce a well-formed 500 instead of a truncated body under a 200 status.
func writeJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		logger.ErrorContext(r.Context(), "encode response", "path", r.URL.Path, "error", err)
		writeError(w, r, logger, http.StatusInternalServerError, CodeInternal, "The response could not be encoded.")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	// HEAD requests are served by the same handlers; net/http discards the body itself.
	_, _ = w.Write(body)
}

// writeError serialises the stable error envelope.
func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, code, message string) {
	body, err := json.Marshal(errorBody{Error: errorDetail{Code: code, Message: message}})
	if err != nil {
		logger.ErrorContext(r.Context(), "encode error response", "path", r.URL.Path, "error", err)
		http.Error(w, `{"error":{"code":"internal_error","message":"Internal error."}}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
