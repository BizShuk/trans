package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/bizshuk/trans/sdk"
)

var pathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type sessionSummary struct {
	SessionID   string         `json:"session_id"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  time.Time      `json:"finished_at"`
	SampleCount int            `json:"sample_count"`
	Models      []modelSummary `json:"models"`
}

type modelSummary struct {
	Model          sdk.Model `json:"model"`
	Engine         string    `json:"engine"`
	Status         Status    `json:"status"`
	Error          string    `json:"error,omitempty"`
	CompletedCount int       `json:"completed_count"`
	FailedCount    int       `json:"failed_count"`
	DurationNS     int64     `json:"duration_ns"`
	ResultPath     string    `json:"result_path"`
}

// Write 將 Report 保存成 Session → Model → result 結構，且不覆寫既有 Session。
func Write(root string, report Report) (string, error) {
	if root == "" {
		return "", fmt.Errorf("benchmark: output root 不可為空")
	}
	if !pathSegmentPattern.MatchString(report.SessionID) {
		return "", fmt.Errorf("benchmark: session ID %q 不安全", report.SessionID)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("benchmark: 建立 output root %s：%w", root, err)
	}

	sessionDir := filepath.Join(root, report.SessionID)
	if err := os.Mkdir(sessionDir, 0o755); err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("benchmark: session %q 已存在", report.SessionID)
		}
		return "", fmt.Errorf("benchmark: 建立 session %s：%w", sessionDir, err)
	}

	summary := sessionSummary{
		SessionID:   report.SessionID,
		StartedAt:   report.StartedAt,
		FinishedAt:  report.FinishedAt,
		SampleCount: report.SampleCount,
		Models:      make([]modelSummary, 0, len(report.Results)),
	}
	for _, result := range report.Results {
		if result.SessionID != report.SessionID {
			return "", fmt.Errorf("benchmark: model %s 的 session ID %q 不一致",
				result.Model, result.SessionID)
		}
		modelName := string(result.Model)
		if !pathSegmentPattern.MatchString(modelName) {
			return "", fmt.Errorf("benchmark: model name %q 不安全", modelName)
		}

		modelDir := filepath.Join(sessionDir, modelName)
		if err := os.Mkdir(modelDir, 0o755); err != nil {
			return "", fmt.Errorf("benchmark: 建立 model directory %s：%w", modelDir, err)
		}
		resultPath := filepath.Join(modelDir, "result.json")
		if err := writeJSON(resultPath, result); err != nil {
			return "", err
		}

		summary.Models = append(summary.Models, modelSummary{
			Model:          result.Model,
			Engine:         result.Engine,
			Status:         result.Status,
			Error:          result.Error,
			CompletedCount: result.CompletedCount,
			FailedCount:    result.FailedCount,
			DurationNS:     result.DurationNS,
			ResultPath:     filepath.ToSlash(filepath.Join(modelName, "result.json")),
		})
	}

	if err := writeJSON(filepath.Join(sessionDir, "session.json"), summary); err != nil {
		return "", err
	}
	return sessionDir, nil
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("benchmark: encode %s：%w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("benchmark: write %s：%w", path, err)
	}
	return nil
}
