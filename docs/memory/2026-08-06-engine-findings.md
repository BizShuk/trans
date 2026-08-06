# 2026-08-06 引擎落地時的實測發現

P1–P3 落地過程中，有四件事與設計時的假設不同。它們都不是「寫錯了」，
而是`外部世界與預期不同`，因此值得記下來，免得日後有人重走一次。

## 1. OpenCC 刻意不換有歧義的孤立詞

設計時假設 `s2twp` 會把 `计算机` 換成 `電腦`。實測`不會`：

| 輸入 | s2twp 輸出 | 說明 |
| --- | --- | --- |
| `计算机` | `計算機` | 台灣也可以指計算器，上游不猜 |
| `质量` | `質量` | 物理語境本來就是質量 |
| `计算机科学` | `電腦科學` | 詞組明確，會換 |
| `鼠标`、`打印机`、`软件` | `滑鼠`、`印表機`、`軟體` | 會換 |

這是上游的刻意設計，不是缺陷。`glossary` 套件因此存在 ——
哪些詞在`本專案的領域`裡只有一種讀法，是領域知識，不是字典該猜的事。

上游的 `addition-dictionary` 是`建置期`合併（`merge-data.sh`），
沒有執行期 API，所以替換層必須由我們自己做。

## 2. OPUS-MT 沒有現成的 CTranslate2 版本

查 HuggingFace（2026-08）：

- `entai2965/nllb-200-distilled-600M-ctranslate2` 存在，1125 次下載，可直接抓。
- OPUS-MT（`Helsinki-NLP/opus-mt-*`）只有原始 Marian 權重，
  搜尋 `ctranslate2` 找到的都是別的語言對或別的模型。

因此 `pull_model.py` 分兩種模式：`download`（已轉好）與 `convert`（本地轉一次）。
轉換需要 PyTorch，而那只在這條路徑上需要 —— 所以它不列為必要相依，
只在真的要轉換時才提示安裝。

## 3. Health 必須分辨「服務沒起來」與「模型沒下載」

兩者的修法完全不同：前者是 `ollama serve` 或 `pm2 start`，
後者是 `ollama pull` 或 `translate models pull`。混成一句「連不上」，
使用者會往錯的方向查。兩個 adapter 的 `Health` 都分開報，並直接給出該執行的命令。

`translate engine status` 的實測輸出（尚未下載任何模型時）：

```
minicpm-2b    openaiapi  down  端點上沒有模型 "minicpm-v:8b"；請先下載（ollama pull …）
nllb-600m     ct2        down  連不上 sidecar http://127.0.0.1:8765；請先啟動（…）
opencc        opencc     ready —
```

## 4. 小模型會在譯文前面加話

Qwen／MiniCPM 這類 instruct 模型的通病是熱心：回「以下是翻譯：」或包一層
code fence。那些字會被原封不動寫進檔案。處置分兩層：

1. system prompt 明講 `output ONLY the translation`、不要解釋與引號。
2. `Clean` 再剝一次`已知`的外殼。只剝已知的，不做聰明猜測 ——
   猜錯會把譯文本身削掉一段，而那種損壞讀起來完全正常。

`temperature` 一律 0：翻譯要的是穩定不是創意，同段輸入兩次跑出不同譯文，
會讓下游的 diff 與快取全部失效。

## 5. 四顆跨語言模型的實測對照（M4 / 16GB）

同一句 `The gross margin fell to 13.8% in the second quarter.`，`en → zh-TW`：

| 模型 | 譯文 | 耗時 | 判讀 |
| --- | --- | --- | --- |
| `qwen2.5-1.5b` | 第二季毛利率降至13.8%。 | 3.3s | 唯一把 gross margin 譯對的 |
| `opus-mt-en-zh` | 第二季度,总差幅降至13.8%。 | **0.22s** | 快 30 倍，但出`简體`且術語錯 |
| `nllb-600m` | 在第二季度, 總利潤降至13.8%. | 6.9s | 繁體正確，術語錯 |
| `minicpm-v4.6:1b` | 原文 + 「which can be translated as…」 | 17.3s | 不遵守指示，最慢 |

三個結論：

1. **財經術語上，小型通用模型贏過專用 NMT。** `毛利率` 這種術語需要領域知識，
   而 NLLB／OPUS 的訓練語料裡它就是一般詞。要靠專用 NMT 的話，
   得補 glossary 才能用。
2. **OPUS-MT 的速度是另一個量級**（0.22s vs 3.3s），但它輸出简體 ——
   目標是 zh-TW 時得再串一次 OpenCC。速度差距大到值得為它做這件事。
3. **`minicpm-v4.6:1b` 不適合翻譯。** 它是`視覺`模型（v4.6 系列），
   會把原文連同解釋一起回來，`Clean` 剝不掉這種形態。
   要留 MiniCPM 這個選項，得換一個純文字、指令跟隨較好的 tag。

## 6. 句級模型一次只能吃一句

實測 NLLB 對 `Ship it before Friday. The gross margin fell to 13.8%.`
只譯了後半句，前半句`整句消失`且沒有任何錯誤訊息。改成一句一批之後，
兩句都在，而且更快（11.4s → 1.4s）—— 長輸入的 beam search 成本是超線性的。

因此 `Spec.SentenceLevel` 對 ct2 系模型為真，`Document` 一律一句一批，
呼叫端設多大的 `MaxChars` 都不影響。這不是效能調校，是`正確性`：
餵超過一句給句級模型，會安靜地掉內容。
