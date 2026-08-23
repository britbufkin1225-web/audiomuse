package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The discovery handler: Phase 1E search, the Phase 1F context control and the Phase 1G
// query-composition control. It stays as thin as
// every other handler in this package: bound the query string, hand it to the immutable index,
// map the typed error, serialise. No matching, ordering, paging or relationship-resolution logic
// lives here — all of it belongs to the service and is tested there directly.

// searchParams is the complete accepted query string. rejectUnknownParams refuses anything
// else, and refuses a parameter supplied twice, so a caller can never be handed a result set
// that silently dropped a filter they believed was applied.
var searchParams = []string{"q", "type", "query_mode", "include_context", "limit", "offset"}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, searchParams...) {
		return
	}
	query := r.URL.Query()

	// q travels through the same bound every other lexical search parameter does: it is
	// trimmed, refused rather than truncated past the service ceiling, and refused if it
	// carries a NUL. It is then plain text and nothing else — never a path, a pattern or a
	// program. See service.Search.
	values, ok := boundedParams(w, r, s.logger, query, "q", "type", "query_mode")
	if !ok {
		return
	}
	// query_mode is the Phase 1G composition control. An absent parameter is the Phase 1E
	// literal search unchanged, so whitespace in q stays part of the phrase; the service decides
	// whether a supplied value names a mode. The one check that belongs here is the wire one: a
	// present but blank parameter is a caller mistake, and treating it as "absent" would hand
	// back literal results to someone who believes they asked for a composed query.
	if _, present := query["query_mode"]; present && values["query_mode"] == "" {
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter query_mode must be one of: "+strings.Join(domain.SearchQueryModeNames(), ", ")+".")
		return
	}
	// include_context is the Phase 1F control and is a plain flag rather than a depth, a field
	// list or a relationship filter. Context is either the record's bounded canonical context or
	// it is absent; letting a caller shape it would make the response a query result rather than
	// a fixed projection, and would put the bounds in reach of the request.
	//
	// boolParam is shared with tri-state filters and treats an empty value as absent. Here the
	// control's wire contract is stricter: once the key is present it must carry one of the two
	// documented spellings, otherwise a malformed request would silently receive the plain
	// Phase 1E response shape.
	if _, present := query["include_context"]; present && strings.TrimSpace(query.Get("include_context")) == "" {
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter include_context must be exactly true or false.")
		return
	}
	includeContext, ok := boolParam(w, r, s.logger, query.Get("include_context"), "include_context")
	if !ok {
		return
	}
	limit, offset, ok := pagingParams(w, r, s.logger, query)
	if !ok {
		return
	}

	results, err := s.knowledge.Search(service.SearchQuery{
		Q:              values["q"],
		Type:           values["type"],
		Mode:           values["query_mode"],
		IncludeContext: includeContext != nil && *includeContext,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		s.writeSearchError(w, r, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, results)
}

// writeSearchError maps the service's typed errors onto the stable envelope.
//
// A search that matched nothing is not an error and never reaches here: it is a 200 carrying
// an empty result list, because "the corpus contains no such text" is an answer. Only a
// malformed request lands here, and both cases are the existing invalid_query code rather than
// a new search-specific one.
func (s *Server) writeSearchError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, service.ErrEmptySearchQuery) {
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter q is required and must not be empty.")
		return
	}
	if errors.Is(err, service.ErrSearchQueryTooLong) {
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter q exceeds the maximum length.")
		return
	}
	// The multi-term bounds. The message states the bound rather than echoing the caller's
	// terms, and it names the mode, because the same q is a valid literal query.
	if errors.Is(err, service.ErrTooFewSearchTerms) {
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter q must supply at least "+strconv.Itoa(service.MinSearchTerms)+
				" distinct whitespace-separated terms when query_mode is all_terms.")
		return
	}
	if errors.Is(err, service.ErrTooManySearchTerms) {
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter q must supply at most "+strconv.Itoa(service.MaxSearchTerms)+
				" distinct whitespace-separated terms when query_mode is all_terms.")
		return
	}
	// An unsupported type arrives as an InvalidFilterError and is rendered by the Phase 1B
	// mapping, which lists the permitted values and never echoes the caller's own.
	writeFilterError(w, r, s.logger, err)
}
