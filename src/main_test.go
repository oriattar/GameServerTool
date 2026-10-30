package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gstBinary is the path to the gst binary, built once for all CLI tests in commands_test.go
var gstBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gst-test")
	if err != nil {
		panic(err)
	}
	gstBinary = filepath.Join(dir, "gst")

	build := exec.Command("go", "build", "-o", gstBinary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("building gst failed: " + err.Error() + "\n" + string(out))
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestValidMemory(t *testing.T) {
	tests := []struct {
		memory string
		want   bool
	}{
		{"1G", true},
		{"4G", true},
		{"32G", true},
		{"0G", false},
		{"33G", false},
		{"-2G", false},
		{"4", false},
		{"4g", false},
		{"4GB", false},
		{"G", false},
		{"", false},
		{"1.5G", false},
		{"abc", false},
	}

	for _, tt := range tests {
		if got := validMemory(tt.memory); got != tt.want {
			t.Errorf("validMemory(%q) = %v, want %v", tt.memory, got, tt.want)
		}
	}
}

func TestHelpTextListsAllCommands(t *testing.T) {
	for _, cmd := range []string{"create", "start", "stop", "delete", "list", "help", "-eula", "-port", "-memory", "-delvolume"} {
		if !strings.Contains(helpText, cmd) {
			t.Errorf("helpText does not mention %q", cmd)
		}
	}
}
