// Package ct2 以本機的 CTranslate2 翻譯服務做翻譯。
//
// NLLB-200 與 OPUS-MT 都是 encoder-decoder 模型，llama.cpp 不支援這個架構，
// 而 CTranslate2 只有 Python／C++ 綁定 —— 因此推論必須跑在獨立服務裡。
//
// 服務本身`不屬於本專案`：它是 `~/projects/platform/inf` 的 `ct2` 元件，
// 與 funasr 同構，可用容器或原生兩種方式啟動。本套件只是它的客戶端。
// 協定見該元件的 README；請求欄位刻意與 agentsdk `provider.TranslateRequest`
// 對齊，因此同一個服務同時服務本 CLI 與 agent runtime。
package ct2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bizshuk/trans/engine"
)

// Name 是這個引擎在登錄表中的名字。
const Name = "ct2"

const (
	DefaultEndpoint = "http://127.0.0.1:8765"
	DefaultModel    = "nllb-600m"
	DefaultTimeout  = 2 * time.Minute
)

// startHint 是所有連線失敗共用的下一步指引。
//
// 服務移出本專案後，「該去啟動什麼」不再是本地路徑，而是另一個 repo 的
// 指令；錯誤訊息裡不寫清楚的話，使用者會回頭找已經不存在的 scripts/。
const startHint = "請先啟動 inf 的 ct2 元件：" +
	"cd ~/projects/platform/inf && npm run start:ct2（或 npm run serve:ct2 直接跑在 host）"

// Engine 是翻譯服務的客戶端。
type Engine struct {
	endpoint string
	model    string
	client   *http.Client
}

// New 建立引擎。
func New(cfg engine.Config) (engine.Engine, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Engine{
		endpoint: strings.TrimRight(orDefault(cfg.Endpoint, DefaultEndpoint), "/"),
		model:    orDefault(cfg.Model, DefaultModel),
		client:   &http.Client{Timeout: timeout},
	}, nil
}

func init() { engine.Register(Name, New) }

func (e *Engine) Name() string { return Name }

// Supports 要求指定目標語言。NLLB 覆蓋 200 語、OPUS-MT 只有單一語言對，
// 實際支援與否由服務回報 —— 在這裡列白名單只會跟模型清單分岔。
func (e *Engine) Supports(_, to string) bool { return strings.TrimSpace(to) != "" }

// Health 確認服務活著且裝了我們要的模型。
//
// 判斷用 models_available（已下載）而不是 models_loaded（已進記憶體）：
// 模型是延遲載入的，用後者會讓每次冷啟動都被誤判成沒安裝。
func (e *Engine) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.endpoint+"/health", nil)
	if err != nil {
		return err
	}

	res, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("ct2: 連不上 %s；%s：%w", e.endpoint, startHint, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ct2: 服務回應 %s", res.Status)
	}

	var health healthResponse
	if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
		return fmt.Errorf("ct2: 解析健康檢查失敗：%w", err)
	}

	for _, m := range health.ModelsAvailable {
		if m == e.model {
			return nil
		}
	}
	return fmt.Errorf("ct2: 服務上沒有模型 %q；請下載（cd ~/projects/platform/inf && "+
		"npm run pull:ct2 %s）。現有：%s",
		e.model, e.model, strings.Join(health.ModelsAvailable, ", "))
}

// Translate 送一次翻譯請求。
func (e *Engine) Translate(ctx context.Context, req engine.Request) (engine.Result, error) {
	body, err := json.Marshal(translateRequest{
		Text:           req.Text,
		SourceLanguage: req.From,
		TargetLanguage: req.To,
		Model:          e.model,
	})
	if err != nil {
		return engine.Result{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.endpoint+"/v1/translations", bytes.NewReader(body))
	if err != nil {
		return engine.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := e.client.Do(httpReq)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ct2: 請求失敗；%s：%w", startHint, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		// 服務的錯誤訊息帶著模型與語言碼的細節，原樣轉出比包一層有用。
		var detail struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(res.Body).Decode(&detail)
		if detail.Detail != "" {
			return engine.Result{}, fmt.Errorf("ct2: %s", detail.Detail)
		}
		return engine.Result{}, fmt.Errorf("ct2: 服務回應 %s", res.Status)
	}

	var parsed translateResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return engine.Result{}, fmt.Errorf("ct2: 解析回應失敗：%w", err)
	}
	return engine.Result{Text: parsed.Text, Engine: Name, Model: parsed.Model}, nil
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// translateRequest 的欄位名由 ct2 元件的協定決定，與 agentsdk
// `provider.TranslateRequest` 的 JSON tag 一致。
type translateRequest struct {
	Text           string `json:"text"`
	TargetLanguage string `json:"target_language"`
	SourceLanguage string `json:"source_language,omitempty"`
	Model          string `json:"model"`
}

type translateResponse struct {
	Model string `json:"model"`
	Text  string `json:"text"`
}

type healthResponse struct {
	Status          string   `json:"status"`
	ModelsAvailable []string `json:"models_available"`
	ModelsLoaded    []string `json:"models_loaded"`
}
