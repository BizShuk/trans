# translate 設計 (Language Conversion Toolkit)

狀態：`實作中（P1–P3 完成，P4 部分完成）`
日期：2026-08-06

---

## 1. 目標 (Goal)

一個以 Go `CLI` 與 `SDK` 為核心的語言轉換工具，
在本機完成兩類工作：

| 類別 | 定義 | 例 |
| --- | --- | --- |
| `轉換 (Conversion)` | 同一語言的字形與地區用語互換 | 简体 → 繁體（台灣用語）|
| `翻譯 (Translation)` | 跨語言 | `en` → `zh-TW`、`ja` → `zh-TW` |

兩者共用同一組介面，因為呼叫端要的是「把這段文字變成我要的語言」，
它不該為了兩種內部機制寫兩套程式碼。

### 1.1 為什麼不是 mactrans

`mactrans` 用 macOS 內建的 `Translation.framework`，品質好、離線、免金鑰，
但有兩個硬限制：

1. **Swift only。** 框架沒有 C ABI，Go 只能透過額外的 helper binary 呼叫，
   而那等於把每一次翻譯變成一次 process spawn。
2. **中文↔中文不支援。** `LanguageAvailability.status(zh-Hans → zh-Hant)`
   實測回報 `unsupported`，mactrans 因此改走 ICU `Simplified-Traditional`——
   那是`字形`轉換：`計算機` 不會變成 `電腦`、`软件` 不會變成 `軟體`。

第 2 點正是本工具的首要需求：拿到的簡體逐字稿要的是`台灣讀者讀起來自然`，
不是筆畫換過去而已。

### 1.2 非目標 (Out of Scope)

- 不做 GUI、不做 Services 選單 —— 那是 `mactrans` 的領域，兩者互補而非取代。
- 不代管任何雲端 LLM provider 的金鑰，也不預設走網路。
- 不做 OCR、不做語音。輸入一律是文字。

---

## 2. 關鍵決策 (Key Decisions)

### 2.1 不用 LLM provider，但用得起本機模型

「不用 LLM provider」的實質是`不把文字送出這台機器`、`不綁定計費 API`。
本機跑的 decoder-only 模型（Qwen2.5、MiniCPM）不違反這條 —— 它們沒有 vendor、
沒有帳單、沒有出網。因此引擎分成三族：

| 族 | 代表 | 性質 |
| --- | --- | --- |
| 規則字典 | OpenCC | 決定性、毫秒級、零模型 |
| 專用 NMT | NLLB-200、OPUS-MT | seq2seq，只會翻譯，不會亂加話 |
| 通用小模型 | Qwen2.5-1.5B | 靠 prompt 翻譯，可處理語氣與術語指示 |

MiniCPM 保留在 Model registry 作為候選，但目前指向的 v4.6 視覺模型不適合文字翻譯；
替代 tag 尚待驗證，不列為推薦能力。

### 2.2 中文轉換`不`交給神經模型

`zh-Hans → zh-Hant` 是查表問題，不是翻譯問題。用 NMT 做它，
換來的是不確定性（會改寫句子）、延遲（百倍）、與難以稽核（同一句兩次結果可能不同）。
OpenCC 的 `s2twp` 走`詞彙級`轉換，`计算机科学` → `電腦科學`、`软件` → `軟體`，
且完全決定性 —— 這是 mactrans 補不上的那一塊。

有歧義的孤立詞不由 OpenCC 猜測：`计算机` → `計算機`、`质量` → `質量`。
特定領域需要其他結果時，在 Engine output 後套用 user-owned Glossary。

實測依據：`github.com/longbridgeapp/opencc@v0.3.13` 為`純 Go`、內嵌字典，
支援 `s2t / s2tw / s2twp / t2s / tw2s / tw2sp / s2hk / hk2s / t2tw / t2hk`，
API 為 `New(conversion)` + `Convert(string)`，無 cgo、無外部檔案。

### 2.3 SDK 是主體，CLI 是它的第一個使用者

`sdk` 套件是 SDK；`cli` 只做旗標解析與輸出格式，根目錄 `main.go`
負責 application bootstrap。
反過來（先寫 CLI 再抽 SDK）會讓 SDK 的簽名帶著 CLI 的味道 ——
`--verbose`、`os.Exit`、印表格這些東西一旦滲進 library，就再也拔不乾淨。

驗收標準：`cli/` 底下不得出現任何語言判斷或引擎選擇邏輯。

### 2.4 引擎不可用時明確失敗，不靜默降級

模型沒下載、sidecar 沒啟動、語言對不支援 —— 一律回明確錯誤並附上修法。
自動退回另一個引擎會讓使用者拿到一份`不知道是誰翻的`譯文，
而翻譯品質的差異大到足以改變決策。呼叫端要改用其他 Model，必須重新明確選擇。

---

## 3. 介面 (Interfaces)

### 3.1 SDK

