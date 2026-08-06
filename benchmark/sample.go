// Package benchmark 執行所有已知 Model 的共用 Sample corpus，並保存可比較的 Session results。
package benchmark

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// Sample 是一次 Benchmark translation 的固定輸入。
type Sample struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"text"`
}

//go:embed samples.json
var sampleJSON []byte

// Samples 回傳內嵌的 50 句共用 Sample corpus。
func Samples() ([]Sample, error) {
	var samples []Sample
	if err := json.Unmarshal(sampleJSON, &samples); err != nil {
		return nil, fmt.Errorf("benchmark: decode samples: %w", err)
	}
	if len(samples) != 50 {
		return nil, fmt.Errorf("benchmark: sample count %d, want 50", len(samples))
	}

	seen := make(map[string]bool, len(samples))
	for i, sample := range samples {
		if sample.ID == "" || seen[sample.ID] {
			return nil, fmt.Errorf("benchmark: sample %d has empty or duplicate ID %q", i+1, sample.ID)
		}
		seen[sample.ID] = true
		if sample.From != "zh-Hans" || sample.To != "zh-TW" {
			return nil, fmt.Errorf("benchmark: sample %s direction is %s -> %s", sample.ID, sample.From, sample.To)
		}
		if strings.TrimSpace(sample.Text) == "" {
			return nil, fmt.Errorf("benchmark: sample %s has empty text", sample.ID)
		}
	}
	return samples, nil
}
