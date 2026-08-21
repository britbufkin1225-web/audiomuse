package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// maxIDLength bounds a path-supplied canonical ID. Canonical IDs are short kebab-case
// strings; anything longer is not a lookup, so it is rejected before it reaches the index.
const maxIDLength = 128

type healthBody struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Mode    string `json:"mode"`
}

// handleHealth reports process health.
//
// It answers from constants only and never touches the index, so it stays a genuine
// liveness signal rather than a serialisation of the whole corpus.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, healthBody{
		Status:  "ok",
		Service: "audiomuse-api",
		Mode:    service.ModeReadOnly,
	})
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, s.knowledge.Project())
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, s.knowledge.Graph())
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, s.knowledge.Diagnostics())
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, "q", "domain", "status", "session", "limit", "offset") {
		return
	}
	query := r.URL.Query()

	q, ok := boundedText(w, r, s.logger, query.Get("q"), "q")
	if !ok {
		return
	}
	limit, ok := intParam(w, r, s.logger, query.Get("limit"), "limit")
	if !ok {
		return
	}
	offset, ok := intParam(w, r, s.logger, query.Get("offset"), "offset")
	if !ok {
		return
	}
	domainFilter, ok := boundedText(w, r, s.logger, query.Get("domain"), "domain")
	if !ok {
		return
	}
	statusFilter, ok := boundedText(w, r, s.logger, query.Get("status"), "status")
	if !ok {
		return
	}
	sessionFilter, ok := boundedText(w, r, s.logger, query.Get("session"), "session")
	if !ok {
		return
	}

	writeJSON(w, r, s.logger, http.StatusOK, s.knowledge.ListNodes(service.NodeQuery{
		Q:       q,
		Domain:  domainFilter,
		Status:  statusFilter,
		Session: sessionFilter,
		Limit:   limit,
		Offset:  offset,
	}))
}

func (s *Server) handleNodeByID(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	id, ok := pathID(w, r, s.logger)
	if !ok {
		return
	}
	node, err := s.knowledge.NodeByID(id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, r, s.logger, http.StatusNotFound, CodeNodeNotFound, "Node was not found.")
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "node lookup", "error", err)
		writeError(w, r, s.logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, node)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, "q", "limit", "offset") {
		return
	}
	query := r.URL.Query()

	q, ok := boundedText(w, r, s.logger, query.Get("q"), "q")
	if !ok {
		return
	}
	limit, ok := intParam(w, r, s.logger, query.Get("limit"), "limit")
	if !ok {
		return
	}
	offset, ok := intParam(w, r, s.logger, query.Get("offset"), "offset")
	if !ok {
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, s.knowledge.ListSessions(service.SessionQuery{
		Q: q, Limit: limit, Offset: offset,
	}))
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	id, ok := pathID(w, r, s.logger)
	if !ok {
		return
	}
	session, err := s.knowledge.SessionByID(id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, r, s.logger, http.StatusNotFound, CodeSessionNotFound, "Session was not found.")
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "session lookup", "error", err)
		writeError(w, r, s.logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, session)
}

// pathID extracts and bounds a canonical ID from the route.
//
// The value is used only as a map key against the in-memory index. It is never joined to a
// path, opened, or handed to the filesystem, so no route here can become a file reader. The
// separator and traversal rejections are defence in depth against that ever changing.
func pathID(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (string, bool) {
	id := r.PathValue("id")
	if id == "" || len(id) > maxIDLength {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Identifier is missing or too long.")
		return "", false
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") || strings.ContainsRune(id, 0) {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Identifier contains characters that are not valid in a canonical AudioMuse ID.")
		return "", false
	}
	return id, true
}

// boundedText validates a caller-supplied filter or search term.
//
// Values longer than the service bound are refused rather than truncated, so a caller is
// never silently given the results of a query they did not ask for.
func boundedText(w http.ResponseWriter, r *http.Request, logger *slog.Logger, value, name string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > service.MaxQueryChars {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter "+name+" exceeds "+strconv.Itoa(service.MaxQueryChars)+" characters.")
		return "", false
	}
	if strings.ContainsRune(trimmed, 0) {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter "+name+" contains an invalid character.")
		return "", false
	}
	return trimmed, true
}

// intParam parses a bounded non-negative integer parameter. An absent parameter yields 0,
// which the service reads as "use the default".
func intParam(w http.ResponseWriter, r *http.Request, logger *slog.Logger, value, name string) (int, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < 0 {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter "+name+" must be a non-negative integer.")
		return 0, false
	}
	return parsed, true
}
