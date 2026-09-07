package skills

import (
	"bytes"
	"context"
	"os/exec"
)

// pipBreakSystemPackagesFlag is the PEP 668 opt-out flag accepted by pip >= 23.0.
// Older pip builds (and the `list` subcommand on every pip) reject it with
// "no such option: --break-system-packages", which breaks skill dependency
// installs on macOS Command Line Tools Python and other legacy environments.
const pipBreakSystemPackagesFlag = "--break-system-packages"

// pipRejectsBSPFlag reports whether pip's combined output indicates the
// --break-system-packages flag was rejected as an unknown option. pip < 23.0
// predates PEP 668 and emits exactly this error (see issue #956).
func pipRejectsBSPFlag(combinedOut []byte) bool {
	return bytes.Contains(combinedOut, []byte("no such option: "+pipBreakSystemPackagesFlag))
}

// dropPipBSPFlag returns args with every --break-system-packages occurrence
// removed, preserving order of the remaining tokens. Used to retry an install
// on pip builds that do not support the flag.
func dropPipBSPFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == pipBreakSystemPackagesFlag {
			continue
		}
		out = append(out, a)
	}
	return out
}

// pipRunInstall runs `pip3 <args...>` where args includes the PEP 668
// --break-system-packages flag. If pip rejects the flag as an unknown option
// (pip < 23.0, issue #956), the command is retried once without it. Returns
// the combined output of the final attempt. Success-path overhead is zero
// extra subprocesses.
func pipRunInstall(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, pipBinary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil && pipRejectsBSPFlag(out) {
		cmd = exec.CommandContext(ctx, pipBinary, dropPipBSPFlag(args)...)
		out, err = cmd.CombinedOutput()
	}
	return out, err
}
