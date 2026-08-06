// Package ct2 以本機的 CTranslate2 sidecar 做翻譯。
//
// NLLB-200 與 OPUS-MT 都是 encoder-decoder 模型，llama.cpp 不支援這個架構，
// 而 CTranslate2 只有 Python／C++ 綁定 —— 因此 sidecar 是本專案唯一的
// 非 Go 成分（scripts/ct2_server.py）。契約由本專案定義，見 §4.4 設計說明。
//
// 不直接用 LibreTranslate 的理由：它自帶 web UI、API key 管理與 argos
// 套件系統，而我們要的只是「送一段字、回一段字」。多帶的東西都要跟著
// 升級、跟著設定、跟著出事。
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

// Engine 是 sidecar 的客戶端。
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
// 實際支援與否由 sidecar 回報 —— 在這裡列白名單只會跟模型清單分岔。
func (e *Engine) Supports(_, to string) bool { return strings.TrimSpace(to) != "" }

// Health 確認 sidecar 活著且載入了我們要的模型。
func (e *Engine) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.endpoint+"/health", nil)
	if err != nil {
		return err
	}

	res, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("ct2: 連不上 sidecar %s；請先啟動（pm2 start ecosystem.config.js 或 "+
			"scripts/ct2_server.sh）：%w", e.endpoint, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ct2: sidecar 回應 %s", res.Status)
	}

	var health struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
		return fmt.Errorf("ct2: 解析健康檢查失敗：%w", err)
	}

	for _, m := range health.Models {
		if m == e.model {
			return nil
		}
	}
	return fmt.Errorf("ct2: sidecar 上沒有模型 %q；請先下載（translate models pull %s）。現有：%s",
		e.model, e.model, strings.Join(health.Models, ", "))
}

// Translate 送一次翻譯請求。
func (e *Engine) Translate(ctx context.Context, req engine.Request) (engine.Result, error) {
	body, err := json.Marshal(translateRequest{
		Text:   req.Text,
		Source: req.From,
		Target: req.To,
		Model:  e.model,
	})
	if err != nil {
		return engine.Result{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.endpoint+"/translate", bytes.NewReader(body))
	if err != nil {
		return engine.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := e.client.Do(httpReq)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ct2: 請求失敗：%w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		// sidecar 的錯誤訊息帶著模型與語言碼的細節，原樣轉出比包一層有用。
		var detail struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(res.Body).Decode(&detail)
		if detail.Detail != "" {
			return engine.Result{}, fmt.Errorf("ct2: %s", detail.Detail)
		}
		return engine.Result{}, fmt.Errorf("ct2: sidecar 回應 %s", res.Status)
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

type translateRequest struct {
	Text   string `json:"text"`
	Source string `json:"source,omitempty"`
	Target string `json:"target"`
	Model  string `json:"model"`
}

type translateResponse struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}
