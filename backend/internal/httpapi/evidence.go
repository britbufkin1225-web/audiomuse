package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The evidence-layer handlers. They stay as thin as the Phase 1A handlers: parse and bound
// the query string, hand it to the immutable index, serialise the result. No filtering,
// resolution or ordering logic lives here.

// sourceParams and claimParams are the complete accepted query strings for the two
// collection endpoints. rejectUnknownParams refuses anything else, so a caller can never be
// handed a result set that silently dropped a filter they believed was applied.
var (
	sourceParams = []string{
		"q", "type", "relationship", "evidence_class", "retrieval",
		"claim_id", "node_id", "session_id", "limit", "offset",
	}
	claimParams = []string{
		"q", "claim_type", "confidence", "dispute_status", "temporal_precision",
		"relation", "source_id", "node_id", "session_id", "limit", "offset",
	}
)

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, sourceParams...) {
		return
	}
	query := r.URL.Query()

	values, ok := boundedParams(w, r, s.logger, query,
		"q", "type", "relationship", "evidence_class", "retrieval", "claim_id", "node_id", "session_id")
	if !ok {
		return
	}
	limit, offset, ok := pagingParams(w, r, s.logger, query)
	if !ok {
		return
	}

	list, err := s.knowledge.ListSources(service.SourceQuery{
		Q:             values["q"],
		Type:          values["type"],
		Relationship:  values["relationship"],
		EvidenceClass: values["evidence_class"],
		Retrieval:     values["retrieval"],
		ClaimID:       values["claim_id"],
		NodeID:        values["node_id"],
		SessionID:     values["session_id"],
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		writeFilterError(w, r, s.logger, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, list)
}

func (s *Server) handleSourceByID(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	id, ok := pathID(w, r, s.logger)
	if !ok {
		return
	}
	source, err := s.knowledge.SourceByID(id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, r, s.logger, http.StatusNotFound, CodeSourceNotFound, "Source was not found.")
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "source lookup", "error", err)
		writeError(w, r, s.logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, source)
}

func (s *Server) handleClaims(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, claimParams...) {
		return
	}
	query := r.URL.Query()

	values, ok := boundedParams(w, r, s.logger, query,
		"q", "claim_type", "confidence", "dispute_status", "temporal_precision",
		"relation", "source_id", "node_id", "session_id")
	if !ok {
		return
	}
	limit, offset, ok := pagingParams(w, r, s.logger, query)
	if !ok {
		return
	}

	list, err := s.knowledge.ListClaims(service.ClaimQuery{
		Q:                 values["q"],
		ClaimType:         values["claim_type"],
		Confidence:        values["confidence"],
		DisputeStatus:     values["dispute_status"],
		TemporalPrecision: values["temporal_precision"],
		Relation:          values["relation"],
		SourceID:          values["source_id"],
		NodeID:            values["node_id"],
		SessionID:         values["session_id"],
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		writeFilterError(w, r, s.logger, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, list)
}

func (s *Server) handleClaimByID(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	id, ok := pathID(w, r, s.logger)
	if !ok {
		return
	}
	claim, err := s.knowledge.ClaimByID(id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, r, s.logger, http.StatusNotFound, CodeClaimNotFound, "Claim was not found.")
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "claim lookup", "error", err)
		writeError(w, r, s.logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, claim)
}

// writeFilterError renders a rejected filter value as 400 invalid_query.
//
// The message names the parameter and lists the permitted values, which come from the
// canonical contract read at startup and are therefore repository data, not caller input.
// The caller's own value is never echoed: it would be the one part of the response an
// attacker controls, and it adds nothing the caller does not already know. Any other error
// from the service is treated as internal and its detail stays in the local log.
func writeFilterError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var invalid *service.InvalidFilterError
	if errors.As(err, &invalid) {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Unsupported value for "+invalid.Param+". Supported: "+strings.Join(invalid.Allowed, ", ")+".")
		return
	}
	logger.ErrorContext(r.Context(), "evidence query", "error", err)
	writeError(w, r, logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
}

// boundedParams validates a set of caller-supplied text parameters in a fixed order, so the
// error a caller receives for two bad parameters is the same on every run.
func boundedParams(
	w http.ResponseWriter, r *http.Request, logger *slog.Logger,
	query map[string][]string, names ...string,
) (map[string]string, bool) {
	out := make(map[string]string, len(names))
	for _, name := range names {
		var raw string
		if values := query[name]; len(values) > 0 {
			raw = values[0]
		}
		value, ok := boundedText(w, r, logger, raw, name)
		if !ok {
			return nil, false
		}
		out[name] = value
	}
	return out, true
}

// pagingParams parses the shared limit and offset parameters.
func pagingParams(w http.ResponseWriter, r *http.Request, logger *slog.Logger, query map[string][]string) (int, int, bool) {
	first := func(name string) string {
		if values := query[name]; len(values) > 0 {
			return values[0]
		}
		return ""
	}
	limit, ok := intParam(w, r, logger, first("limit"), "limit")
	if !ok {
		return 0, 0, false
	}
	offset, ok := intParam(w, r, logger, first("offset"), "offset")
	if !ok {
		return 0, 0, false
	}
	return limit, offset, true
}
