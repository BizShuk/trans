// Package openaiapi 以本機的 OpenAI 相容端點做翻譯。
//
// ollama 與 llama-server 講同一套 /v1/chat/completions，因此一個 adapter
// 打兩個 runtime —— 差別只在 base URL 與模型 id。
//
// 它跑的是本機模型：文字不出這台機器、沒有 vendor、沒有帳單。
package openaiapi

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
const Name = "openaiapi"

// 預設值指向 ollama 的慣用埠；llama-server 只要換 endpoint 就能用。
const (
	DefaultEndpoint = "http://127.0.0.1:11434/v1"
	DefaultModel    = "qwen2.5:1.5b"
	DefaultTimeout  = 2 * time.Minute
)

// Engine 是 OpenAI 相容端點的客戶端。
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

// Supports 只要求指定目標語言：小模型的語言覆蓋由模型本身決定，
// 這一層無從得知，硬列白名單只會擋掉實際上做得到的組合。
func (e *Engine) Supports(_, to string) bool { return strings.TrimSpace(to) != "" }

// Health 確認服務活著且掛著我們要的模型。
//
// 這兩件事分開報：服務沒起來與模型沒下載的修法完全不同，
// 混成一句「連不上」會讓人往錯的方向查。
func (e *Engine) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.endpoint+"/models", nil)
	if err != nil {
		return err
	}

	res, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("openaiapi: 連不上 %s；請先啟動服務（ollama serve）：%w", e.endpoint, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("openaiapi: %s 回應 %s", e.endpoint, res.Status)
	}

	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		return fmt.Errorf("openaiapi: 解析模型清單失敗：%w", err)
	}

	available := make([]string, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == e.model {
			return nil
		}
		available = append(available, m.ID)
	}
	return fmt.Errorf("openaiapi: 端點上沒有模型 %q；請先下載（ollama pull %s）。現有：%s",
		e.model, e.model, strings.Join(available, ", "))
}

// Translate 送一次 chat completion。
func (e *Engine) Translate(ctx context.Context, req engine.Request) (engine.Result, error) {
	body, err := json.Marshal(chatRequest{
		Model: e.model,
		// 溫度歸零：翻譯要的是穩定，不是創意。同一段輸入兩次跑出不同譯文，
		// 會讓下游的 diff 與快取全部失效。
		Temperature: 0,
		Stream:      false,
		Messages: []message{
			{Role: "system", Content: SystemPrompt(req)},
			{Role: "user", Content: req.Text},
		},
	})
	if err != nil {
		return engine.Result{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return engine.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := e.client.Do(httpReq)
	if err != nil {
		return engine.Result{}, fmt.Errorf("openaiapi: 請求失敗：%w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return engine.Result{}, fmt.Errorf("openaiapi: %s 回應 %s", e.endpoint, res.Status)
	}

	var parsed chatResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return engine.Result{}, fmt.Errorf("openaiapi: 解析回應失敗：%w", err)
	}
	if len(parsed.Choices) == 0 {
		return engine.Result{}, fmt.Errorf("openaiapi: 回應沒有任何內容")
	}

	return engine.Result{
		Text:   Clean(parsed.Choices[0].Message.Content),
		Engine: Name,
		Model:  e.model,
	}, nil
}

// SystemPrompt 組出系統指示。
//
// 三件事寫死在指示裡：只回譯文、保留格式、不加解釋。小模型的通病
// 就是熱心 —— 附上「以下是翻譯：」或補一段說明，而那些字會被原封不動
// 當成譯文寫進檔案。
func SystemPrompt(req engine.Request) string {
	var b strings.Builder
	b.WriteString("You are a translation engine. Translate the user's text")
	if req.From != "" {
		fmt.Fprintf(&b, " from %s", req.From)
	}
	fmt.Fprintf(&b, " into %s.\n", req.To)
	b.WriteString("Rules: output ONLY the translation; keep the original line breaks and " +
		"punctuation style; do not add explanations, notes, quotes, or prefixes; " +
		"keep numbers, units and proper nouns unchanged.")

	if req.Prompt != "" {
		b.WriteString("\n")
		b.WriteString(req.Prompt)
	}
	return b.String()
}

// prefixes 是小模型最常見的熱心前綴。
var prefixes = []string{
	"以下是翻譯：", "以下是翻译：", "翻譯：", "翻译：", "譯文：", "译文：",
	"Translation:", "Translated text:", "Here is the translation:",
	"Sure, here is the translation:",
}

// Clean 去掉模型多加的包裝。
//
// 只剝`已知`的外殼，不做聰明的猜測：猜錯會把譯文本身削掉一段，
// 而那種損壞讀起來完全正常。
func Clean(s string) string {
	out := strings.TrimSpace(s)

	// 整段被 ``` 包起來時剝掉圍欄，但保留內容裡的換行。
	if strings.HasPrefix(out, "```") {
		if idx := strings.Index(out, "\n"); idx >= 0 {
			out = out[idx+1:]
		}
		out = strings.TrimSuffix(strings.TrimSpace(out), "```")
		out = strings.TrimSpace(out)
	}

	for _, p := range prefixes {
		if trimmed, ok := strings.CutPrefix(out, p); ok {
			out = strings.TrimSpace(trimmed)
			break
		}
	}
	return out
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}
