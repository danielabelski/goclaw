package skills

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPipRejectsBSPFlag(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"exact error", "no such option: --break-system-packages", true},
		{"error with context", "Usage: pip3 install\n\nno such option: --break-system-packages", true},
		{"unrelated error", "ERROR: Could not find a version that satisfies the requirement", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pipRejectsBSPFlag([]byte(c.out)); got != c.want {
				t.Errorf("pipRejectsBSPFlag(%q) = %v, want %v", c.out, got, c.want)
			}
		})
	}
}

func TestDropPipBSPFlag(t *testing.T) {
	in := []string{"install", "--upgrade", "--no-cache-dir", "--break-system-packages", "--upgrade-strategy", "only-if-needed", "--pre", "pkg"}
	got := dropPipBSPFlag(in)
	want := []string{"install", "--upgrade", "--no-cache-dir", "--upgrade-strategy", "only-if-needed", "--pre", "pkg"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("dropPipBSPFlag() = %v, want %v", got, want)
	}
}

// writeLegacyPipScript writes a fake `pip3` that behaves like pip < 23.0: it
// rejects --break-system-packages with the exact error from issue #956 and
// succeeds without it. Every successful non-flag invocation appends its args
// to argsFile so tests can assert the flag was not passed on the retry.
func writeLegacyPipScript(t *testing.T, argsFile string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pip3")
	body := "#!/bin/sh\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$a\" = \"--break-system-packages\" ]; then\n" +
		"    echo \"no such option: --break-system-packages\" >&2\n" +
		"    exit 2\n" +
		"  fi\n" +
		"done\n" +
		"case \"$1\" in\n" +
		"  install|cache) echo \"$@\" >> \"" + argsFile + "\"; exit 0 ;;\n" +
		"esac\n" +
		"exit 0\n"
	if runtime.GOOS == "windows" {
		path += ".cmd"
		body = "@echo off\r\n" +
			"setlocal\r\n" +
			"for %%A in (%*) do (\r\n" +
			"  if \"%%~A\"==\"--break-system-packages\" (\r\n" +
			"    echo no such option: --break-system-packages 1>&2\r\n" +
			"    exit /b 2\r\n" +
			"  )\r\n" +
			")\r\n" +
			"if \"%~1\"==\"install\" (\r\n" +
			"  echo %* >> \"" + argsFile + "\"\r\n" +
			"  exit /b 0\r\n" +
			")\r\n" +
			"if \"%~1\"==\"cache\" (\r\n" +
			"  echo %* >> \"" + argsFile + "\"\r\n" +
			"  exit /b 0\r\n" +
			")\r\n" +
			"exit /b 0\r\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write legacy pip script: %v", err)
	}
	return path
}

// setupLegacyPip points pipBinary/pipLookPath at a legacy pip script that
// rejects --break-system-packages.
func setupLegacyPip(t *testing.T, argsFile string) {
	t.Helper()
	scriptPath := writeLegacyPipScript(t, argsFile)
	origBinary := pipBinary
	origLookPath := pipLookPath
	pipBinary = scriptPath
	pipLookPath = func(string) (string, error) { return scriptPath, nil }
	t.Cleanup(func() {
		pipBinary = origBinary
		pipLookPath = origLookPath
	})
}

// TestInstallSingleDepPip_LegacyPipNoFlag reproduces issue #956: on a pip that
// rejects --break-system-packages, installing a pip dep must still succeed via
// the automatic retry without the flag.
func TestInstallSingleDepPip_LegacyPipNoFlag(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "captured-args.txt")
	setupLegacyPip(t, argsFile)

	ok, msg := InstallSingleDep(context.Background(), "pip:requests")
	if !ok {
		t.Fatalf("InstallSingleDep failed on legacy pip: %s", msg)
	}

	captured, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("args file not written: %v", err)
	}
	if strings.Contains(string(captured), "--break-system-packages") {
		t.Fatalf("legacy pip received --break-system-packages: %q", string(captured))
	}
}

// TestInstallDepsPip_LegacyPipNoFlag covers the batch path (InstallDeps) with
// the same legacy pip fixture.
func TestInstallDepsPip_LegacyPipNoFlag(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "captured-args.txt")
	setupLegacyPip(t, argsFile)

	res, err := InstallDeps(context.Background(), &SkillManifest{}, []string{"pip:requests", "pip:numpy"})
	if err != nil {
		t.Fatalf("InstallDeps returned error: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("InstallDeps errors = %v, want none", res.Errors)
	}
	if len(res.Pip) != 2 {
		t.Fatalf("InstallDeps installed = %v, want 2 packages", res.Pip)
	}

	captured, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("args file not written: %v", err)
	}
	if strings.Contains(string(captured), "--break-system-packages") {
		t.Fatalf("legacy pip received --break-system-packages: %q", string(captured))
	}
}

// TestPipUpdateExecutor_LegacyPipNoFlag verifies the upgrade path also retries
// without the flag on legacy pip.
func TestPipUpdateExecutor_LegacyPipNoFlag(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "captured-args.txt")
	setupLegacyPip(t, argsFile)

	e := NewPipUpdateExecutor()
	if err := e.Update(context.Background(), "requests", "2.31.0", nil); err != nil {
		t.Fatalf("Update failed on legacy pip: %v", err)
	}

	captured, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("args file not written: %v", err)
	}
	if strings.Contains(string(captured), "--break-system-packages") {
		t.Fatalf("legacy pip received --break-system-packages: %q", string(captured))
	}
}
