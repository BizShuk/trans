# Model Benchmark 實作計畫 (Implementation Plan)

狀態：`完成`
日期：2026-08-06

> **For agentic workers:** 依序完成下列任務，以 TDD 驗證每一層；本專案直接在 `master` 執行，不建立 branch 或 worktree。

**Goal:** 建立可重跑的全 Model Benchmark，以同一組 50 句 Sample corpus 產生可比較的 Session results。

**Architecture:** Benchmark 由獨立 package 擁有 corpus、執行與 result persistence；CLI 只負責啟動。輸出採 `Session → Model → result`，每個已知 Model 都有結果，未落地或不可用的 Model 明確標示狀態。

**Tech Stack:** Go、既有 translate SDK、Cobra、JSON

## Global Constraints

- 使用繁體中文 + English Terminology。
- 不做 backward compatibility、fallback 或 migration。
- 一個 Session 使用同一組 50 句 `zh-Hans → zh-TW` Sample corpus。
- 自動從 Model registry 取得全部 Model，不維護第二份 Model 清單。
- 不可用的 Model 不得由其他 Model 代跑。
- 輸出固定為 `benchmark/sessions/<session>/<model>/result.json`。
- 直接在 `master` 工作，不建立 branch 或 worktree。

---

### Task 1: Sample corpus

**Files:**

- Create: `benchmark/samples.json`
- Create: `benchmark/sample.go`
- Test: `benchmark/sample_test.go`

- [x] 建立 50 句涵蓋科技、商務、金融、口語、格式與歧義詞的共用 corpus。
- [x] 驗證 ID 唯一、語言方向完整、句子非空且總數固定為 50。
- [x] 執行 targeted test，完成 RED → GREEN。

### Task 2: Session runner and result persistence

**Files:**

- Create: `benchmark/result.go`
- Create: `benchmark/run.go`
- Create: `benchmark/write.go`
- Test: `benchmark/run_test.go`
- Test: `benchmark/write_test.go`

- [x] 依 Model registry 順序執行全部 Model，保存每句 output 與 duration。
- [x] 明確區分 `completed`、`unavailable`、`failed`，保留原始錯誤原因。
- [x] 建立唯一 Session directory 與每個 Model 的 `result.json`，拒絕覆寫既有 Session。
- [x] 執行 targeted tests，逐項完成 RED → GREEN。

### Task 3: CLI integration and project contract

**Files:**

- Create: `cmd/benchmark.go`
- Test: `cmd/benchmark_test.go`
- Modify: `layering_test.go`
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `README.todo`
- Modify: `docs/terminology.md`

- [x] 增加 `translate benchmark` 入口與 Session/output 選項。
- [x] 更新 package layering contract 與使用者文件。
- [x] 執行一次完整 Benchmark Session，保存全部 Model results。
- [x] 執行 format、test、vet、build 與 result schema/structure 驗證。
