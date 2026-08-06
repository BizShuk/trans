package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	translate "github.com/bizshuk/trans/sdk"
)

func TestWriteUsesSessionModelResultLayout(t *testing.T) {
	now := time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)
	report := Report{
		SessionID:   "20260806T080000Z",
		StartedAt:   now,
		FinishedAt:  now.Add(time.Second),
		SampleCount: 1,
		Results: []ModelResult{
			{
				SessionID: "20260806T080000Z", Model: translate.ModelOpenCC,
				Engine: "opencc", Status: StatusCompleted, InputCount: 1, CompletedCount: 1,
			},
			{
				SessionID: "20260806T080000Z", Model: translate.ModelQwen25,
				Engine: "openaiapi", Status: StatusUnavailable, Error: "model unavailable",
			},
		},
	}

	sessionDir, err := Write(t.TempDir(), report)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if filepath.Base(sessionDir) != report.SessionID {
		t.Fatalf("session dir = %s", sessionDir)
	}

	for _, model := range []translate.Model{translate.ModelOpenCC, translate.ModelQwen25} {
		path := filepath.Join(sessionDir, string(model), "result.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var result ModelResult
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if result.Model != model || result.SessionID != report.SessionID {
			t.Errorf("unexpected result at %s: %+v", path, result)
		}
	}

	if _, err := os.Stat(filepath.Join(sessionDir, "session.json")); err != nil {
		t.Fatalf("session summary missing: %v", err)
	}
}

func TestWriteRejectsExistingOrUnsafeSession(t *testing.T) {
	root := t.TempDir()
	report := Report{SessionID: "same-session"}
	if _, err := Write(root, report); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if _, err := Write(root, report); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("second Write error = %v, want 已存在", err)
	}

	report.SessionID = "../outside"
	if _, err := Write(root, report); err == nil {
		t.Fatal("unsafe session ID must be rejected")
	}
}
