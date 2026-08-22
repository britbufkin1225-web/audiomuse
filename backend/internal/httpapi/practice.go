package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The practice-layer handlers. They stay as thin as the Phase 1A and 1B handlers: parse and
// bound the query string, hand it to the immutable index, serialise the result. No filtering,
// resolution or ordering logic lives here.

// The complete accepted query string for each collection endpoint. rejectUnknownParams refuses
// anything else, so a caller can never be handed a result set that silently dropped a filter
// they believed was applied.
var (
	vocabularyParams = []string{"q", "domain", "node_id", "session_id", "tag", "limit", "offset"}
	experimentParams = []string{
		"q", "status", "type", "difficulty",
		"node_id", "vocabulary_id", "session_id", "source_id", "limit", "offset",
	}
	experimentRunParams = []string{"experiment_id", "status", "source_id", "performed", "limit", "offset"}
)

func (s *Server) handleVocabulary(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, vocabularyParams...) {
		return
	}
	query := r.URL.Query()

	values, ok := boundedParams(w, r, s.logger, query, "q", "domain", "node_id", "session_id", "tag")
	if !ok {
		return
	}
	limit, offset, ok := pagingParams(w, r, s.logger, query)
	if !ok {
		return
	}

	writeJSON(w, r, s.logger, http.StatusOK, s.knowledge.ListVocabulary(service.VocabularyQuery{
		Q:         values["q"],
		Domain:    values["domain"],
		NodeID:    values["node_id"],
		SessionID: values["session_id"],
		Tag:       values["tag"],
		Limit:     limit,
		Offset:    offset,
	}))
}

func (s *Server) handleVocabularyByID(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	id, ok := pathID(w, r, s.logger)
	if !ok {
		return
	}
	entry, err := s.knowledge.VocabularyByID(id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, r, s.logger, http.StatusNotFound, CodeVocabularyNotFound, "Vocabulary entry was not found.")
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "vocabulary lookup", "error", err)
		writeError(w, r, s.logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, entry)
}

func (s *Server) handleExperiments(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, experimentParams...) {
		return
	}
	query := r.URL.Query()

	values, ok := boundedParams(w, r, s.logger, query,
		"q", "status", "type", "difficulty", "node_id", "vocabulary_id", "session_id", "source_id")
	if !ok {
		return
	}
	limit, offset, ok := pagingParams(w, r, s.logger, query)
	if !ok {
		return
	}

	list, err := s.knowledge.ListExperiments(service.ExperimentQuery{
		Q:            values["q"],
		Status:       values["status"],
		Type:         values["type"],
		Difficulty:   values["difficulty"],
		NodeID:       values["node_id"],
		VocabularyID: values["vocabulary_id"],
		SessionID:    values["session_id"],
		SourceID:     values["source_id"],
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		writeFilterError(w, r, s.logger, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, list)
}

func (s *Server) handleExperimentByID(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	id, ok := pathID(w, r, s.logger)
	if !ok {
		return
	}
	experiment, err := s.knowledge.ExperimentByID(id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, r, s.logger, http.StatusNotFound, CodeExperimentNotFound, "Experiment was not found.")
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "experiment lookup", "error", err)
		writeError(w, r, s.logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, experiment)
}

func (s *Server) handleExperimentRuns(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, experimentRunParams...) {
		return
	}
	query := r.URL.Query()

	values, ok := boundedParams(w, r, s.logger, query, "experiment_id", "status", "source_id")
	if !ok {
		return
	}
	performed, ok := boolParam(w, r, s.logger, query.Get("performed"), "performed")
	if !ok {
		return
	}
	limit, offset, ok := pagingParams(w, r, s.logger, query)
	if !ok {
		return
	}

	list, err := s.knowledge.ListExperimentRuns(service.ExperimentRunQuery{
		ExperimentID: values["experiment_id"],
		Status:       values["status"],
		SourceID:     values["source_id"],
		Performed:    performed,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		writeFilterError(w, r, s.logger, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, list)
}

func (s *Server) handleExperimentRunByID(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger) {
		return
	}
	id, ok := pathID(w, r, s.logger)
	if !ok {
		return
	}
	run, err := s.knowledge.ExperimentRunByID(id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, r, s.logger, http.StatusNotFound, CodeExperimentRunNotFound, "Experiment run was not found.")
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "experiment run lookup", "error", err)
		writeError(w, r, s.logger, http.StatusInternalServerError, CodeInternal, "The request could not be completed.")
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, run)
}

// boolParam parses a strict tri-state boolean filter: absent, true, or false.
//
// Only the two exact spellings are accepted. Treating "1", "yes" or "TRUE" as true would mean
// guessing, and guessing wrong on a filter that separates planned records from performed ones
// would return the opposite set to the one the caller asked for.
func boolParam(w http.ResponseWriter, r *http.Request, logger *slog.Logger, value, name string) (*bool, bool) {
	trimmed := strings.TrimSpace(value)
	switch trimmed {
	case "":
		return nil, true
	case "true":
		yes := true
		return &yes, true
	case "false":
		no := false
		return &no, true
	default:
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter "+name+" must be exactly true or false.")
		return nil, false
	}
}
