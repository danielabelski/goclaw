package skills

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// PipUpdateExecutor implements UpdateExecutor for the "pip" source.
// It upgrades a single package via `pip3 install --upgrade ...`.
// Thread-safe: no mutable state; concurrent package serialization is handled
// upstream by PackageLocker (injected via UpdateRegistry.Apply).
type PipUpdateExecutor struct{}

// NewPipUpdateExecutor returns a PipUpdateExecutor ready for use.
func NewPipUpdateExecutor() *PipUpdateExecutor { return &PipUpdateExecutor{} }

// Source returns "pip".
func (e *PipUpdateExecutor) Source() string { return "pip" }

// Update upgrades `name` to `toVersion` using pip3.
//
// Argument ordering matches UpdateExecutor interface: (ctx, name, toVersion, meta).
// `name` is validated via ValidatePipPackageName before any exec.
// `--pre` is appended when meta["preRelease"]==true OR IsPipPreRelease(toVersion).
// On success, cleanCaches is called for symmetry with dep_installer.go.
// On failure, stderr is classified via ClassifyPipStderr and a wrapped sentinel is returned.
func (e *PipUpdateExecutor) Update(ctx context.Context, name, toVersion string, meta map[string]any) error {
	if err := ValidatePipPackageName(name); err != nil {
		return err
	}

	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{"install", "--upgrade",
		"--no-cache-dir", pipBreakSystemPackagesFlag,
		"--upgrade-strategy", "only-if-needed",
	}

	// Determine whether pre-release flag is needed.
	preRelease := false
	if meta != nil {
		if v, ok := meta["preRelease"].(bool); ok && v {
			preRelease = true
		}
	}
	if !preRelease && IsPipPreRelease(toVersion) {
		preRelease = true
	}
	if preRelease {
		args = append(args, "--pre")
	}
	args = append(args, name)

	start := time.Now()
	stderr, runErr := runPipUpdateCommand(cctx, args)
	// If pip rejects the PEP 668 flag (pip < 23.0, issue #956), retry once
	// without it so updates keep working on legacy environments.
	if runErr != nil && pipRejectsBSPFlag([]byte(stderr)) {
		stderr, runErr = runPipUpdateCommand(cctx, dropPipBSPFlag(args))
	}
	durationMs := time.Since(start).Milliseconds()

	if runErr != nil {
		sentinel, reason := ClassifyPipStderr(stderr)
		if sentinel == nil {
			sentinel = fmt.Errorf("pip install failed: %w", runErr)
		}
		slog.Warn("package.update.pip.outcome",
			"name", name,
			"status", "failed",
			"err_class", fmt.Sprintf("%T:%v", sentinel, sentinel),
			"reason", reason,
			"duration_ms", durationMs)
		return fmt.Errorf("%w: %s", sentinel, reason)
	}

	// Success path: purge caches for disk symmetry with dep_installer.go.
	cleanCaches(cctx)

	slog.Info("package.update.pip.outcome",
		"name", name,
		"to", toVersion,
		"status", "success",
		"duration_ms", durationMs)
	return nil
}

// runPipUpdateCommand runs `pip3 <args...>` and returns stderr only (it feeds
// ClassifyPipStderr); stdout is captured and discarded. Command-level
// WaitDelay mirrors the previous inline exec.
func runPipUpdateCommand(ctx context.Context, args []string) (stderr string, runErr error) {
	cmd := exec.CommandContext(ctx, pipBinary, args...)
	cmd.WaitDelay = 2 * time.Second
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr = cmd.Run()
	return errBuf.String(), runErr
}
