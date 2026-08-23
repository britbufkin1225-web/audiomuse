package httpapi

import (
	"errors"
	"net/http"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The discovery handler: Phase 1E search and the Phase 1F context control. It stays as thin as
// every other handler in this package: bound the query string, hand it to the immutable index,
// map the typed error, serialise. No matching, ordering, paging or relationship-resolution logic
// lives here — all of it belongs to the service and is tested there directly.

// searchParams is the complete accepted query string. rejectUnknownParams refuses anything
// else, and refuses a parameter supplied twice, so a caller can never be handed a result set
// that silently dropped a filter they believed was applied.
var searchParams = []string{"q", "type", "include_context", "limit", "offset"}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, searchParams...) {
		return
	}
	query := r.URL.Query()

	// q travels through the same bound every other lexical search parameter does: it is
	// trimmed, refused rather than truncated past the service ceiling, and refused if it
	// carries a NUL. It is then plain text and nothing else — never a path, a pattern or a
	// program. See service.Search.
	values, ok := boundedParams(w, r, s.logger, query, "q", "type")
	if !ok {
		return
	}
	// include_context is the Phase 1F control and is a plain flag rather than a depth, a field
	// list or a relationship filter. Context is either the record's bounded canonical context or
	// it is absent; letting a caller shape it would make the response a query result rather than
	// a fixed projection, and would put the bounds in reach of the request.
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
	// An unsupported type arrives as an InvalidFilterError and is rendered by the Phase 1B
	// mapping, which lists the permitted values and never echoes the caller's own.
	writeFilterError(w, r, s.logger, err)
}
