# translate

`translate` 是 local-first 語言轉換工具，以同一組 `CLI` 與 `SDK` 介面處理
中文字形／地區用語轉換，以及跨語言翻譯。中文轉換可直接在 Go process 內執行；
跨語言翻譯則連接使用者準備的本機模型 runtime。

## 業務目的 (Purpose)

本工具讓逐字稿、文件與程式輸入能在保留段落結構的前提下轉成目標語言。
它特別補足「簡體中文 → 台灣繁體中文」不只換字形、也要換地區用語的需求，
同時讓跨語言模型維持可選、可追蹤且不會 silent fallback。

## 業務領域 (Business Domains)

### 中文轉換 (Chinese Conversion)

使用 OpenCC 規則字典處理中文字形與地區用語。相同輸入與 Profile 會得到相同輸出，
不需要下載模型；使用者可再套用自己的 Glossary 強制統一領域詞彙。

`領域流程 (Domain Flow):`

1. 接收文字、目標地區與可選 Profile。
2. 依語言標籤決定轉換方向並套用字典。
3. 套用 Glossary，保留原始空行、縮排與段落。

`核心實體 (Key Entities):` Conversion Profile、Language Tag、Glossary。

### 跨語言翻譯 (Cross-language Translation)

使用具名 Model 將文字翻成另一種語言。未指定 Model 時由 SDK 依語言對路由；
runtime 或 Model 不可用時明確失敗，不會自動換另一顆 Model 代跑。

`領域流程 (Domain Flow):`

1. 接收來源語言、目標語言與可選 Model。
2. 確認 Model 與 runtime 已就緒。
3. 將文件分成安全批次後翻譯，再按原始結構重組。

`核心實體 (Key Entities):` Model、Engine、Runtime、Document、Batch。

### 模型治理與評估 (Model Operations and Evaluation)

Model registry 集中定義可選 Model 與 runtime requirement。使用者可以檢查就緒狀態、
安裝 CTranslate2 Model，或用固定 Sample corpus 執行 Benchmark Session。

`領域流程 (Domain Flow):`

1. 從 Model registry 取得完整 Model 清單。
2. 檢查服務與 Model 是否可用，保留明確原因。
3. 對全部 Model 執行相同 corpus，分別保存結果與狀態。

`核心實體 (Key Entities):` Model registry、Health Check、Benchmark、Session、Model Result。

## 領域關聯 (Domain Relationships)

中文轉換與跨語言翻譯共用 Document segmentation、Batch translation、Reassembly 與
Glossary pipeline。模型治理提供跨語言翻譯所需的就緒資訊；Benchmark 使用同一個 SDK，
因此評估結果與一般翻譯不會走出兩套行為。

## 快速開始 (Quick Start)

### CLI

```bash
translate zh "这个软件的视频质量很好。"
pbpaste | translate zh --to tw
translate text --from en --to zh-TW --model qwen2.5-1.5b "Ship it before Friday."
translate models
translate engine status
translate benchmark
```

Glossary 預設位於 `~/.config/trans/glossary.txt`，每行使用
`來源文字<TAB>目標文字`：

```bash
translate glossary
translate zh --glossary ./glossary.txt "计算机"
```

### SDK

```go
import "github.com/bizshuk/trans/sdk"

out, err := sdk.ToTraditional(ctx, text)

t, err := sdk.New(sdk.Options{
    Model: sdk.ModelOpenCC,
    To:    "zh-TW",
})
out, err := t.Document(ctx, longText)
```

## 支援邊界 (Support Boundaries)

- OpenCC 中文轉換不需要外部 runtime，可直接使用。
- 跨語言 Model 需要本機 runtime 與模型檔；以 `translate engine status` 檢查。
- 預設 endpoint 指向 localhost。使用 `--endpoint` 或 SDK `Endpoint` 連到其他位置時，
  資料邊界由呼叫端自行負責。
- OpenCC 刻意保留有歧義的孤立詞，例如 `计算机 → 計算機`、`质量 → 質量`；
  特定領域需要不同結果時使用 Glossary。
- 句級 NMT Model 一律一句一批，避免多句輸入被 runtime 靜默漏掉。
- Engine 或 Model 不可用時不做 silent fallback。

## 文件導覽 (Documentation)

- [CLAUDE.md](CLAUDE.md)：技術結構、分層契約、runtime 與開發方式。
- [README.business.md](README.business.md)：業務價值、流程、約束與風險。
- [docs/terminology.md](docs/terminology.md)：領域術語、Profile、狀態值與縮寫。
- [README.todo](README.todo)：尚未完成或尚待驗證的工作。
- [plans/2026-08-06-translate-design.md](plans/2026-08-06-translate-design.md)：目前設計與落地狀態。
- [docs/memory/2026-08-06-engine-findings.md](docs/memory/2026-08-06-engine-findings.md)：模型與引擎的實測決策紀錄。
