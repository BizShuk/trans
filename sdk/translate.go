// Package sdk 是語言轉換的 SDK：中文字形與地區用語`轉換`，
// 以及跨語言`翻譯`，共用同一組介面。
//
// 這個套件是主體，CLI 只是它的第一個使用者 —— 因此這裡不讀設定檔、
// 不寫 stdout、不呼叫 os.Exit。所有可調項都在 Options 上。
package sdk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bizshuk/trans/engine"
	"github.com/bizshuk/trans/engine/opencc"
	"github.com/bizshuk/trans/glossary"
	"github.com/bizshuk/trans/segment"
)

// Options 是建立轉換器的參數。
type Options struct {
	// Model 為空時依 From／To 自動挑選（中文對走 OpenCC，其餘走 NLLB）。
	Model Model

	// From 為空代表交給引擎自行判斷。
	From string

	// To 是目標語言，如 zh-TW、en、ja。中文轉換也接受 tw／cn／hk。
	To string

	// Profile 覆寫引擎專屬的轉換模式（OpenCC 的 s2twp 之類）。
	Profile string

	// Prompt 是 decoder-only 模型的系統指示，其他引擎忽略它。
	Prompt string

	// MaxChars 是單一批次的字元上限，0 使用 segment 的預設值。
	MaxChars int

	// Endpoint 覆寫引擎的服務位址，空字串使用該引擎的預設值。
	Endpoint string

	// EngineModel 覆寫引擎端的模型識別字（ollama 的 `qwen2.5:1.5b` 之類）。
	EngineModel string

	// Timeout 是單次請求上限，0 使用引擎預設值。
	Timeout time.Duration

	// Glossary 是專案級的強制替換，於引擎轉換`之後`套用。
	// 順序不能反：先替換會讓替換結果再被字典轉一次。
	Glossary *glossary.Glossary
}

// Translator 是一個綁定好模型與語言對的轉換器，可重複使用且併發安全。
type Translator struct {
	engine engine.Engine
	spec   Spec
	opt    Options

	// script 在引擎輸出仍是简體、而目標是繁體時補一次字形轉換。
	script *opencc.Engine
}

// New 建立轉換器。
//
// 模型對應的引擎尚未落地時，在`這裡`就失敗並講清楚它屬於哪個階段 ——
// 讓使用者翻到一半才發現引擎不存在，是最沒必要的浪費。
func New(opt Options) (*Translator, error) {
	model := opt.Model
	if model == "" {
		model = Route(opt.From, opt.To)
	}

	spec, err := Lookup(model)
	if err != nil {
		return nil, err
	}

	e, err := engine.Build(spec.Engine, engine.Config{
		Endpoint: opt.Endpoint,
		Model:    orEmpty(opt.EngineModel, spec.EngineModel),
		Timeout:  opt.Timeout,
	})
	if err != nil {
		return nil, err
	}
	if !e.Supports(opt.From, opt.To) {
		return nil, fmt.Errorf("translate: %s 不支援 %s → %s",
			spec.Model, orAuto(opt.From), opt.To)
	}
	t := &Translator{engine: e, spec: spec, opt: opt}
	if needsScriptFix(spec, opt.To) {
		t.script = opencc.New()
	}
	return t, nil
}

// needsScriptFix 判斷引擎的輸出要不要再過一次字形轉換。
//
// 條件：目標是繁體，而這顆模型的中文字形不可靠。這不是降級也不是改派引擎 ——
// 譯文出自誰沒有變，只是把使用者要的字形補上；不補的話，
// 使用者要求 zh-TW 卻拿到简體，那本身就是錯的輸出。
func needsScriptFix(spec Spec, to string) bool {
	return spec.NormalizeScript && opencc.IsTraditional(to)
}

// Model 回傳實際選用的模型，供呼叫端記錄譯文出處。
func (t *Translator) Model() Model { return t.spec.Model }

// Health 檢查引擎是否可用：模型在不在、服務起來沒。
func (t *Translator) Health(ctx context.Context) error { return t.engine.Health(ctx) }

// Text 轉換一段文字，不做分段。
func (t *Translator) Text(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}

	res, err := t.engine.Translate(ctx, engine.Request{
		Text:    text,
		From:    t.opt.From,
		To:      t.opt.To,
		Profile: t.opt.Profile,
		Prompt:  t.opt.Prompt,
	})
	if err != nil {
		return "", err
	}

	out := res.Text
	if t.script != nil {
		converted, err := t.script.Translate(ctx, engine.Request{Text: out, To: t.opt.To})
		if err != nil {
			return "", err
		}
		out = converted.Text
	}
	// 術語表最後才套用：它是`最終`的說法，不該再被字形轉換動到。
	return t.opt.Glossary.Apply(out), nil
}

// Batch 依序轉換多段文字，回傳順序與輸入一致。
func (t *Translator) Batch(ctx context.Context, texts []string) ([]string, error) {
	out := make([]string, 0, len(texts))
	for i, text := range texts {
		got, err := t.Text(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("translate: 第 %d/%d 段：%w", i+1, len(texts), err)
		}
		out = append(out, got)
	}
	return out, nil
}

// Document 轉換一整份文件：自行斷句、分批、重組，段落結構原樣保留。
//
// 呼叫端不必知道模型的 token 上限 —— 那是實作細節，
// 而超出上限的後果（安靜截斷）不該由呼叫端承擔。
func (t *Translator) Document(ctx context.Context, text string) (string, error) {
	doc := segment.Parse(text, t.maxChars())

	translated, err := t.Batch(ctx, doc.Batches())
	if err != nil {
		return "", err
	}
	return doc.Reassemble(translated)
}

// maxChars 決定批次大小。句級模型一律一句一批 —— 呼叫端設多大都一樣，
// 因為餵超過一句給它們會安靜地漏掉內容，那不是呼叫端能自行判斷的事。
func (t *Translator) maxChars() int {
	if t.spec.SentenceLevel {
		return segment.OneSentence
	}
	return t.opt.MaxChars
}

// ToTraditional 是最常見的那條路：簡體轉台灣繁體，含地區用語。
func ToTraditional(ctx context.Context, text string) (string, error) {
	t, err := New(Options{Model: ModelOpenCC, To: "zh-TW"})
	if err != nil {
		return "", err
	}
	return t.Document(ctx, text)
}

func orEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func orAuto(s string) string {
	if s == "" {
		return "auto"
	}
	return s
}
