package ct2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bizshuk/trans/engine"
)

func TestTranslateSendsContract(t *testing.T) {
	var captured map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/translate" {
			t.Errorf("path = %s, want /translate", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		json.NewEncoder(w).Encode(map[string]string{"text": "你好", "model": "nllb-600m"})
	}))
	defer srv.Close()

	e, err := New(engine.Config{Endpoint: srv.URL, Model: "nllb-600m"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := e.Translate(context.Background(), engine.Request{
		Text: "Hello", From: "en", To: "zh-TW",
	})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if got.Text != "你好" || got.Model != "nllb-600m" || got.Engine != Name {
		t.Errorf("結果 = %+v", got)
	}
	if captured["text"] != "Hello" || captured["source"] != "en" ||
		captured["target"] != "zh-TW" || captured["model"] != "nllb-600m" {
		t.Errorf("送出的請求 = %+v", captured)
	}
}

// sidecar 的錯誤帶著模型與語言碼的細節，原樣轉出比包一層有用。
func TestTranslateSurfacesSidecarDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"detail": "NLLB 沒有 xx-YY 的語言碼",
		})
	}))
	defer srv.Close()

	e, _ := New(engine.Config{Endpoint: srv.URL})
	_, err := e.Translate(context.Background(), engine.Request{Text: "x", To: "xx-YY"})
	if err == nil {
		t.Fatal("400 應回報錯誤")
	}
	if !strings.Contains(err.Error(), "語言碼") {
		t.Errorf("sidecar 的細節被吞掉了：%v", err)
	}
}

// 連不上 sidecar 與模型沒安裝，修法完全不同。
func TestHealthDistinguishesDownFromMissingModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []string{"opus-mt-en-zh"}})
	}))
	defer srv.Close()

	e, _ := New(engine.Config{Endpoint: srv.URL, Model: "nllb-600m"})
	err := e.Health(context.Background())
	if err == nil {
		t.Fatal("模型不在時應失敗")
	}
	if !strings.Contains(err.Error(), "translate models pull nllb-600m") {
		t.Errorf("錯誤訊息要給出修法：%v", err)
	}
	if !strings.Contains(err.Error(), "opus-mt-en-zh") {
		t.Errorf("錯誤訊息要列出現有模型：%v", err)
	}

	down, _ := New(engine.Config{Endpoint: "http://127.0.0.1:1"})
	err = down.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Errorf("服務不在時要說該啟動什麼：%v", err)
	}
}

func TestHealthPassesWhenModelInstalled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []string{"nllb-600m"}})
	}))
	defer srv.Close()

	e, _ := New(engine.Config{Endpoint: srv.URL, Model: "nllb-600m"})
	if err := e.Health(context.Background()); err != nil {
		t.Errorf("模型在就該通過：%v", err)
	}
}

func TestDefaults(t *testing.T) {
	e, err := New(engine.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	impl := e.(*Engine)
	if impl.endpoint != DefaultEndpoint || impl.model != DefaultModel {
		t.Errorf("預設 = %s / %s，want %s / %s", impl.endpoint, impl.model, DefaultEndpoint, DefaultModel)
	}
}
