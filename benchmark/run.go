package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/bizshuk/trans/sdk"
)

// Options 是一次 Benchmark Session 的執行參數。
type Options struct {
	SessionID string
	Samples   []Sample

	// Endpoints 以 Engine name 為鍵，空值使用各 Engine 的預設位址。
	Endpoints map[string]string

	// Timeout 是單次 health check 與 translation 的上限。
	Timeout time.Duration
}

// Run 依 Model registry 順序執行全部 Model，不因單一 Model 不可用而中止 Session。
func Run(ctx context.Context, opt Options) Report {
	startedAt := time.Now().UTC()
	report := Report{
		SessionID:   opt.SessionID,
		StartedAt:   startedAt,
		SampleCount: len(opt.Samples),
		Results:     make([]ModelResult, 0, len(sdk.Models())),
	}

	for _, spec := range sdk.Models() {
		report.Results = append(report.Results, runModel(ctx, opt, spec))
	}
	report.FinishedAt = time.Now().UTC()
	return report
}

func runModel(ctx context.Context, opt Options, spec sdk.Spec) ModelResult {
	startedAt := time.Now().UTC()
	result := ModelResult{
		SessionID:   opt.SessionID,
		Model:       spec.Model,
		Engine:      spec.Engine,
		EngineModel: spec.EngineModel,
		Requires:    spec.Requires,
		Description: spec.Description,
		Status:      StatusCompleted,
		StartedAt:   startedAt,
		InputCount:  len(opt.Samples),
		Samples:     make([]SampleResult, 0, len(opt.Samples)),
	}

	if len(opt.Samples) == 0 {
		return finishModel(result, startedAt)
	}

	translator, err := sdk.New(sdk.Options{
		Model:    spec.Model,
		From:     opt.Samples[0].From,
		To:       opt.Samples[0].To,
		Endpoint: opt.Endpoints[spec.Engine],
		Timeout:  opt.Timeout,
	})
	if err != nil {
		result.Status = StatusUnavailable
		result.Error = err.Error()
		return finishModel(result, startedAt)
	}
	if err := translator.Health(ctx); err != nil {
		result.Status = StatusUnavailable
		result.Error = err.Error()
		return finishModel(result, startedAt)
	}

	for _, sample := range opt.Samples {
		sampleStartedAt := time.Now()
		output, err := translator.Text(ctx, sample.Text)
		sampleResult := SampleResult{
			ID:         sample.ID,
			From:       sample.From,
			To:         sample.To,
			Input:      sample.Text,
			DurationNS: time.Since(sampleStartedAt).Nanoseconds(),
		}
		if err != nil {
			sampleResult.Error = err.Error()
			result.FailedCount++
		} else {
			sampleResult.Output = output
			result.CompletedCount++
		}
		result.Samples = append(result.Samples, sampleResult)
	}

	if result.FailedCount > 0 {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("%d/%d samples failed", result.FailedCount, result.InputCount)
	}
	return finishModel(result, startedAt)
}

func finishModel(result ModelResult, startedAt time.Time) ModelResult {
	result.FinishedAt = time.Now().UTC()
	result.DurationNS = result.FinishedAt.Sub(startedAt).Nanoseconds()
	return result
}
