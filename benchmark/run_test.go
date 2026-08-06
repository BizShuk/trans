package benchmark

import (
	"context"
	"testing"
	"time"

	"github.com/bizshuk/trans/sdk"
)

func TestRunBenchmarksEveryKnownModelWithoutFallback(t *testing.T) {
	samples := []Sample{
		{ID: "S001", From: "zh-Hans", To: "zh-TW", Text: "这个软件很好。"},
		{ID: "S002", From: "zh-Hans", To: "zh-TW", Text: "鼠标连接到计算机。"},
	}

	report := Run(context.Background(), Options{
		SessionID: "test-session",
		Samples:   samples,
		Endpoints: map[string]string{
			"ct2":       "http://127.0.0.1:1",
			"openaiapi": "http://127.0.0.1:1",
		},
		Timeout: 100 * time.Millisecond,
	})
	if report.SessionID != "test-session" || report.SampleCount != len(samples) {
		t.Fatalf("unexpected report header: %+v", report)
	}
	if len(report.Results) != len(sdk.Models()) {
		t.Fatalf("model result count = %d, want %d", len(report.Results), len(sdk.Models()))
	}

	results := make(map[sdk.Model]ModelResult, len(report.Results))
	for _, result := range report.Results {
		results[result.Model] = result
	}

	openCC := results[sdk.ModelOpenCC]
	if openCC.Status != StatusCompleted {
		t.Fatalf("opencc status = %q, error = %q", openCC.Status, openCC.Error)
	}
	if openCC.CompletedCount != len(samples) || len(openCC.Samples) != len(samples) {
		t.Fatalf("opencc counts = completed %d, samples %d", openCC.CompletedCount, len(openCC.Samples))
	}
	if openCC.Samples[0].Output != "這個軟體很好。" {
		t.Errorf("opencc output = %q", openCC.Samples[0].Output)
	}

	for _, spec := range sdk.Models() {
		if spec.Model == sdk.ModelOpenCC {
			continue
		}
		result := results[spec.Model]
		if result.Status != StatusUnavailable {
			t.Errorf("%s status = %q, want unavailable", spec.Model, result.Status)
		}
		if result.Requires != spec.Requires || result.Error == "" {
			t.Errorf("%s does not preserve availability context: %+v", spec.Model, result)
		}
		if result.CompletedCount != 0 || len(result.Samples) != 0 {
			t.Errorf("%s must not fall back to another model: %+v", spec.Model, result)
		}
	}
}
