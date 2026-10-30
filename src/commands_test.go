package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// runGst runs the gst binary with the given arguments and returns its output and exit code
func runGst(t *testing.T, args ...string) (string, int) {
	t.Helper()
	out, err := exec.Command(gstBinary, args...).CombinedOutput()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatalf("running gst %v: %v", args, err)
	}
	return string(out), 0
}

// These cases all fail argument validation before any Docker call, so they run without a Docker daemon
func TestCommandArgumentValidation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{"no command", nil, 1, "Usage: gst <command>"},
		{"help", []string{"help"}, 0, "Usage: gst <command>"},

		{"start without name", []string{"start"}, 1, "Usage: gst start <serverName>"},
		{"start with extra args", []string{"start", "a", "b"}, 1, "Usage: gst start <serverName>"},
		{"stop without name", []string{"stop"}, 1, "Usage: gst stop <serverName>"},
		{"stop with extra args", []string{"stop", "a", "b"}, 1, "Usage: gst stop <serverName>"},

		{"create without name", []string{"create", "-eula"}, 1, "Usage: gst create"},
		{"create flag after name", []string{"create", "-eula", "survival", "-port", "25570"}, 1, "Usage: gst create"},
		{"create without eula", []string{"create", "survival"}, 1, "EULA not accepted"},
		{"create memory too high", []string{"create", "-eula", "-memory", "64G", "survival"}, 1, "Invalid memory value"},
		{"create memory bad format", []string{"create", "-eula", "-memory", "4GB", "survival"}, 1, "Invalid memory value"},
		{"create port not a number", []string{"create", "-eula", "-port", "abc", "survival"}, 2, "invalid value"},
		{"create unknown flag", []string{"create", "-eula", "-foo", "survival"}, 2, "flag provided but not defined"},

		{"delete without name", []string{"delete"}, 1, "Usage: gst delete"},
		{"delete bool with separate value", []string{"delete", "-delvolume", "false", "survival"}, 1, "Usage: gst delete"},
		{"delete flag after name", []string{"delete", "survival", "-delvolume"}, 1, "Usage: gst delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := runGst(t, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\noutput:\n%s", code, tt.wantCode, out)
			}
			if !strings.Contains(out, tt.wantOutput) {
				t.Errorf("output does not contain %q\noutput:\n%s", tt.wantOutput, out)
			}
		})
	}
}
