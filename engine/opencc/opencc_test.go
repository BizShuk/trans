package opencc

import (
	"context"
	"testing"

	"github.com/bizshuk/trans/engine"
)

// 這是本工具存在的理由：mactrans 走 ICU 字形轉換，只換筆畫；
// 我們要的是`地區用語`。哪天字典換版讓它退回字形轉換，這裡必須當場紅。
func TestConvertsVocabularyNotJustGlyphs(t *testing.T) {
	cases := map[string]string{
		"软件":     "軟體",
		"视频":     "影片",
		"内存":     "記憶體",
		"网络":     "網路",
		"这个项目":   "這個專案",
		"计算机科学":  "電腦科學",
		"鼠标和硬盘":  "滑鼠和硬碟",
		"打印机没反应": "印表機沒反應",
	}

	e := New()
	for in, want := range cases {
		got, err := e.Translate(context.Background(), engine.Request{Text: in, To: "zh-TW"})
		if err != nil {
			t.Fatalf("Translate(%q): %v", in, err)
		}
		if got.Text != want {
			t.Errorf("Translate(%q) = %q, want %q（只換筆畫不算通過）", in, got.Text, want)
		}
	}
}

// OpenCC 刻意`不`換有歧義的孤立詞：`計算機` 在台灣也可以是計算器，
// `質量` 在物理語境就是質量。這條界線釘在測試裡，免得日後有人以為
// s2twp 壞了 —— 這類詞要換得靠專案自己的 glossary（見 README.todo）。
func TestLeavesAmbiguousStandaloneTermsAlone(t *testing.T) {
	e := New()
	for in, want := range map[string]string{
		"计算机": "計算機",
		"质量":  "質量",
	} {
		got, err := e.Translate(context.Background(), engine.Request{Text: in, To: "zh-TW"})
		if err != nil {
			t.Fatalf("Translate(%q): %v", in, err)
		}
		if got.Text != want {
			t.Errorf("Translate(%q) = %q, want %q（字典行為改變，請確認是否仍需 glossary）", in, got.Text, want)
		}
	}
}

func TestReportsEngineAndProfile(t *testing.T) {
	got, err := New().Translate(context.Background(), engine.Request{Text: "软件", To: "zh-TW"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	// 譯文出自誰必須跟著結果走，否則沒人查得出品質差異的來源。
	if got.Engine != Name || got.Model != "s2twp" {
		t.Errorf("engine=%q model=%q，want opencc/s2twp", got.Engine, got.Model)
	}
}

func TestProfileRouting(t *testing.T) {
	cases := []struct {
		from, to string
		want     string
	}{
		{to: "zh-TW", want: "s2twp"},
		{to: "tw", want: "s2twp"},
		{from: "zh-Hans", to: "zh-TW", want: "s2twp"},
		{from: "zh-Hant", to: "zh-TW", want: "t2tw"},
		{from: "zh-TW", to: "zh-CN", want: "tw2sp"},
		{to: "zh-Hant", want: "s2t"},
		{to: "zh-HK", want: "s2hk"},
		{from: "zh-TW", to: "zh-HK", want: "t2hk"},
		{to: "", want: DefaultProfile},
	}

	for _, c := range cases {
		if got := Profile(c.from, c.to); got != c.want {
			t.Errorf("Profile(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

// 標籤寫法的差異不該變成使用者要記的知識。
func TestAcceptsTagSpellings(t *testing.T) {
	for _, tag := range []string{"zh-TW", "zh_TW", "zh-Hant-TW", "tw", "TW", " zh-tw "} {
		if got := Profile("", tag); got != "s2twp" {
			t.Errorf("Profile(_, %q) = %q, want s2twp", tag, got)
		}
	}
}

func TestSupportsOnlyChinese(t *testing.T) {
	e := New()
	if !e.Supports("", "zh-TW") || !e.Supports("zh-Hans", "zh-TW") {
		t.Error("中文互轉應被支援")
	}
	if e.Supports("en", "zh-TW") || e.Supports("", "ja") {
		t.Error("跨語言不該由 opencc 承接：它只會查表，不會翻譯")
	}
}

func TestExplicitProfileWins(t *testing.T) {
	got, err := New().Translate(context.Background(), engine.Request{
		Text: "電腦", To: "zh-TW", Profile: "tw2sp",
	})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if got.Text != "电脑" {
		t.Errorf("明說的 profile 應優先於推論，got %q", got.Text)
	}
}

func TestUnknownProfileFailsWithOptions(t *testing.T) {
	_, err := New().Translate(context.Background(), engine.Request{
		Text: "软件", Profile: "s2xx",
	})
	if err == nil {
		t.Fatal("未知 profile 應報錯")
	}
	if want := "s2twp"; !contains(err.Error(), want) {
		t.Errorf("錯誤訊息應列出可用值，got %q", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
