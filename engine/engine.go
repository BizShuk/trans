// Package engine 定義「把文字變成另一種語言」的實作介面與登錄表。
//
// 一個引擎可能是行程內的字典（opencc）、本機 sidecar（ct2），
// 或本機模型服務（ollama）。呼叫端只認這個介面，換引擎不改任何上層程式碼。
package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Request 是一次轉換請求。
type Request struct {
	Text string

	// From 為空代表交給引擎自行判斷。
	From string
	To   string

	// Profile 是引擎專屬的轉換模式（opencc 的 s2twp 之類）。
	Profile string

	// Prompt 是 decoder-only 模型的系統指示，其他引擎忽略它。
	Prompt string
}

// Result 是一次轉換的結果。
//
// Engine 與 Model 一併回報，因為譯文品質差異大到足以改變決策 ——
// 拿到一份不知道是誰翻的譯文，比拿不到更危險。
type Result struct {
	Text   string
	Engine string
	Model  string
}

// Engine 是一種轉換實作。
type Engine interface {
	Name() string

	// Supports 回報這個語言對能不能處理。回 false 時上層應明確報錯，
	// 不得改派別的引擎 —— 靜默降級會讓使用者不知道譯文出自何處。
	Supports(from, to string) bool

	// Health 檢查引擎是否可用：模型在不在、服務起來沒。
	//
	// 它是介面的一等公民而非附屬品：本機模型九成的失敗是「沒下載」
	// 與「沒啟動」，把這類問題埋進翻譯錯誤裡，等於逼使用者從一句
	// 翻譯失敗訊息反推部署狀態。
	Health(ctx context.Context) error

	Translate(ctx context.Context, req Request) (Result, error)
}

// Config 是建立引擎所需的連線參數。
//
// 行程內的引擎（opencc）整份忽略它；需要外部服務的引擎（ct2、openaiapi）
// 靠它知道要連到哪裡、用哪顆模型。
type Config struct {
	// Endpoint 是服務位址，空字串使用該引擎的預設值。
	Endpoint string

	// Model 是服務端的模型識別字，如 `qwen2.5:1.5b`、`nllb-600m`。
	Model string

	Timeout time.Duration
}

// Factory 由連線參數建立引擎。
//
// 註冊的是工廠而非現成實例：需要外部服務的引擎必須知道端點才能建立，
// 而端點是應用程式的設定，不是套件初始化時就知道的事。
type Factory func(Config) (Engine, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register 註冊引擎工廠。重複註冊同一個名字會 panic ——
// 那是組裝錯誤，不該等到執行期才靜默取用到錯的實作。
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()

	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("engine: %q registered twice", name))
	}
	registry[name] = f
}

// Build 依名稱與參數建立引擎。
func Build(name string, cfg Config) (Engine, error) {
	mu.RLock()
	f, ok := registry[name]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("engine: %q 尚未註冊（已註冊：%v）", name, Names())
	}
	return f(cfg)
}

// Names 回傳所有已註冊的引擎名稱，排序後回傳以便輸出穩定。
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return names()
}

func names() []string {
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
