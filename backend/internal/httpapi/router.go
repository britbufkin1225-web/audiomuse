package httpapi

import (
	"log/slog"
	"net/http"
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
	mux.HandleFunc("GET "+APIBase+"/graph", s.handleGraph)
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
func rejectUnknownParams(w http.ResponseWriter, r *http.Request, logger *slog.Logger, known ...string) bool {
	allowed := make(map[string]bool, len(known))
	for _, name := range known {
		allowed[name] = true
	}
	for name := range r.URL.Query() {
		if !allowed[name] {
			writeError(w, r, logger, http.StatusBadRequest, CodeInvalidQuery,
				"Unsupported query parameter: "+sanitizeParamName(name)+". Supported: "+strings.Join(known, ", ")+".")
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
