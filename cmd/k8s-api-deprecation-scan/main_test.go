package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// runCLI drives run() with the given arguments and captures stdout.
func runCLI(t *testing.T, args ...string) (int, string) {
	t.Helper()
	oldArgs, oldStdout := os.Args, os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Args = append([]string{"k8s-api-deprecation-scan"}, args...)
	os.Stdout = w
	code := run()
	w.Close()
	os.Args, os.Stdout = oldArgs, oldStdout

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	r.Close()
	return code, sb.String()
}

const examples = "../../examples/manifests"

func TestExitsOneOnRemovedAPIs(t *testing.T) {
	code, out := runCLI(t, "--target-version", "1.29", examples)
	if code != 1 {
		t.Errorf("exit = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "REMOVED") {
		t.Errorf("want REMOVED lines, got:\n%s", out)
	}
}

func TestExitsZeroWhenOnlyDeprecated(t *testing.T) {
	// Every example uses an API removed at 1.22 or later, so 1.21 leaves only warnings.
	code, out := runCLI(t, "--target-version", "1.21", examples)
	if code != 0 {
		t.Errorf("exit = %d, want 0\n%s", code, out)
	}
	if strings.Contains(out, "REMOVED") {
		t.Errorf("did not expect REMOVED at 1.21:\n%s", out)
	}
	if !strings.Contains(out, "DEPRECATED") {
		t.Errorf("want DEPRECATED lines, got:\n%s", out)
	}
}

func TestJSONOutputParses(t *testing.T) {
	code, out := runCLI(t, "--target-version", "1.30", "--format", "json", examples)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	var payload struct {
		TargetVersion string `json:"targetVersion"`
		Findings      []struct {
			Kind  string `json:"kind"`
			Level string `json:"level"`
		} `json:"findings"`
		Summary struct {
			Removed int `json:"removed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if payload.TargetVersion != "1.30" {
		t.Errorf("targetVersion = %q", payload.TargetVersion)
	}
	if payload.Summary.Removed != len(payload.Findings) || payload.Summary.Removed != 4 {
		t.Errorf("summary.removed = %d with %d findings, want 4/4",
			payload.Summary.Removed, len(payload.Findings))
	}
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{},                                       // no target version
		{"--target-version", "1.29"},             // no paths
		{"--target-version", "banana", examples}, // unparseable version
		{"--target-version", "1.29", "--format", "yaml", examples}, // bad format
	}
	for _, args := range cases {
		if code, out := runCLI(t, args...); code != 2 {
			t.Errorf("args %v: exit = %d, want 2\n%s", args, code, out)
		}
	}
}

func TestListRules(t *testing.T) {
	code, out := runCLI(t, "--list-rules")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var rules []map[string]any
	if err := json.Unmarshal([]byte(out), &rules); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rules) < 40 {
		t.Errorf("got %d rules, want the full table", len(rules))
	}
}

func TestVersionFlag(t *testing.T) {
	code, out := runCLI(t, "--version")
	if code != 0 || strings.TrimSpace(out) != version {
		t.Errorf("exit = %d, out = %q, want 0 / %q", code, out, version)
	}
}