```go
import "github.com/bizshuk/trans/sdk"

// 最短路徑：簡體轉台灣繁體。
out, err := sdk.ToTraditional(ctx, text)

// 完整形式。
t, err := sdk.New(sdk.Options{
    Model:  sdk.ModelOpenCC,     // 見 §4 的模型表
    From:   "zh-Hans",                 // 空 = 自動偵測
    To:     "zh-TW",
})
out, err := t.Text(ctx, text)
outs, err := t.Batch(ctx, []string{a, b, c})

// 長文：由 SDK 自行斷句、分批、重組，呼叫端不必知道模型的 token 上限。
out, err := t.Document(ctx, longText)
```

`Options` 擁有 `Model`、`From`、`To`、`Profile`、`Prompt`、`MaxChars`、`Endpoint`、
`EngineModel`、`Timeout` 與 `Glossary`。SDK 不讀 application config；CLI 將設定轉成
Options 後才呼叫 SDK。

### 3.2 CLI

```bash
# 中文轉換（最常用，預設 s2twp）
translate zh --to tw < in.txt > out.txt
translate zh --to cn --profile tw2sp -i a.txt -o b.txt

# 跨語言翻譯
translate text --to zh-TW "Ship it before Friday."
pbpaste | translate text --from en --to zh-TW --model nllb-600m

# Model 與 runtime
translate models                      # 已知 Model、Engine 與 runtime requirement
translate models pull nllb-600m
translate engine status               # Model 與 runtime readiness
translate benchmark                   # 固定 corpus 的全 Model comparison
translate glossary                    # 檢視 user-owned Glossary

# gosdk 共用子命令
translate config
```

旗標語意固定：`--from` 空值代表自動偵測；`--model` 空值代表依 `--to`／`--from`
挑預設模型（§4.3）；`-i/-o` 未給時走 stdin/stdout，讓它能站在 pipe 中間。

---

## 4. 引擎與模型 (Engines & Models)

### 4.1 Engine port

```go
// Engine 是一種「把文字變成另一種語言」的實作方式。
type Engine interface {
    Name() string
    Translate(ctx context.Context, req Request) (Result, error)
    Health(ctx context.Context) error          // 模型在不在、服務起來沒
    Supports(from, to string) bool
}
```

`Health` 是介面的一等公民而不是附屬品：本機模型的失敗模式九成是
「沒下載」與「沒啟動」，把它塞進 `Translate` 的錯誤裡等於逼使用者從
一段翻譯失敗訊息裡反推部署問題。

### 4.2 Model registry

| Model literal | Engine | Runtime | 現況 |
| --- | --- | --- | --- |
| `opencc` | `opencc` | in-process | 中文 Conversion，可直接使用 |
| `nllb-600m` | `ct2` | CTranslate2 sidecar | 200 語 NMT；句級 Model |
| `opus-mt` | `ct2` | CTranslate2 sidecar | 英 → 中 inference 已實測；clean-install path 待驗證 |
| `qwen2.5-1.5b` | `openaiapi` | ollama／llama-server | 推薦的通用小模型候選 |
| `minicpm-2b` | `openaiapi` | ollama／llama-server | 目前 Engine Model 不適合翻譯，待換 tag |

### 4.3 預設路由

| 情境 | 預設模型 | 理由 |
| --- | --- | --- |
| `zh-Hans` ↔ `zh-Hant`/`zh-TW`/`zh-HK` | `ModelOpenCC` | 決定性、毫秒級 |
| 其他語言對 | `ModelNLLB600M` | 一顆模型覆蓋所有語言對，不必每對裝一個 |
| 呼叫端明說 `--model` | 照辦 | 明說優先於推論 |

### 4.4 三個 runtime adapter

| Adapter | 對應模型 | 傳輸 |
| --- | --- | --- |
| `engine/opencc` | OpenCC | 行程內，零 IPC |
| `engine/ct2` | NLLB、OPUS-MT | HTTP 到本機 CTranslate2 sidecar |
| `engine/openaiapi` | Qwen2.5、MiniCPM | HTTP `/v1/chat/completions`（ollama 或 llama-server 同一份契約） |

`ct2` sidecar 是本專案唯一的非 Go 成分：CTranslate2 只有 Python／C++ 綁定，
NLLB 與 OPUS-MT 都是 encoder-decoder，llama.cpp 不支援這個架構。
sidecar 由 `scripts/ct2_server.py` 提供 FastAPI contract，
契約由本專案定義並以 pm2 常駐（`ecosystem.config.js` 的 `trans-ct2`）。

> 為什麼不直接用 LibreTranslate：它自帶一整套 web UI、API key 管理與
> argos 套件系統，而我們要的只是「送一段字、回一段字」。多帶的東西
> 都要跟著升級、跟著設定、跟著出事。

---

## 5. 套件佈局 (Package Layout)

