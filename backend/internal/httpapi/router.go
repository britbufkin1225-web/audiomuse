package httpapi

import (
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// APIBase is the versioned API prefix.
const APIBase = "/api/v1"

// allowedMethods is the complete set of methods this service accepts, anywhere.
//
// HEAD is included because net/http serves it from the GET handler with the body
// discarded, which is the correct behaviour for a read-only API.
var allowedMethods = map[string]bool{
	http.MethodGet:  true,
	http.MethodHead: true,
}

// allowHeader is the value returned alongside every 405.
const allowHeader = "GET, HEAD"

// Server holds the handler dependencies.
type Server struct {
	knowledge *service.Knowledge
	logger    *slog.Logger
}

// NewServer builds the fully wired read-only HTTP handler.
func NewServer(knowledge *service.Knowledge, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{knowledge: knowledge, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET "+APIBase+"/project", s.handleProject)
	mux.HandleFunc("GET "+APIBase+"/nodes", s.handleNodes)
	mux.HandleFunc("GET "+APIBase+"/nodes/{id}", s.handleNodeByID)
	mux.HandleFunc("GET "+APIBase+"/sessions", s.handleSessions)
	mux.HandleFunc("GET "+APIBase+"/sessions/{id}", s.handleSessionByID)
	mux.HandleFunc("GET "+APIBase+"/sources", s.handleSources)
	mux.HandleFunc("GET "+APIBase+"/sources/{id}", s.handleSourceByID)
	mux.HandleFunc("GET "+APIBase+"/claims", s.handleClaims)
	mux.HandleFunc("GET "+APIBase+"/claims/{id}", s.handleClaimByID)
	mux.HandleFunc("GET "+APIBase+"/vocabulary", s.handleVocabulary)
	mux.HandleFunc("GET "+APIBase+"/vocabulary/{id}", s.handleVocabularyByID)
	mux.HandleFunc("GET "+APIBase+"/experiments", s.handleExperiments)
	mux.HandleFunc("GET "+APIBase+"/experiments/{id}", s.handleExperimentByID)
	mux.HandleFunc("GET "+APIBase+"/experiment-runs", s.handleExperimentRuns)
	mux.HandleFunc("GET "+APIBase+"/experiment-runs/{id}", s.handleExperimentRunByID)
	mux.HandleFunc("GET "+APIBase+"/search", s.handleSearch)
	mux.HandleFunc("GET "+APIBase+"/related/{entity_type}/{id}", s.handleRelatedKnowledge)
	mux.HandleFunc("GET "+APIBase+"/graph", s.handleGraph)
	mux.HandleFunc("GET "+APIBase+"/graph/entities/{entity_type}/{id}/relationships", s.handleEntityRelationships)
	mux.HandleFunc("GET "+APIBase+"/graph/entities/{entity_type}/{id}/traverse", s.handleEntityTraverse)
	mux.HandleFunc("GET "+APIBase+"/diagnostics", s.handleDiagnostics)
	mux.HandleFunc("/", s.handleNotFound)

	// Order matters: the method lock runs outermost so a mutating request is refused
	// before any route is consulted. A write route added by accident would be unreachable.
	return s.methodLock(s.requestLog(mux))
}

// methodLock enforces the read-only contract at the edge of the service.
//
// Read-only is asserted here rather than merely implied by the absence of write handlers,
// which makes the guarantee explicit, test-coverable, and independent of the routing table.
func (s *Server) methodLock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedMethods[r.Method] {
			w.Header().Set("Allow", allowHeader)
			writeError(w, r, s.logger, http.StatusMethodNotAllowed, CodeMethodNotAllow,
				"The AudioMuse knowledge API is read-only. Only GET and HEAD are supported.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the status code for request logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// requestLog records method, route, status and duration.
//
// Only the request path is logged, never a response body or a filesystem path, so the log
// stays useful without accumulating corpus content or local layout.
func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// handleNotFound answers any unrouted path with the stable error envelope, so a client
// never receives the net/http plain-text default from this service.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, s.logger, http.StatusNotFound, CodeNotFound, "No such endpoint.")
}

// knownParams bounds each endpoint's accepted query string.
//
// An unrecognised parameter is refused rather than ignored: silently dropping a filter the
// caller believed was applied would return a result set that does not mean what they think
// it means, which for a knowledge projection is worse than an error.
//
// The query string is parsed here rather than read from r.URL.Query(), and the difference is the
// whole point of the first check below. r.URL.Query() discards url.ParseQuery's error and returns
// whatever pairs it managed to read, so a query string carrying an invalid percent-escape or a
// semicolon separator arrives at a handler with the malformed pairs simply absent - and an absent
// filter is an unfiltered request. A caller who wrote entity_types=%zz would have received the
// complete unrestricted result set under a 200, with no echo of the filter they believed they had
// applied and nothing in the response to mark that anything had been dropped. That is precisely
// the failure this function exists to prevent, arriving one layer earlier than the check that was
// written for it. A query string Go cannot parse is therefore refused whole, before any parameter
// is looked at: refusing part of it would leave the same silent drop in place for the rest.
//
// Handlers still call r.URL.Query() afterwards, and that stays correct because this guard runs
// first on every route: past it, the raw string has parsed cleanly, so the two readings of it are
// the same map rather than two possibly different ones.
//
// Parameter names are then checked in sorted order rather than in Go map order. A request
// carrying more than one violation - two unsupported names, or an unsupported name beside a
// repeated one - was refused either way, but which rule the message cited was decided by map
// iteration, so one unchanged request produced two different bodies across runs. This API's
// contract is that an identical request returns an identical response, and an error body is part
// of the response. Sorting decides it by the parameter name instead, which is a property of the
// request. A request with exactly one violation is unaffected: it reported that violation before
// and reports the same one now.
func rejectUnknownParams(w http.ResponseWriter, r *http.Request, logger *slog.Logger, known ...string) bool {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		// The message states the fault and never echoes the string that caused it, following
		// every other refusal on this API: the caller already holds their own query string, and
		// it is the one part of a response an attacker would control.
		writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
			"The query string could not be parsed. Supported: "+strings.Join(known, ", ")+".")
		return false
	}
	allowed := make(map[string]bool, len(known))
	for _, name := range known {
		allowed[name] = true
	}
	names := make([]string, 0, len(query))
	for name := range query {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !allowed[name] {
			writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
				"Unsupported query parameter: "+sanitizeParamName(name)+". Supported: "+strings.Join(known, ", ")+".")
			return false
		}
		if len(query[name]) != 1 {
			writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
				"Query parameter "+sanitizeParamName(name)+" must be supplied exactly once.")
			return false
		}
	}
	return true
}

// sanitizeParamName bounds and strips a caller-supplied name before it is echoed back, so
// a hostile query string cannot inject control characters into the response or the log.
func sanitizeParamName(name string) string {
	const max = 32
	var b strings.Builder
	for _, r := range name {
		if b.Len() >= max {
			b.WriteString("...")
			break
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "(unnamed)"
	}
	return b.String()
}
