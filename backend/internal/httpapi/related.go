package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The Phase 2A related-knowledge handler. It stays as thin as every other handler in this
// package: bound the route values, bound the query string, hand it to the immutable index, map
// the typed error, serialise. No grouping, ranking, deduplication or bounding happens here - all
// of it belongs to the service, where it is tested directly and where a future non-HTTP caller
// inherits it rather than depending on this file.

// relatedParams is the complete accepted query string. rejectUnknownParams refuses anything
// else, and refuses a parameter supplied twice, so a caller can never be handed a discovery list
// that silently dropped a filter they believed was applied.
//
// There is no offset, no depth and no q. Each is absent because the operation does not have it,
// not because the handler forgot: accepting and ignoring one would let a caller believe they had
// asked for something the response does not reflect.
var relatedParams = []string{"entity_types", "limit"}

func (s *Server) handleRelatedKnowledge(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, relatedParams...) {
		return
	}
	entityType, id, ok := relatedRoute(w, r, s.logger)
	if !ok {
		return
	}
	query := r.URL.Query()

	// entity_types is the destination scope and is split exactly as the search route splits it:
	// the comma is a wire convention, the service validates each member and decides what a class
	// list means. The split is deliberately naive - no empty members are dropped and nothing is
	// collapsed - so a leading, trailing or doubled comma survives as the blank member it is and
	// is refused, instead of being quietly repaired into a working filter.
	values, ok := boundedParams(w, r, s.logger, query, "entity_types")
	if !ok {
		return
	}
	var entityTypes []string
	if _, present := query["entity_types"]; present {
		if values["entity_types"] == "" {
			writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
				"Parameter entity_types must be a comma-separated list of: "+
					strings.Join(domain.SearchEntityTypeNames(), ", ")+".")
			return
		}
		entityTypes = strings.Split(values["entity_types"], ",")
	}
	// limit is parsed by the shared non-negative integer parser, so a negative or non-integer
	// value is refused here and the range is applied in the service. That is the same division
	// every other limit on this API uses, and it is what keeps the ceiling a property of the
	// contract rather than of this handler.
	limit, ok := intParam(w, r, s.logger, query.Get("limit"), "limit")
	if !ok {
		return
	}

	result, err := s.knowledge.RelatedKnowledgeFor(entityType, id, service.RelatedQuery{
		Limit:       limit,
		EntityTypes: entityTypes,
	})
	if err != nil {
		s.writeRelatedError(w, r, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, result)
}

// relatedRoute extracts and bounds the two route values.
//
// The starting class is checked against the closed searchable set before the ID is even looked
// at, so an unsupported class is a 400 rather than a lookup that happens to miss. The ID goes
// through the same pathID bounds every other AudioMuse route uses: it is a map key and is never
// joined to a path, so a discovery identifier cannot become a file read. A percent-encoded
// separator is decoded by the router before the handler sees it, so an encoded traversal attempt
// arrives here as the separator it decodes to and is refused by the same check.
//
// It is a separate function from entityRoute rather than a parameter on it, because the two
// validate different closed sets - four graph classes there, six searchable classes here - and
// sharing one function would mean one of the two routes reported a set it does not accept.
func relatedRoute(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (string, string, bool) {
	entityType := r.PathValue("entity_type")
	if entityType == "" || len(entityType) > maxIDLength || !domain.ValidSearchEntityType(entityType) {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Unsupported related-knowledge entity type. Supported: "+
				strings.Join(domain.SearchEntityTypeNames(), ", ")+".")
		return "", "", false
	}
	id, ok := pathID(w, r, logger)
	if !ok {
		return "", "", false
	}
	return entityType, id, true
}

// writeRelatedError maps the service's typed errors onto the stable envelope.
//
// The mapping is by error type, never by matching on message prose. A record that exists and is
// related to nothing never reaches here: it is a 200 carrying an empty item list, because "this
// record references nothing and nothing references it" is an answer. Only a malformed request
// and an unresolvable start land here.
func (s *Server) writeRelatedError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrUnsupportedRelatedEntityType):
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Unsupported related-knowledge entity type. Supported: "+
				strings.Join(domain.SearchEntityTypeNames(), ", ")+".")
	case errors.Is(err, service.ErrNotFound):
		// A distinct code from entity_not_found, which names a graph vertex. A discovery start
		// may be a vocabulary entry or an experiment definition, neither of which is a graph
		// entity, so reusing that code would tell a client the lookup failed in a layer it never
		// asked about.
		writeError(w, r, s.logger, http.StatusNotFound, CodeRelatedStartNotFound,
			"Related-knowledge start record was not found.")
	default:
		// The scope errors are the Phase 1H ones, rendered by the shared helper so the two
		// routes cannot drift into refusing one malformed class list with two different
		// messages. Anything else is an InvalidFilterError or an unexpected error and is handled
		// by the Phase 1B mapping, which lists the permitted values and never echoes the
		// caller's own.
		if writeScopeFilterError(w, r, s.logger, err) {
			return
		}
		writeFilterError(w, r, s.logger, err)
	}
}
