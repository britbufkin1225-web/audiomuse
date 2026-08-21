// Package config resolves how the AudioMuse API is launched.
//
// Resolution is explicit and ordered so that a launch is reproducible and never depends on
// where the process happened to start from without saying so.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Defaults. The listen address binds the loopback interface: AudioMuse is local-first
// software and a knowledge corpus should not be reachable from the network by accident.
const (
	DefaultAddr     = "127.0.0.1:8788"
	EnvRepoRoot     = "AUDIOMUSE_REPO_ROOT"
	EnvAddr         = "AUDIOMUSE_ADDR"
	ReadTimeout     = 10 * time.Second
	WriteTimeout    = 30 * time.Second
	IdleTimeout     = 60 * time.Second
	ShutdownTimeout = 10 * time.Second
	// MaxHeaderBytes bounds request headers well below the net/http default, since no
	// endpoint accepts a body and none needs large headers.
	MaxHeaderBytes = 32 << 10
)

// rootMarkers identify an AudioMuse repository root during upward discovery. They are the
// same canonical paths the filesystem adapter requires, so discovery cannot select a
// directory the adapter will then reject.
var rootMarkers = []string{
	"nodes",
	"schemas/node.schema.yaml",
	"schemas/relationship-types.yaml",
	"sources/source-registry.yaml",
}

// Config is a fully resolved launch configuration.
type Config struct {
	// RepoRoot is an absolute, cleaned path to the canonical repository.
	RepoRoot string
	// RepoRootSource records how RepoRoot was determined, for startup diagnostics.
	RepoRootSource string
	// Addr is the TCP address to listen on.
	Addr string
}

// ErrRepoRootNotFound reports that no repository root was supplied and none could be
// discovered from the working directory.
var ErrRepoRootNotFound = errors.New("no AudioMuse repository root supplied and none found from the working directory")

// Load resolves configuration from command-line flags, then the environment, then upward
// discovery from the working directory.
//
// Precedence is flag, then environment, then discovery. Discovery exists so that running
// the API from inside the repository works without ceremony, and it is bounded: it walks
// only from the working directory to the filesystem root, checking for canonical markers,
// and never inspects an unrelated sibling tree.
func Load(args []string, getenv func(string) string, stderr io.Writer) (Config, error) {
	fs := flag.NewFlagSet("audiomuse-api", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoRoot := fs.String("repo-root", "", "path to the canonical AudioMuse repository (overrides "+EnvRepoRoot+")")
	addr := fs.String("addr", "", "TCP address to listen on (overrides "+EnvAddr+"); default "+DefaultAddr)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected positional argument %q", fs.Arg(0))
	}

	// A flag that was supplied is honoured even if its value is blank, so an explicit
	// -repo-root "" fails loudly instead of silently falling through to discovery and
	// serving whatever repository happens to sit above the working directory.
	supplied := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { supplied[f.Name] = true })

	cfg := Config{Addr: DefaultAddr}

	switch {
	case supplied["addr"]:
		cfg.Addr = strings.TrimSpace(*addr)
	case strings.TrimSpace(getenv(EnvAddr)) != "":
		cfg.Addr = strings.TrimSpace(getenv(EnvAddr))
	}
	if !strings.Contains(cfg.Addr, ":") {
		return Config{}, fmt.Errorf("listen address %q must include a port", cfg.Addr)
	}

	var raw, origin string
	switch {
	case supplied["repo-root"]:
		raw, origin = strings.TrimSpace(*repoRoot), "flag -repo-root"
	case strings.TrimSpace(getenv(EnvRepoRoot)) != "":
		raw, origin = strings.TrimSpace(getenv(EnvRepoRoot)), "environment "+EnvRepoRoot
	default:
		discovered, err := discoverRoot()
		if err != nil {
			return Config{}, err
		}
		raw, origin = discovered, "discovered from working directory"
	}

	resolved, err := resolveRoot(raw)
	if err != nil {
		return Config{}, err
	}
	cfg.RepoRoot = resolved
	cfg.RepoRootSource = origin
	return cfg, nil
}

// resolveRoot normalises a supplied root and confirms it is a readable AudioMuse
// repository. Normalisation matters on Windows, where a caller may pass a path with mixed
// separators, a trailing separator, or a quoted trailing backslash.
func resolveRoot(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, `"`)
	if cleaned == "" {
		return "", fmt.Errorf("repository root is empty")
	}
	abs, err := filepath.Abs(filepath.Clean(cleaned))
	if err != nil {
		return "", fmt.Errorf("resolve repository root %q: %w", raw, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("repository root %s is not readable: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository root %s is not a directory", abs)
	}
	if missing, ok := missingMarker(abs); !ok {
		return "", fmt.Errorf("repository root %s does not look like an AudioMuse repository: missing %s", abs, missing)
	}
	return abs, nil
}

// discoverRoot walks upward from the working directory looking for canonical markers.
func discoverRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine working directory: %w", err)
	}
	for {
		if _, ok := missingMarker(dir); ok {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrRepoRootNotFound
		}
		dir = parent
	}
}

// missingMarker reports the first absent canonical marker, and whether all were present.
func missingMarker(dir string) (string, bool) {
	for _, marker := range rootMarkers {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(marker))); err != nil {
			return marker, false
		}
	}
	return "", true
}
