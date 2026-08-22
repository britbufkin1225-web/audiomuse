package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The Phase 1C traversal handlers. They stay as thin as every other handler in this
// package: bound the route values, bound the query string, hand it to the immutable index,
// map the typed error, serialise. No adjacency lookup, no filtering and no breadth-first
// search happens here — that logic belongs to the service and is tested there directly.

// traversalParams and relationshipParams are the complete accepted query strings.
var (
	relationshipParams = []string{"relationship", "target_type"}
	traversalParams    = []string{"depth", "relationship", "target_type"}
)

func (s *Server) handleEntityRelationships(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, relationshipParams...) {
		return
	}
	entityType, id, ok := entityRoute(w, r, s.logger)
	if !ok {
		return
	}
	values, ok := boundedParams(w, r, s.logger, r.URL.Query(), "relationship", "target_type")
	if !ok {
		return
	}

	result, err := s.knowledge.EntityRelationshipsFor(entityType, id, service.TraversalQuery{
		Relationship: values["relationship"],
		TargetType:   values["target_type"],
	})
	if err != nil {
		s.writeTraversalError(w, r, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, result)
}

func (s *Server) handleEntityTraverse(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownParams(w, r, s.logger, traversalParams...) {
		return
	}
	entityType, id, ok := entityRoute(w, r, s.logger)
	if !ok {
		return
	}
	query := r.URL.Query()

	// Depth is parsed here and bounded in the service. The HTTP layer owns the "is this
	// even an integer, and was it supplied at all" question; the service owns the range, so
	// a future non-HTTP caller inherits the ceiling rather than depending on this handler.
	depth, ok := depthParam(w, r, s.logger, query)
	if !ok {
		return
	}
	values, ok := boundedParams(w, r, s.logger, query, "relationship", "target_type")
	if !ok {
		return
	}

	result, err := s.knowledge.Traverse(entityType, id, service.TraversalQuery{
		Depth:        depth,
		Relationship: values["relationship"],
		TargetType:   values["target_type"],
	})
	if err != nil {
		s.writeTraversalError(w, r, err)
		return
	}
	writeJSON(w, r, s.logger, http.StatusOK, result)
}

// depthParam parses the optional depth control.
//
// It exists rather than reusing intParam because the shared helper reads an absent
// parameter as zero, and depth has to tell an absent parameter from an explicit depth=0.
// Absent means "use the default"; an explicit zero is a request for a traversal that
// returns only its own root, which answers nothing and is refused rather than quietly
// promoted to the default.
func depthParam(w http.ResponseWriter, r *http.Request, logger *slog.Logger, query url.Values) (int, bool) {
	raw, present := query["depth"]
	if !present {
		return 0, true
	}
	trimmed := strings.TrimSpace(raw[0])
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < service.MinTraversalDepth || parsed > service.MaxTraversalDepth {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Parameter depth must be an integer between "+strconv.Itoa(service.MinTraversalDepth)+
				" and "+strconv.Itoa(service.MaxTraversalDepth)+".")
		return 0, false
	}
	return parsed, true
}

// entityRoute extracts and bounds the two route values.
//
// The entity type is checked against the closed model set before the ID is even looked at,
// so an unsupported class is a 400 rather than a lookup that happens to miss. The ID goes
// through the same pathID bounds every other AudioMuse route uses: it is a map key and is
// never joined to a path, so a traversal identifier cannot become a file read.
func entityRoute(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (string, string, bool) {
	entityType := r.PathValue("entity_type")
	if entityType == "" || len(entityType) > maxIDLength || !domain.ValidEntityType(entityType) {
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"Unsupported graph entity type. Supported: "+strings.Join(domain.EntityTypeNames(), ", ")+".")
		return "", "", false
	}
	id, ok := pathID(w, r, logger)
	if !ok {
		return "", "", false
	}
	return entityType, id, true
}

// writeTraversalError maps the service's typed errors onto the stable envelope.
//
// The mapping is by error type, never by matching on message prose. An entity class outside
// the model and a filter value outside a vocabulary are both caller mistakes and answer
// 400; an ID that resolves to no record answers 404, which is what keeps "this entity does
// not exist" distinguishable from "this entity exists and has no relationships".
func (s *Server) writeTraversalError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrUnsupportedEntityType):
		writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
			"Unsupported graph entity type. Supported: "+strings.Join(domain.EntityTypeNames(), ", ")+".")
	case errors.Is(err, service.ErrNotFound):
		writeError(w, r, s.logger, http.StatusNotFound, CodeEntityNotFound,
			"Graph entity was not found.")
	default:
		var depthErr *service.InvalidDepthError
		if errors.As(err, &depthErr) {
			writeError(w, r, s.logger, http.StatusBadRequest, CodeInvalidQuery,
				"Parameter depth must be between "+strconv.Itoa(depthErr.Min)+" and "+
					strconv.Itoa(depthErr.Max)+".")
			return
		}
		// Filter errors and anything unexpected share the Phase 1B mapping, which lists the
		// permitted values from the canonical contract and never echoes the caller's own.
		writeFilterError(w, r, s.logger, err)
	}
}
