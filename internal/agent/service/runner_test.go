package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	agentcfg "github.com/badskater/distributed-encoder/internal/agent/config"
)

// buildToolEnv returns the environment slice the runner would build for a task,
// using the same logic as executeTask but without the full process setup.
// It exists to let us test the tool-path injection rules in isolation.
func buildToolEnv(tools agentcfg.ToolsConfig, taskVars map[string]string) []string {
	env := os.Environ()
	if tools.FFmpeg != "" {
		env = append(env, "FFMPEG_BIN="+tools.FFmpeg)
	}
	if tools.FFprobe != "" {
		env = append(env, "FFPROBE_BIN="+tools.FFprobe)
	}
	if tools.DoviTool != "" {
		env = append(env, "DOVI_TOOL_BIN="+tools.DoviTool)
	}
	for k, v := range taskVars {
		env = append(env, k+"="+v)
	}
	return env
}

// lastValue returns the last value assigned to key in a KEY=VALUE slice,
// mimicking the "last assignment wins" behaviour of most process environments.
func lastValue(env []string, key string) string {
	val := ""
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			val = strings.TrimPrefix(entry, prefix)
		}
	}
	return val
}

func TestBuildToolEnv_ToolsInjected(t *testing.T) {
	tools := agentcfg.ToolsConfig{
		FFmpeg:   "/usr/bin/ffmpeg",
		FFprobe:  "/usr/bin/ffprobe",
		DoviTool: "/usr/local/bin/dovi_tool",
	}
	env := buildToolEnv(tools, nil)

	checks := map[string]string{
		"FFMPEG_BIN":    "/usr/bin/ffmpeg",
		"FFPROBE_BIN":   "/usr/bin/ffprobe",
		"DOVI_TOOL_BIN": "/usr/local/bin/dovi_tool",
	}
	for key, want := range checks {
		if got := lastValue(env, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestBuildToolEnv_EmptyToolsNotInjected(t *testing.T) {
	// When a tool path is empty, the env var should not appear at all.
	tools := agentcfg.ToolsConfig{
		FFmpeg:   "/usr/bin/ffmpeg",
		FFprobe:  "",
		DoviTool: "",
	}
	env := buildToolEnv(tools, nil)

	for _, key := range []string{"FFPROBE_BIN", "DOVI_TOOL_BIN"} {
		if got := lastValue(env, key); got != "" {
			t.Errorf("%s should not be set, but got %q", key, got)
		}
	}
	if got := lastValue(env, "FFMPEG_BIN"); got != "/usr/bin/ffmpeg" {
		t.Errorf("FFMPEG_BIN = %q, want /usr/bin/ffmpeg", got)
	}
}

func TestBuildToolEnv_TaskVarsOverrideTools(t *testing.T) {
	// Task variables must override the agent config tool paths when the same
	// key is present, since they are appended last.
	tools := agentcfg.ToolsConfig{
		FFprobe: "/usr/bin/ffprobe",
	}
	taskVars := map[string]string{
		"FFPROBE_BIN": "/custom/ffprobe",
	}
	env := buildToolEnv(tools, taskVars)

	if got := lastValue(env, "FFPROBE_BIN"); got != "/custom/ffprobe" {
		t.Errorf("FFPROBE_BIN = %q, want /custom/ffprobe (task var should override)", got)
	}
}

func TestBuildToolEnv_DoviToolInjected(t *testing.T) {
	tools := agentcfg.ToolsConfig{DoviTool: "/opt/dovi_tool/dovi_tool"}
	env := buildToolEnv(tools, nil)

	if got := lastValue(env, "DOVI_TOOL_BIN"); got != "/opt/dovi_tool/dovi_tool" {
		t.Errorf("DOVI_TOOL_BIN = %q, want /opt/dovi_tool/dovi_tool", got)
	}
}

// TestToolEnvVisibleToChild verifies end-to-end that a child process can read
// FFPROBE_BIN and DOVI_TOOL_BIN when they are set via buildToolEnv.
// The test is skipped on platforms where "sh" is unavailable (e.g. pure Windows).
func TestToolEnvVisibleToChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh not available on Windows; tool env injection is covered by unit tests above")
	}

	tools := agentcfg.ToolsConfig{
		FFprobe:  "/test/ffprobe",
		DoviTool: "/test/dovi_tool",
	}
	env := buildToolEnv(tools, nil)

	// Write a tiny shell script that prints the env vars.
	dir := t.TempDir()
	script := filepath.Join(dir, "check.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho FFPROBE_BIN=$FFPROBE_BIN\necho DOVI_TOOL_BIN=$DOVI_TOOL_BIN\n"), 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	cmd := exec.Command("/bin/sh", script)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running script: %v", err)
	}

	got := string(out)
	if !strings.Contains(got, "FFPROBE_BIN=/test/ffprobe") {
		t.Errorf("child did not see FFPROBE_BIN; output:\n%s", got)
	}
	if !strings.Contains(got, "DOVI_TOOL_BIN=/test/dovi_tool") {
		t.Errorf("child did not see DOVI_TOOL_BIN; output:\n%s", got)
	}
}
