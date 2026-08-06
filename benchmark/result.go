package benchmark

import (
	"time"

	translate "github.com/bizshuk/trans/sdk"
)

// Status 是一個 Model 在本次 Session 的完成狀態。
type Status string

const (
	// StatusCompleted 代表所有 Sample 都已產生 output。
	StatusCompleted Status = "completed"

	// StatusUnavailable 代表 Model 的 runtime、服務或模型檔不可用。
	StatusUnavailable Status = "unavailable"

	// StatusFailed 代表 Model 可用，但至少一個 Sample translation 失敗。
	StatusFailed Status = "failed"
)

// Report 是一次 Benchmark Session 的完整執行結果。
type Report struct {
	SessionID   string        `json:"session_id"`
	StartedAt   time.Time     `json:"started_at"`
	FinishedAt  time.Time     `json:"finished_at"`
	SampleCount int           `json:"sample_count"`
	Results     []ModelResult `json:"results"`
}

// ModelResult 是一個 Model 在同一組 Sample corpus 上的結果。
type ModelResult struct {
	SessionID      string          `json:"session_id"`
	Model          translate.Model `json:"model"`
	Engine         string          `json:"engine"`
	EngineModel    string          `json:"engine_model,omitempty"`
	Requires       string          `json:"requires,omitempty"`
	Description    string          `json:"description"`
	Status         Status          `json:"status"`
	Error          string          `json:"error,omitempty"`
	StartedAt      time.Time       `json:"started_at"`
	FinishedAt     time.Time       `json:"finished_at"`
	DurationNS     int64           `json:"duration_ns"`
	InputCount     int             `json:"input_count"`
	CompletedCount int             `json:"completed_count"`
	FailedCount    int             `json:"failed_count"`
	Samples        []SampleResult  `json:"samples"`
}

// SampleResult 保存單句的 input、output、error 與 duration。
type SampleResult struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	To         string `json:"to"`
	Input      string `json:"input"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationNS int64  `json:"duration_ns"`
}
