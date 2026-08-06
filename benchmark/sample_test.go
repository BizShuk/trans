package benchmark

import (
	"strings"
	"testing"
)

func TestSamplesAreCompleteAndValid(t *testing.T) {
	samples, err := Samples()
	if err != nil {
		t.Fatalf("Samples: %v", err)
	}
	if len(samples) != 50 {
		t.Fatalf("sample count = %d, want 50", len(samples))
	}

	seen := make(map[string]bool, len(samples))
	for i, sample := range samples {
		if sample.ID == "" {
			t.Errorf("sample %d has empty ID", i+1)
		}
		if seen[sample.ID] {
			t.Errorf("sample ID %q is duplicated", sample.ID)
		}
		seen[sample.ID] = true

		if sample.From != "zh-Hans" || sample.To != "zh-TW" {
			t.Errorf("sample %s direction = %s -> %s, want zh-Hans -> zh-TW",
				sample.ID, sample.From, sample.To)
		}
		if strings.TrimSpace(sample.Text) == "" {
			t.Errorf("sample %s has empty text", sample.ID)
		}
	}
}
