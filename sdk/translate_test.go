package translate

import (
	"context"
	"strings"
	"testing"
)

func TestToTraditionalConvertsVocabulary(t *testing.T) {
	got, err := ToTraditional(context.Background(), "这个软件的视频质量很好。")
	if err != nil {
		t.Fatalf("ToTraditional: %v", err)
	}
	if !strings.Contains(got, "軟體") || !strings.Contains(got, "影片") {
		t.Errorf("地區用語未轉換：%q", got)
	}
}

// 中文互轉一律走 OpenCC：那是查表問題，不該付神經模型的延遲與不確定性。
func TestRoutesChinesePairsToOpenCC(t *testing.T) {
	for _, c := range []struct{ from, to string }{
		{"", "zh-TW"},
		{"zh-Hans", "zh-TW"},
		{"zh-TW", "zh-CN"},
		{"", "tw"},
	} {
		tr, err := New(Options{From: c.from, To: c.to})
		if err != nil {
			t.Fatalf("New(%q→%q): %v", c.from, c.to, err)
		}
		if tr.Model() != ModelOpenCC {
			t.Errorf("%q→%q 選到 %s，want opencc", c.from, c.to, tr.Model())
		}
	}
}

// 引擎的外部相依缺席時，錯誤要說出「該去啟動什麼」，
// 而不是丟一個連線錯誤讓人自己推。
func TestHealthPointsAtTheMissingDependency(t *testing.T) {
	tr, err := New(Options{From: "en", To: "zh-TW", Endpoint: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if tr.Model() != ModelNLLB600M {
		t.Fatalf("跨語言預設應走 NLLB，got %s", tr.Model())
	}

	err = tr.Health(context.Background())
	if err == nil {
		t.Fatal("sidecar 不在時 Health 應失敗")
	}
	if !strings.Contains(err.Error(), "sidecar") {
		t.Errorf("錯誤訊息應指出缺的是 sidecar：%v", err)
	}
}

func TestUnknownModelListsOptions(t *testing.T) {
	_, err := New(Options{Model: "gpt-5", To: "zh-TW"})
	if err == nil {
		t.Fatal("未知模型應報錯")
	}
	if !strings.Contains(err.Error(), "opencc") {
		t.Errorf("錯誤訊息應列出可用模型：%v", err)
	}
}

// 明說模型但語言對不合，也要當場失敗而不是硬翻。
func TestExplicitModelRejectsUnsupportedPair(t *testing.T) {
	if _, err := New(Options{Model: ModelOpenCC, From: "en", To: "ja"}); err == nil {
		t.Fatal("opencc 不該接跨語言請求")
	}
}

// 文件路徑要保住段落：段落是後續筆記與摘要唯一能倚賴的線索。
func TestDocumentPreservesStructure(t *testing.T) {
	in := "第一段的软件。\n\n第二段的视频。\n"

	got, err := ToTraditional(context.Background(), in)
	if err != nil {
		t.Fatalf("ToTraditional: %v", err)
	}
	if got != "第一段的軟體。\n\n第二段的影片。\n" {
		t.Errorf("結構或內容不符：%q", got)
	}
}

func TestTextPassesThroughBlankInput(t *testing.T) {
	tr, err := New(Options{To: "zh-TW"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := tr.Text(context.Background(), "   ")
	if err != nil || got != "   " {
		t.Errorf("空白輸入應原樣回傳，got %q, %v", got, err)
	}
}

func TestModelsListIsStableAndComplete(t *testing.T) {
	models := Models()
	if len(models) != 4 {
		t.Fatalf("模型數 = %d, want 4", len(models))
	}
	for _, m := range models {
		if m.Engine == "" || m.Description == "" {
			t.Errorf("模型 %s 的規格不完整：%+v", m.Model, m)
		}
		// 需要外部服務的模型一定要說出前提，否則 Health 失敗時沒有線索。
		if m.Engine != "opencc" && (m.Requires == "" || m.EngineModel == "") {
			t.Errorf("模型 %s 缺少執行期前提或引擎端模型 id：%+v", m.Model, m)
		}
	}
}

func TestHealthOfOpenCCNeedsNothing(t *testing.T) {
	tr, err := New(Options{To: "zh-TW"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := tr.Health(context.Background()); err != nil {
		t.Errorf("字典內嵌於二進位檔，不該有健康檢查失敗：%v", err)
	}
}

// 專用 NMT 是句級模型：一次餵兩句進去會安靜地漏掉整句。
// 這件事不該由呼叫端判斷，因此 SDK 強制一句一批。
func TestSentenceLevelModelsIgnoreMaxChars(t *testing.T) {
	tr, err := New(Options{Model: ModelNLLB600M, To: "zh-TW", MaxChars: 4000})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := tr.maxChars(); got != 1 {
		t.Errorf("句級模型的批次大小 = %d，want 1（呼叫端設多大都一樣）", got)
	}

	llm, err := New(Options{Model: ModelQwen25, To: "zh-TW", MaxChars: 4000})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := llm.maxChars(); got != 4000 {
		t.Errorf("decoder-only 模型應吃得下長批次，got %d", got)
	}
}

// 字形不可靠的模型要補一次轉換：使用者要 zh-TW 卻拿到简體就是錯的輸出。
// 實測 opus-mt 固定出简體，NLLB 即使指定 zho_Hant 仍會漏字。
func TestScriptNormalizationForUnreliableModels(t *testing.T) {
	for _, m := range []Model{ModelNLLB600M, ModelOpusMT} {
		spec, err := Lookup(m)
		if err != nil {
			t.Fatalf("Lookup(%s): %v", m, err)
		}
		if !spec.NormalizeScript {
			t.Errorf("%s 的中文字形不可靠，應補字形轉換", m)
		}
		if !needsScriptFix(spec, "zh-TW") {
			t.Errorf("%s → zh-TW 應觸發字形轉換", m)
		}
		// 目標不是繁體時不該多此一舉。
		if needsScriptFix(spec, "en") || needsScriptFix(spec, "zh-CN") {
			t.Errorf("%s 對非繁體目標不該補轉換", m)
		}
	}

	// 自己就會出繁體的模型不必補。
	qwen, _ := Lookup(ModelQwen25)
	if needsScriptFix(qwen, "zh-TW") {
		t.Error("qwen 的輸出已是繁體，不該再轉一次")
	}
}
