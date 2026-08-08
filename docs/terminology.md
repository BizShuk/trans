# translate — 術語表 (Terminology)

本檔是領域名詞、狀態值與縮寫的單一定義來源。CLI、SDK 與文件使用同一組正名。

## 中文轉換 (Chinese Conversion)

| 術語 (Term) | 英文 (English) | 定義 (Definition) | 出處 (Source) |
| --- | --- | --- | --- |
| 轉換 | Conversion | 同一語言內的字形與地區用語互換，例如簡體中文轉台灣繁體中文 | `sdk/translate.go`、`engine/opencc/opencc.go` |
| 轉換設定檔 | Conversion Profile | OpenCC 的轉換方向與地區用語規則，字面值如 `s2twp`、`tw2sp` | `engine/opencc/opencc.go` |
| 術語表 | Glossary | 使用者維護的強制替換規則，在引擎輸出後套用 | `glossary/glossary.go` |
| 術語規則 | Glossary Rule | 一組來源文字與目標文字；長詞優先，避免短詞提早覆蓋 | `glossary.Rule`、`glossary.New` |
| 語言標籤 | Language Tag | 描述來源或目標語言與地區變體的輸入值，例如 `zh-TW`、`zh-Hans` | `engine/opencc.variant` |

## 跨語言翻譯 (Cross-language Translation)

| 術語 (Term) | 英文 (English) | 定義 (Definition) | 出處 (Source) |
| --- | --- | --- | --- |
| 翻譯 | Translation | 將文字從一種語言轉成另一種語言 | `sdk/translate.go` |
| 引擎 | Engine | 執行轉換或翻譯的方式，例如行程內字典或本機模型服務 | `engine.Engine` |
| 模型 | Model | 使用者選擇的具名翻譯能力；同一引擎可承載多個模型 | `sdk.Model`、`sdk.Spec` |
| 執行期 | Runtime | 提供模型推論能力的本機服務或行程 | `sdk.Spec.Requires` |
| 翻譯服務 | Translation Service | 獨立運行、以 HTTP 提供 encoder-decoder 模型推論的本機服務；由 `platform/inf` 的 `ct2` 元件擁有，本專案只是客戶端 | `engine/ct2` |
| 句級模型 | Sentence-level Model | 一次只處理一個句子的模型；文件處理會強制一句一批 | `sdk.Spec.SentenceLevel`、`sdk.Translator.maxChars` |
| 健康檢查 | Health Check | 分辨服務未啟動、模型未安裝與模型可用的就緒檢查 | `engine.Engine.Health` |

## 共用文件處理 (Shared Document Processing)

| 術語 (Term) | 英文 (English) | 定義 (Definition) | 出處 (Source) |
| --- | --- | --- | --- |
| 文件 | Document | 保留原始空白與段落結構、可拆成多個翻譯批次的完整輸入 | `segment.Document` |
| 分段 | Segmentation | 將文件拆成可翻譯文字與原始空白，同時保留重組所需的位置資訊 | `segment.Parse` |
| 批次 | Batch | 一次送給引擎的文字單位；輸入與輸出在批次層級對齊 | `sdk.Translator.Batch`、`segment.Document.Batches` |
| 重組 | Reassembly | 將各批譯文接回原始空白與段落位置；數量不一致時明確失敗 | `segment.Document.Reassemble` |

## 模型治理與評估 (Model Operations and Evaluation)

| 術語 (Term) | 英文 (English) | 定義 (Definition) | 出處 (Source) |
| --- | --- | --- | --- |
| Model registry | Model Registry | 模型、引擎、引擎端識別字與 runtime requirement 的單一清單 | `sdk/model.go` |
| Benchmark | Benchmark | 對所有已知模型執行同一組 Sample corpus，保存品質與耗時結果 | `benchmark.Run` |
| Session | Benchmark Session | 一次完整 Benchmark run；每個 Model 各自產生一份結果 | `benchmark.Report`、`benchmark.Write` |
| Sample corpus | Sample Corpus | 固定的 50 句 `zh-Hans → zh-TW` 輸入，確保跨模型比較基準一致 | `benchmark/samples.json`、`benchmark.Samples` |
| Model result | Model Result | 單一模型在一個 Session 內的狀態、輸出、錯誤與耗時 | `benchmark.ModelResult` |

## 狀態值 (Status Values)

| 對象 | 字面值 (Literal) | 語意 | 出處 |
| --- | --- | --- | --- |
| Benchmark Model result | `completed` | 全部 Sample 都已產生 output | `benchmark.StatusCompleted` |
| Benchmark Model result | `unavailable` | runtime、服務或模型檔不可用 | `benchmark.StatusUnavailable` |
| Benchmark Model result | `failed` | 模型可用，但至少一個 Sample translation 失敗 | `benchmark.StatusFailed` |
| Engine readiness | `ready` | 模型與 runtime 均可用 | `cmd/engine.go` |
| Engine readiness | `down` | 健康檢查失敗 | `cmd/engine.go` |
| Engine readiness | `error` | Translator 無法建立或設定無效 | `cmd/engine.go` |

## OpenCC Profile 對照 (OpenCC Profiles)

| Profile | 方向 | 說明 | 出處 |
| --- | --- | --- | --- |
| `s2twp` | 簡體 → 台灣繁體 | 同時轉換字形與台灣地區用語；預設值 | `engine/opencc.DefaultProfile` |
| `s2tw` | 簡體 → 台灣繁體 | 轉換字形，不套用 phrase-level 地區用語 | `engine/opencc.Profiles` |
| `s2t` | 簡體 → 通用繁體 | 不指定地區變體 | `engine/opencc.Profiles` |
| `s2hk` | 簡體 → 香港繁體 | 套用香港變體 | `engine/opencc.Profiles` |
| `tw2sp` | 台灣繁體 → 簡體 | 同時反向轉換地區用語 | `engine/opencc.Profiles` |
| `tw2s`、`hk2s`、`t2s` | 繁體 → 簡體 | 依來源變體轉換 | `engine/opencc.Profiles` |
| `t2tw`、`t2hk` | 通用繁體 → 地區繁體 | 轉成台灣或香港變體 | `engine/opencc.Profiles` |

## 語言標籤 (Language Tags)

輸入標籤會先正規化；大小寫與 `-`／`_` 差異不影響結果。

| 正規化值 | 接受的代表寫法 | 出處 |
| --- | --- | --- |
| `tw` | `zh-TW`、`zh-Hant-TW`、`tw`、`taiwan` | `engine/opencc.variant` |
| `hk` | `zh-HK`、`hk`、`hong kong` | `engine/opencc.variant` |
| `hant` | `zh-Hant`、`繁體` | `engine/opencc.variant` |
| `hans` | `zh-Hans`、`zh-CN`、`zh`、`cn`、`簡體` | `engine/opencc.variant` |

## 縮寫 (Abbreviations)

| 縮寫 | 全稱 | 說明 |
| --- | --- | --- |
| NMT | Neural Machine Translation | 專用的神經機器翻譯模型類型 |
| NLLB | No Language Left Behind | Model registry 中的多語 NMT 模型系列 |
| CT2 | CTranslate2 | 本機 encoder-decoder 模型的推論 runtime |
