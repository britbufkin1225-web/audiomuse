package config_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/config"
)

// fakeRepo builds a directory carrying the canonical root markers.
func fakeRepo(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{
		"nodes/.keep",
		"schemas/node.schema.yaml",
		"schemas/relationship-types.yaml",
		"sources/source-registry.yaml",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("# marker\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return root
}

func noEnv(string) string { return "" }

func TestDefaults(t *testing.T) {
	root := fakeRepo(t)
	cfg, err := config.Load([]string{"-repo-root", root}, noEnv, io.Discard)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got, want := cfg.Addr, config.DefaultAddr; got != want {
		t.Errorf("addr = %q, want %q", got, want)
	}
	if !strings.HasPrefix(cfg.Addr, "127.0.0.1:") {
		t.Errorf("default addr %q does not bind loopback", cfg.Addr)
	}
	if cfg.RepoRoot != root {
		t.Errorf("repo root = %q, want %q", cfg.RepoRoot, root)
	}
	if !strings.Contains(cfg.RepoRootSource, "flag") {
		t.Errorf("repo root source = %q, want the flag", cfg.RepoRootSource)
	}
}

func TestEnvironmentIsUsedWhenNoFlagIsGiven(t *testing.T) {
	root := fakeRepo(t)
	env := func(key string) string {
		switch key {
		case config.EnvRepoRoot:
			return root
		case config.EnvAddr:
			return "127.0.0.1:9999"
		}
		return ""
	}
	cfg, err := config.Load(nil, env, io.Discard)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RepoRoot != root {
		t.Errorf("repo root = %q, want %q", cfg.RepoRoot, root)
	}
	if got, want := cfg.Addr, "127.0.0.1:9999"; got != want {
		t.Errorf("addr = %q, want %q", got, want)
	}
	if !strings.Contains(cfg.RepoRootSource, config.EnvRepoRoot) {
		t.Errorf("repo root source = %q, want the environment", cfg.RepoRootSource)
	}
}

func TestFlagOverridesEnvironment(t *testing.T) {
	flagRoot := fakeRepo(t)
	envRoot := fakeRepo(t)
	env := func(key string) string {
		if key == config.EnvRepoRoot {
			return envRoot
		}
		if key == config.EnvAddr {
			return "127.0.0.1:1111"
		}
		return ""
	}
	cfg, err := config.Load([]string{"-repo-root", flagRoot, "-addr", "127.0.0.1:2222"}, env, io.Discard)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RepoRoot != flagRoot {
		t.Errorf("repo root = %q, want the flag value %q", cfg.RepoRoot, flagRoot)
	}
	if got, want := cfg.Addr, "127.0.0.1:2222"; got != want {
		t.Errorf("addr = %q, want %q", got, want)
	}
}

// TestWindowsStylePathsAreNormalised covers the operator environment: a quoted path with a
// trailing separator and mixed separators must still resolve.
func TestWindowsStylePathsAreNormalised(t *testing.T) {
	root := fakeRepo(t)
	for _, variant := range []string{
		root + string(filepath.Separator),
		`"` + root + `"`,
		"  " + root + "  ",
		filepath.Join(root, "nodes", ".."),
	} {
		cfg, err := config.Load([]string{"-repo-root", variant}, noEnv, io.Discard)
		if err != nil {
			t.Fatalf("load(%q): %v", variant, err)
		}
		if cfg.RepoRoot != root {
			t.Errorf("load(%q) resolved to %q, want %q", variant, cfg.RepoRoot, root)
		}
	}
}

func TestInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  func(string) string
	}{
		{"empty repo root", []string{"-repo-root", "   "}, noEnv},
		{"missing directory", []string{"-repo-root", filepath.Join(t.TempDir(), "nope")}, noEnv},
		{"not an audiomuse repository", []string{"-repo-root", t.TempDir()}, noEnv},
		{"address without a port", []string{"-repo-root", fakeRepo(t), "-addr", "localhost"}, noEnv},
		{"unexpected positional argument", []string{"-repo-root", fakeRepo(t), "extra"}, noEnv},
		{"unknown flag", []string{"-nope"}, noEnv},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.Load(tc.args, tc.env, io.Discard); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

// TestRepoRootIsAFileNotADirectory guards a plausible operator mistake.
func TestRepoRootIsAFileNotADirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "audiomuse.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := config.Load([]string{"-repo-root", file}, noEnv, io.Discard); err == nil {
		t.Fatal("want an error when the repository root is a file")
	}
}

// TestDiscoveryFindsTheRootFromASubdirectory covers launching from backend/ rather than
// from the repository root.
func TestDiscoveryFindsTheRootFromASubdirectory(t *testing.T) {
	root := fakeRepo(t)
	sub := filepath.Join(root, "backend", "cmd", "audiomuse-api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(sub)

	cfg, err := config.Load(nil, noEnv, io.Discard)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// t.TempDir may sit behind a symlink on some platforms; compare resolved paths.
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		wantRoot = root
	}
	gotRoot, err := filepath.EvalSymlinks(cfg.RepoRoot)
	if err != nil {
		gotRoot = cfg.RepoRoot
	}
	if gotRoot != wantRoot {
		t.Errorf("discovered %q, want %q", gotRoot, wantRoot)
	}
	if !strings.Contains(cfg.RepoRootSource, "discovered") {
		t.Errorf("repo root source = %q, want discovery", cfg.RepoRootSource)
	}
}

func TestDiscoveryFailsOutsideARepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := config.Load(nil, noEnv, io.Discard); err == nil {
		t.Fatal("want an error when no repository root can be discovered")
	}
}