```tree
trans/
├── main.go             # executable 與 gosdk application bootstrap
├── sdk/                # SDK：Options、Text/Batch/Document、Model registry
├── engine/
│   ├── engine.go       # Engine port + registry
│   ├── opencc/         # 規則字典（純 Go，行程內）
│   ├── ct2/            # CTranslate2 sidecar client
│   └── openaiapi/      # ollama / llama-server client
├── segment/            # 斷句、分批、重組（純函式）
├── glossary/           # user-owned forced replacement rules
├── benchmark/          # fixed corpus、Session runner、results
├── cli/                # cobra：zh / text / models / engine
├── config/             # 扁平 viper key
├── scripts/ct2_server.py
└── ecosystem.config.js # pm2：trans-ct2 sidecar
```

依賴方向與白名單由 `CLAUDE.md` 與 `layering_test.go` 單一擁有。
`engine/*` 之間不得互相 import；`segment` 不得 import 任何 engine。

---

## 6. 長文處理 (Document Handling)

NMT 模型有 token 上限（NLLB 約 512、OPUS-MT 更短），
而真實輸入是一整份逐字稿。`segment` 負責：

1. `斷句`：以中英文句尾標點切句，保留小數點與縮寫中的 `.`。
2. `分批`：一般 Model 依 `MaxChars` 合併；CT2 句級 Model 強制一句一批。
3. `重組`：譯文以原本的分隔符接回，空行與段落結構原樣保留 ——
   逐字稿的段落是後續筆記生成的線索，被壓平就再也回不來。

`ModelOpenCC` 不需要分批（字典轉換沒有長度上限），但仍走同一條路徑，
好讓「換一個模型輸出結構就變了」不會發生。

已知風險：句級翻譯會失去跨句代名詞的上下文。這是所有 NMT 的通病，
Qwen／MiniCPM 這類 decoder-only 模型可以吃更長的上下文，
因此 `openaiapi` adapter 的批次上限單獨設定，不與 NMT 共用。

---

## 7. 設定 (Configuration)

`扁平 viper key`，預設值內嵌，設定根目錄由 gosdk `config.Default` 固定於
`~/.config/trans/`。模型檔放 `~/.config/trans/models/`。

| Key | 預設 | 說明 |
| --- | --- | --- |
| `zh_profile` | `s2twp` | 中文轉換的預設 OpenCC profile |
| `zh_target` | `zh-TW` | `translate zh` 的預設目標 |
| `ct2_endpoint` | `http://127.0.0.1:8765` | sidecar 位址 |
| `openai_endpoint` | `http://127.0.0.1:11434/v1` | ollama 預設埠 |
| `segment_max_chars` | `1200` | NMT 的批次上限 |
| `segment_max_chars_llm` | `4000` | decoder-only 模型的批次上限 |
| `request_timeout` | `2m` | 單次請求上限 |

`default_model`、`openai_model`、`cache_enabled` 目前存在於 default settings，
但 application flow 尚未消費；它們不代表相關 capability 已完成。

---

## 8. 驗證策略 (Verification)

| 層 | 方式 | 不依賴 |
| --- | --- | --- |
| `segment` | 純函式黃金測資（含空行、中英混排、清單） | 一切 |
| `engine/opencc` | 地區用語與歧義邊界：`软件→軟體`、`计算机→計算機` | 網路、模型 |
| `engine/ct2`、`openaiapi` | fake HTTP server contract tests | live runtime |
| SDK | 路由測試：語言對 → 選到哪個 engine | 模型 |
| CLI | Benchmark command integration；root executable/config identity | live runtime |

地區用語與歧義邊界分成兩組 executable tests：明確詞組必須轉換，
有歧義的孤立詞必須保持 OpenCC upstream 行為。

---

## 9. 落地順序 (Landing Order)

| Phase | 內容 | 狀態 |
| --- | --- | --- |
| P1 | SDK、OpenCC、segment、Glossary、`translate zh` | 完成 |
| P2 | OpenAI-compatible adapter、output cleaning、`translate text` | 完成 |
| P3 | CT2 sidecar、NLLB、OPUS pull/convert path、Model commands | 核心完成；clean-install path 待驗證 |
| P4 | Health、PM2、Benchmark、cache | Health／Benchmark 完成；PM2 live acceptance 與 cache 待完成 |

---

## 10. 未決問題 (Open Questions)

1. **CTranslate2 在 Apple Silicon 只有 CPU 路徑**（無 Metal）。
   NLLB-600M int8 在 M4 上的實際吞吐待實測；若慢到不可用，
   P3 要改以 OPUS-MT 單語言對為主力，NLLB 降為備援。
2. `MiniCPM` 應改用哪個純文字 Model tag，才能穩定遵守 translation-only instruction？
3. `vid-note` 該在哪一層接：轉換放在 `svc/stt` 出口（逐字稿即是繁體）
   還是留給 note 階段的 prompt？前者讓 artifact 一致，後者省一次處理。
4. Translation cache 應以哪些輸入組成 key，並如何避免 stale Model output？
