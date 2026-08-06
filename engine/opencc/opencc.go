// Package opencc 以 OpenCC 字典做中文字形與地區用語轉換。
//
// 它是`規則`引擎而非模型：同一段輸入永遠得到同一段輸出，毫秒級完成，
// 且不會改寫句子。中文簡繁是查表問題，交給神經模型只會換來不確定性。
//
// 這也是本工具存在的理由 —— macOS 內建的 Translation 引擎不支援
// 中文↔中文，mactrans 因此退回 ICU 字形轉換，`計算機` 不會變成 `電腦`。
package opencc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bizshuk/trans/engine"
	lib "github.com/longbridgeapp/opencc"
)

// Name 是這個引擎在登錄表中的名字。
const Name = "opencc"

// DefaultProfile 是未指定時的轉換模式。
//
// s2twp 而非 s2tw：`p` 代表 phrase，會一併換掉地區用語
// （`软件` → `軟體`、`视频` → `影片`）。少了它就只是換筆畫，
// 而那正是我們要補的缺口。
const DefaultProfile = "s2twp"

// Profiles 是 OpenCC 支援的全部轉換模式。
var Profiles = []string{
	"s2t", "s2tw", "s2twp", "s2hk",
	"t2s", "tw2s", "tw2sp", "hk2s",
	"t2tw", "t2hk",
}

// Engine 是 OpenCC 轉換器。轉換器建立成本不低（要載字典），
// 因此依 profile 快取後重複使用。
type Engine struct {
	mu    sync.Mutex
	cache map[string]*lib.OpenCC
}

// New 建立引擎。
func New() *Engine { return &Engine{cache: map[string]*lib.OpenCC{}} }

// 字典內嵌於二進位檔，因此工廠忽略連線參數：這個引擎沒有可連的東西。
func init() {
	engine.Register(Name, func(engine.Config) (engine.Engine, error) { return New(), nil })
}

func (e *Engine) Name() string { return Name }

// Supports 只接受中文互轉。跨語言請走 NMT 或本機模型引擎。
func (e *Engine) Supports(from, to string) bool {
	if !isChinese(to) {
		return false
	}
	return from == "" || isChinese(from)
}

// Health 一律通過：字典內嵌在二進位檔裡，沒有可失敗的外部相依。
func (e *Engine) Health(context.Context) error { return nil }

// Translate 執行轉換。
func (e *Engine) Translate(_ context.Context, req engine.Request) (engine.Result, error) {
	profile := req.Profile
	if profile == "" {
		profile = Profile(req.From, req.To)
	}

	converter, err := e.converter(profile)
	if err != nil {
		return engine.Result{}, err
	}

	out, err := converter.Convert(req.Text)
	if err != nil {
		return engine.Result{}, fmt.Errorf("opencc: convert with %s: %w", profile, err)
	}
	return engine.Result{Text: out, Engine: Name, Model: profile}, nil
}

func (e *Engine) converter(profile string) (*lib.OpenCC, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if c, ok := e.cache[profile]; ok {
		return c, nil
	}
	c, err := lib.New(profile)
	if err != nil {
		return nil, fmt.Errorf("opencc: unknown profile %q（可用：%s）",
			profile, strings.Join(Profiles, ", "))
	}
	e.cache[profile] = c
	return c, nil
}

// Profile 由語言標籤推出轉換模式。
//
// 推不出來時回 DefaultProfile：本工具最常見的用途就是簡體轉台灣繁體，
// 讓最常見的那條路不必每次都指定參數。
func Profile(from, to string) string {
	fromVariant, toVariant := variant(from), variant(to)

	switch toVariant {
	case "tw":
		if fromVariant == "hant" || fromVariant == "hk" {
			return "t2tw"
		}
		return "s2twp"
	case "hk":
		if fromVariant == "hant" || fromVariant == "tw" {
			return "t2hk"
		}
		return "s2hk"
	case "hant":
		return "s2t"
	case "hans":
		switch fromVariant {
		case "tw":
			return "tw2sp"
		case "hk":
			return "hk2s"
		default:
			return "t2s"
		}
	default:
		return DefaultProfile
	}
}

// variant 把語言標籤正規化成 hans / hant / tw / hk。
//
// 使用者會寫 zh-TW、zh-Hant-TW、tw、繁體，一律收斂到同一組值 ——
// 標籤寫法的差異不該變成使用者要記的知識。
func variant(tag string) string {
	t := strings.ToLower(strings.TrimSpace(tag))
	t = strings.NewReplacer("_", "-", "繁體", "hant", "简体", "hans", "簡體", "hans").Replace(t)

	switch {
	case t == "":
		return ""
	case strings.Contains(t, "tw") || strings.Contains(t, "taiwan"):
		return "tw"
	case strings.Contains(t, "hk") || strings.Contains(t, "hong"):
		return "hk"
	case strings.Contains(t, "hant") || t == "zh-t" || t == "t":
		return "hant"
	case strings.Contains(t, "hans") || strings.Contains(t, "cn") || t == "zh-s" || t == "s":
		return "hans"
	case t == "zh":
		return "hans"
	default:
		return ""
	}
}

func isChinese(tag string) bool { return variant(tag) != "" }

// IsTraditional 回報這個語言標籤是否為繁體（含台灣與香港變體）。
//
// 不要拿 Profile 的回傳值來判斷這件事：Profile 對認不出的標籤會回退成
// 預設的 s2twp，於是連 `en` 都會被當成繁體目標。
func IsTraditional(tag string) bool {
	switch variant(tag) {
	case "tw", "hant", "hk":
		return true
	default:
		return false
	}
}
