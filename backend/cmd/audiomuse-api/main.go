// Command audiomuse-api serves a deterministic read-only HTTP projection of the canonical
// AudioMuse repository.
//
// The repository remains authoritative. This process reads it once at startup, validates
// what it read, and never writes to it.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/config"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/httpapi"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stderr); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		fmt.Fprintf(os.Stderr, "audiomuse-api: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stderr *os.File) error {
	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(args, getenv, stderr)
	if err != nil {
		return err
	}

	repo, err := filesystem.New(cfg.RepoRoot)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	knowledge, err := service.New(ctx, repo)
	if err != nil {
		return err
	}
	report := knowledge.Report()
	project := knowledge.Project()

	// Startup diagnostics. The absolute repository root appears here, in the operator's own
	// terminal, and nowhere in any HTTP response.
	logger.Info("AudioMuse API",
		"mode", service.ModeReadOnly,
		"repository", cfg.RepoRoot,
		"repository_source", cfg.RepoRootSource,
		"nodes", project.Counts.Nodes,
		"sessions", project.Counts.Sessions,
		"sources", project.Counts.Sources,
		"edges", project.Counts.Edges,
		"validation", report.Status(),
		"warnings", len(report.Warnings()),
		"listen", cfg.Addr,
	)
	for _, warning := range report.Warnings() {
		logger.Warn("validation", "code", warning.Code, "ref", warning.Ref, "path", warning.Path, "message", warning.Message)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewServer(knowledge, logger),
		ReadTimeout:       config.ReadTimeout,
		ReadHeaderTimeout: config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	}
}
