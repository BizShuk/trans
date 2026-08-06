package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	translate "github.com/bizshuk/trans/sdk"
	"github.com/spf13/viper"
)

func TestBenchmarkCommandWritesStructuredSession(t *testing.T) {
	viper.Set("ct2_endpoint", "http://127.0.0.1:1")
	viper.Set("openai_endpoint", "http://127.0.0.1:1")
	viper.Set("request_timeout", "100ms")
	t.Cleanup(viper.Reset)

	root := t.TempDir()
	cmd := newBenchmarkCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--session", "test-session", "--output", root})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v\n%s", err, output.String())
	}
	sessionDir := filepath.Join(root, "test-session")
	if strings.TrimSpace(output.String()) != sessionDir {
		t.Errorf("command output = %q, want %q", output.String(), sessionDir)
	}
	for _, spec := range translate.Models() {
		path := filepath.Join(sessionDir, string(spec.Model), "result.json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("model result missing at %s: %v", path, err)
		}
	}
}

func TestBenchmarkSessionIDUsesUTC(t *testing.T) {
	now := time.Date(2026, 8, 6, 16, 30, 45, 123456789, time.FixedZone("SGT", 8*60*60))
	if got, want := benchmarkSessionID(now), "20260806T083045.123456789Z"; got != want {
		t.Errorf("benchmarkSessionID = %q, want %q", got, want)
	}
}
