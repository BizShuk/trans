// Package config 載入 CLI 的設定。
//
// 它`只`服務 CLI：SDK 不讀設定檔，所有可調項都走 Options。
// 反過來的話，任何 import 這個 SDK 的程式都會被迫接受一個全域設定來源，
// 而那是應用程式才該有的權力。
package config

import (
	_ "embed"
	"time"

	gosdkcmdconfig "github.com/bizshuk/gosdk/cmd/config"
	gosdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/spf13/viper"
)

//go:embed example/settings.example.json
var defaultSettingsJSON string

// init 登記內嵌範例, 供 `config default` 複製到設定目錄.
func init() {
	gosdkcmdconfig.MustRegisterDefault("settings.json", []byte(defaultSettingsJSON))
}

// Default 初始化設定；內嵌範例是最低層預設值 (WithDefaultValue), 任何設定檔都蓋得過它。
func Default(options ...gosdkconfig.ConfigOption) {
	options = append(options, gosdkconfig.WithDefaultValue(defaultSettingsJSON))
	gosdkconfig.Default(options...)
}

// DefaultModel 是未指定時使用的模型，空字串代表依語言對自動路由。
func DefaultModel() string { return viper.GetString("default_model") }

// ZHProfile 是中文轉換的預設 OpenCC profile。
func ZHProfile() string { return viper.GetString("zh_profile") }

// ZHTarget 是中文轉換的預設目標。
func ZHTarget() string { return viper.GetString("zh_target") }

// SegmentMaxChars 是 NMT 的批次字元上限。
func SegmentMaxChars() int { return viper.GetInt("segment_max_chars") }

// SegmentMaxCharsLLM 是 decoder-only 模型的批次上限，
// 與 NMT 分開：它們吃得下更長的上下文，跨句代名詞因此譯得更準。
func SegmentMaxCharsLLM() int { return viper.GetInt("segment_max_chars_llm") }

// CT2Endpoint 是 CTranslate2 sidecar 的位址。
func CT2Endpoint() string { return viper.GetString("ct2_endpoint") }

// OpenAIEndpoint 是 ollama／llama-server 的 OpenAI 相容端點。
func OpenAIEndpoint() string { return viper.GetString("openai_endpoint") }

// OpenAIModel 是該端點上的模型 id。
func OpenAIModel() string { return viper.GetString("openai_model") }

// RequestTimeout 是單次請求上限。
func RequestTimeout() time.Duration { return viper.GetDuration("request_timeout") }

// CacheEnabled 決定是否啟用本機轉換快取。
func CacheEnabled() bool { return viper.GetBool("cache_enabled") }
