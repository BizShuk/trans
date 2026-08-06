package openaiapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bizshuk/trans/engine"
)

// 小模型的通病是熱心：附上「以下是翻譯：」或包一層 code fence，
// 而那些字會被原封不動當成譯文寫進檔案。
func TestCleanStripsKnownWrappers(t *testing.T) {
	cases := map[string]string{
		"以下是翻譯：你好":                  "你好",
		"Translation: Hello":        "Hello",
		"```\n你好\n```":              "你好",
		"```text\n第一行\n第二行\n```":    "第一行\n第二行",
		"  前後空白  ":                  "前後空白",
		"正常譯文":                      "正常譯文",
		"翻譯：這句本身就以翻譯開頭":             "這句本身就以翻譯開頭",
		"這段內容提到 Translation: 但不是前綴": "這段內容提到 Translation: 但不是前綴",
	}

	for in, want := range cases {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

// 指示要把「只回譯文」講死，否則模型會自己加解釋。
func TestSystemPromptForbidsExtras(t *testing.T) {
	prompt := SystemPrompt(engine.Request{From: "en", To: "zh-TW"})

	for _, want := range []string{"ONLY the translation", "en", "zh-TW", "line breaks"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("指示缺少 %q：%s", want, prompt)
		}
	}

	// 呼叫端的附加指示要接在後面，而不是取代掉規則。
	withExtra := SystemPrompt(engine.Request{To: "ja", Prompt: "保留專有名詞原文"})
	if !strings.Contains(withExtra, "保留專有名詞原文") || !strings.Contains(withExtra, "ONLY the translation") {
		t.Errorf("附加指示應疊加而非覆蓋：%s", withExtra)
	}
}

func TestTranslateSendsZeroTemperature(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "以下是翻譯：你好"}},
			},
		})
	}))
	defer srv.Close()

	e, err := New(engine.Config{Endpoint: srv.URL, Model: "test-model"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := e.Translate(context.Background(), engine.Request{Text: "Hello", To: "zh-TW"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if got.Text != "你好" {
		t.Errorf("譯文未清洗：%q", got.Text)
	}
	if got.Model != "test-model" {
		t.Errorf("Model = %q，譯文出處必須跟著結果走", got.Model)
	}
	// 翻譯要的是穩定不是創意：同段輸入兩次跑出不同譯文，
	// 會讓下游的 diff 與快取全部失效。
	if temp, _ := captured["temperature"].(float64); temp != 0 {
		t.Errorf("temperature = %v, want 0", captured["temperature"])
	}
	if stream, _ := captured["stream"].(bool); stream {
		t.Error("不該開串流：我們要的是完整結果")
	}
}

// 服務沒起來與模型沒下載的修法完全不同，錯誤訊息必須分得開。
func TestHealthDistinguishesDownFromMissingModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "other-model"}},
		})
	}))
	defer srv.Close()

	e, _ := New(engine.Config{Endpoint: srv.URL, Model: "wanted"})
	err := e.Health(context.Background())
	if err == nil {
		t.Fatal("模型不在時應失敗")
	}
	if !strings.Contains(err.Error(), "ollama pull wanted") {
		t.Errorf("錯誤訊息要給出修法：%v", err)
	}
	if !strings.Contains(err.Error(), "other-model") {
		t.Errorf("錯誤訊息要列出現有模型：%v", err)
	}

	down, _ := New(engine.Config{Endpoint: "http://127.0.0.1:1"})
	err = down.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "ollama serve") {
		t.Errorf("服務不在時要說該啟動什麼：%v", err)
	}
}

func TestHealthPassesWhenModelPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "qwen2.5:1.5b"}},
		})
	}))
	defer srv.Close()

	e, _ := New(engine.Config{Endpoint: srv.URL, Model: "qwen2.5:1.5b"})
	if err := e.Health(context.Background()); err != nil {
		t.Errorf("模型在就該通過：%v", err)
	}
}

func TestDefaultsPointAtOllama(t *testing.T) {
	e, err := New(engine.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	impl := e.(*Engine)
	if impl.endpoint != DefaultEndpoint || impl.model != DefaultModel {
		t.Errorf("預設值 = %s / %s，want %s / %s", impl.endpoint, impl.model, DefaultEndpoint, DefaultModel)
	}
}

func TestSupportsNeedsTarget(t *testing.T) {
	e, _ := New(engine.Config{})
	if !e.Supports("en", "zh-TW") {
		t.Error("指定目標語言就該支援")
	}
	if e.Supports("en", "  ") {
		t.Error("沒有目標語言時不該宣稱支援")
	}
}
