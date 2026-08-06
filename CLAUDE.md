# translate — 技術脈絡 (Technical Context)

Module path：`github.com/bizshuk/trans`。Executable 位於 module root；
SDK import path 為 `github.com/bizshuk/trans/sdk`。

業務定義見 [README.md](README.md)，領域用詞以
[docs/terminology.md](docs/terminology.md) 為準，設計與落地狀態見
[plans/2026-08-06-translate-design.md](plans/2026-08-06-translate-design.md)。

## 專案結構 (Project Structure)

```tree
trans/
├── main.go              # executable 與 gosdk application bootstrap
├── layering_test.go     # package dependency contract
├── sdk/                 # public SDK、Model registry、routing
├── engine/              # Engine port 與 runtime adapters
│   ├── opencc/          # in-process OpenCC conversion
│   ├── openaiapi/       # OpenAI-compatible local runtime adapter
│   └── ct2/             # CTranslate2 sidecar adapter
├── segment/             # Document segmentation 與 lossless reassembly
├── glossary/            # user-owned forced replacement rules
├── benchmark/           # Sample corpus、Session runner、result persistence
├── cli/                 # Cobra command tree
├── config/              # CLI-only gosdk/viper configuration
├── scripts/             # metadata setup、model pull、CT2 sidecar
├── docs/                # terminology 與 retrospective memory
├── plans/               # active design / implementation plans
├── ecosystem.config.js  # PM2 task definition
└── package.json         # unified project tasks
```

新增 Go package 時必須同步 `layering_test.go` 的 dependency contract。

## 技術棧 (Tech Stack)

| 類別 | 選擇 | 用途 |
| --- | --- | --- |
| Language | Go 1.26.3 | CLI、SDK、Engine adapters、Benchmark |
| CLI | Cobra | command tree 與 flags |
| Configuration | gosdk config + Viper | application config 與 shared config command |
| Chinese conversion | longbridgeapp/opencc | embedded dictionary、無 cgo |
| Local model API | `net/http` | OpenAI-compatible endpoint 與 CT2 sidecar contract |
| CT2 sidecar | Python + FastAPI + CTranslate2 | encoder-decoder Model inference |
| Process management | PM2 ecosystem | `trans-ct2` long-running task definition |

## 模組對應 (Module Mapping)

| 業務領域 (Domain) | 套件/模組 (Package/Module) | 進入點 (Entry Point) |
| --- | --- | --- |
| 中文轉換 (Chinese Conversion) | `sdk`、`engine/opencc`、`glossary`、`segment` | `translate.ToTraditional`、`translate.New`、`translate zh` |
| 跨語言翻譯 (Cross-language Translation) | `sdk`、`engine/openaiapi`、`engine/ct2`、`segment` | `translate.New`、`Translator.Document`、`translate text` |
| 模型治理與評估 (Model Operations and Evaluation) | `sdk/model.go`、`benchmark`、`cli` | `translate.Models`、`translate engine status`、`translate benchmark` |

## 分層契約 (Layering Contract)

| 層 | 套件 | 可 import |
| --- | --- | --- |
| leaf | `segment`、`engine`、`glossary` | 無 project package |
| adapter | `engine/*` | `engine` |
| SDK | `sdk` | `engine`、`engine/*`、`glossary`、`segment` |
| application | `benchmark` | `sdk` |
| CLI | `cli` | `sdk`、`benchmark`、`config`、`glossary` |
| binary | `.` | `cli`、`config` |

`layering_test.go` 同時驗證 package coverage、SDK/config separation、adapter isolation、
root executable 與 application config identity。

## 不變式 (Invariants)

1. `sdk` 不得依賴 `config`。設定來源由 application 決定，SDK 只接受 `Options`。
2. `cli` 不得擁有語言判斷或 Engine routing；CLI 與 SDK 必須共用同一條行為路徑。
3. Engine adapter 彼此獨立，不得互相 import 或綁定彼此 runtime。
4. 不做 silent fallback。Engine、Model 或語言對不可用時回傳明確錯誤。
5. `segment.Parse → Reassemble` 必須保留原始空白、縮排與段落；批次數量不符即失敗。
6. CT2 句級 Model 一律一句一批，避免多句輸入被靜默遺漏。
7. Benchmark 從 Model registry 取得清單，Session 不覆寫既有輸出。

## 關鍵決策 (Key Decisions)

### 中文互轉使用規則字典

中文簡繁與地區用語屬於 deterministic conversion，不交給神經模型改寫。
預設 Profile 是 `s2twp`；OpenCC 刻意保留的有歧義詞由 user-owned Glossary 補足。

### Model 與 Engine 分離

Model 表示使用者選擇的具名能力，Engine 表示執行方式。Model registry 以 Model 為鍵，
同時保存 Engine、runtime requirement、Engine-side model ID 與 sentence-level metadata。

### 批次層級對齊

SDK 以完整 Batch 取代完整 Batch，不要求模型輸出與來源句數一一對齊。
Document structure 由 `segment` 保管，不依賴 Engine 保留 whitespace。

### runtime failure 明確分流

Health Check 分辨「服務未啟動」與「Model 未安裝」，因為兩者修復方式不同。
Benchmark 將 runtime 缺席記為 `unavailable`，不會改用其他 Model。

實測背景見 [docs/memory/2026-08-06-engine-findings.md](docs/memory/2026-08-06-engine-findings.md)。

## 設定與資料 (Configuration and Data)

root `main.go` 以 gosdk application name `trans` 初始化設定。預設根目錄：

```text
~/.config/trans/
├── settings.json
├── glossary.txt
├── models/
└── logs/
```

設定預設值由 `config/default_settings.json` 單一擁有。`config` 只服務 CLI；
SDK consumer 必須透過 `translate.Options` 傳入 Model、Endpoint、Timeout、Profile 與 Glossary。
設定檔中尚未被 application flow 消費的 key 不代表對應 capability 已完成。

## Runtime 與部署 (Runtime and Deployment)

- OpenCC 在 executable 內運行，沒有外部 runtime。
- `openaiapi` 預設連接 localhost 的 OpenAI-compatible endpoint，可由 caller 覆寫。
- `ct2` 透過 `scripts/ct2_server.py` 提供本機 HTTP sidecar。
- `ecosystem.config.js` 定義 `trans-ct2` PM2 task；不會因 build 或 setup 自動套用。
- `scripts/run.sh` 只建立 metadata directories 與 script permissions，不啟動服務。
- 套用 ecosystem 使用 `pm2 task start ./ecosystem.config.js`；套用前需確認其中的 local paths。

## 開發指南 (Development Guide)

```bash
go build -o translate .
go test ./...
go vet ./...
go fmt ./...
```

Production verification 應將 build output 導向 temporary directory，避免覆寫 workspace artifact。
Live Model、paid service 與 PM2 acceptance 不屬於 deterministic Go tests；若未執行必須分開回報。

## 慣例 (Conventions)

- 使用繁體中文敘述搭配 English Terminology；領域正名以 `docs/terminology.md` 為準。
- 一個 package/folder 只擁有一個 domain；新增 package 必須更新 layering contract。
- Error 必須保留 action context 與可操作修法，不用 fallback 隱藏失敗。
- Go tests 預設不依賴 live runtime；HTTP adapter 以 fake server 做 contract tests。
- Historical findings 放 `docs/memory/`；pending work 放 `README.todo`；現況設計放 canonical docs。
