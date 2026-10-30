package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
)

func TestConstructMinecraftServerData(t *testing.T) {
	data, err := constructMinecraftServerData("survival", 25570, "8G")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Name != "survival" {
		t.Errorf("Name = %q, want %q", data.Name, "survival")
	}
	if data.Image != "gst-minecraft" {
		t.Errorf("Image = %q, want %q", data.Image, "gst-minecraft")
	}
	if data.Labels["gst-managed"] != "true" {
		t.Errorf("container is missing the gst-managed=true label")
	}

	for _, env := range []string{"EULA=TRUE", "MEMORY=8G"} {
		if !slices.Contains(data.Env, env) {
			t.Errorf("Env = %v, missing %q", data.Env, env)
		}
	}

	// Minecraft always listens on 25565 inside the container, the -port flag only changes the host side
	containerPort, err := network.ParsePort("25565/tcp")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := data.ExposedPorts[containerPort]; !ok {
		t.Errorf("ExposedPorts = %v, want container port 25565/tcp", data.ExposedPorts)
	}
	bindings := data.PortBindings[containerPort]
	if len(bindings) != 1 || bindings[0].HostPort != "25570" {
		t.Errorf("PortBindings[25565/tcp] = %v, want host port 25570", bindings)
	}

	if len(data.Mounts) != 1 {
		t.Fatalf("Mounts = %v, want exactly one data volume", data.Mounts)
	}
	m := data.Mounts[0]
	if m.Type != mount.TypeVolume || m.Source != "gst-survival-data" || m.Target != "/data" {
		t.Errorf("Mount = %+v, want volume gst-survival-data at /data", m)
	}
	if m.VolumeOptions == nil || m.VolumeOptions.Labels["gst-managed"] != "true" {
		t.Errorf("data volume is missing the gst-managed=true label")
	}
}

// runEntrypoint runs ../entrypoint.sh in a temp directory with only the given environment variables.
// A fake java on PATH prints its arguments instead of starting Minecraft.
// It returns the output, the exit code and the directory the script ran in.
func runEntrypoint(t *testing.T, env map[string]string, existingProperties string) (string, int, string) {
	t.Helper()

	script, err := filepath.Abs("../entrypoint.sh")
	if err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	fakeJava := "#!/bin/sh\necho \"java $*\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "java"), []byte(fakeJava), 0o755); err != nil {
		t.Fatal(err)
	}

	workDir := t.TempDir()
	if existingProperties != "" {
		if err := os.WriteFile(filepath.Join(workDir, "server.properties"), []byte(existingProperties), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("sh", script)
	cmd.Dir = workDir
	cmd.Env = []string{"PATH=" + binDir + ":" + os.Getenv("PATH")}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), workDir
	}
	if err != nil {
		t.Fatalf("running entrypoint: %v", err)
	}
	return string(out), 0, workDir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEntrypointStartsServer(t *testing.T) {
	out, code, dir := runEntrypoint(t, map[string]string{"EULA": "TRUE", "MEMORY": "4G"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\noutput:\n%s", code, out)
	}

	wantJava := "java -Xms4G -Xmx4G -jar /opt/minecraft/server.jar nogui"
	if !strings.Contains(out, wantJava) {
		t.Errorf("output does not contain %q\noutput:\n%s", wantJava, out)
	}
	if got := readFile(t, filepath.Join(dir, "eula.txt")); got != "eula=true\n" {
		t.Errorf("eula.txt = %q, want %q", got, "eula=true\n")
	}
	if got := readFile(t, filepath.Join(dir, "server.properties")); !strings.Contains(got, "enable-rcon=false\n") {
		t.Errorf("server.properties = %q, want enable-rcon=false when RCON is not set", got)
	}
}

func TestEntrypointRejectsBadInput(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		wantOutput string
	}{
		{"no eula", map[string]string{"MEMORY": "4G"}, "not accepted the Minecraft EULA"},
		{"eula false", map[string]string{"EULA": "false", "MEMORY": "4G"}, "not accepted the Minecraft EULA"},
		{"no memory", map[string]string{"EULA": "TRUE"}, "MEMORY is not set"},
		{"memory too low", map[string]string{"EULA": "TRUE", "MEMORY": "0G"}, "Invalid MEMORY value"},
		{"memory too high", map[string]string{"EULA": "TRUE", "MEMORY": "33G"}, "Invalid MEMORY value"},
		{"memory without G", map[string]string{"EULA": "TRUE", "MEMORY": "4"}, "Invalid MEMORY value"},
		{"memory not a number", map[string]string{"EULA": "TRUE", "MEMORY": "abcG"}, "Invalid MEMORY value"},
		{"rcon without password", map[string]string{"EULA": "TRUE", "MEMORY": "4G", "RCON": "true"}, "RCON_PASSWORD is not set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code, _ := runEntrypoint(t, tt.env, "")
			if code != 1 {
				t.Errorf("exit code = %d, want 1\noutput:\n%s", code, out)
			}
			if !strings.Contains(out, tt.wantOutput) {
				t.Errorf("output does not contain %q\noutput:\n%s", tt.wantOutput, out)
			}
			if strings.Contains(out, "java ") {
				t.Errorf("java was started despite invalid input\noutput:\n%s", out)
			}
		})
	}
}

func TestEntrypointEnablesRcon(t *testing.T) {
	env := map[string]string{"EULA": "TRUE", "MEMORY": "4G", "RCON": "true", "RCON_PASSWORD": "p@ss|&x", "RCON_PORT": "25580"}
	out, code, dir := runEntrypoint(t, env, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\noutput:\n%s", code, out)
	}

	props := readFile(t, filepath.Join(dir, "server.properties"))
	for _, line := range []string{"enable-rcon=true", "rcon.password=p@ss|&x", "rcon.port=25580"} {
		if !strings.Contains(props, line+"\n") {
			t.Errorf("server.properties does not contain %q\n%s", line, props)
		}
	}
}

func TestEntrypointKeepsExistingProperties(t *testing.T) {
	existing := "motd=hello\nenable-rcon=true\nmax-players=10\n"
	out, code, dir := runEntrypoint(t, map[string]string{"EULA": "TRUE", "MEMORY": "4G"}, existing)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\noutput:\n%s", code, out)
	}

	props := readFile(t, filepath.Join(dir, "server.properties"))
	for _, line := range []string{"motd=hello", "max-players=10", "enable-rcon=false"} {
		if !strings.Contains(props, line+"\n") {
			t.Errorf("server.properties does not contain %q\n%s", line, props)
		}
	}
	if n := strings.Count(props, "enable-rcon="); n != 1 {
		t.Errorf("enable-rcon appears %d times, want 1\n%s", n, props)
	}
}
