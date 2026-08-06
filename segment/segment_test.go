package segment

import (
	"strings"
	"testing"
)

// 最重要的性質：不翻譯時，拆了再組必須一字不差。
// 這條守住了，「換模型會不會弄亂排版」就永遠不是問題。
func TestRoundTripIsLossless(t *testing.T) {
	inputs := []string{
		"單句沒有標點",
		"第一句。第二句！第三句？",
		"開頭\n\n中間空行\n\n結尾\n",
		"  前導空白仍在。\n\ttab 也在。\n",
		"混排 English sentence. 然後中文。\n",
		"",
		"\n\n\n",
	}

	for _, in := range inputs {
		doc := Parse(in, 0)
		got, err := doc.Reassemble(doc.Batches())
		if err != nil {
			t.Fatalf("Reassemble(%q): %v", in, err)
		}
		if got != in {
			t.Errorf("round trip 改動了內容\n input: %q\noutput: %q", in, got)
		}
	}
}

// 空行是段落邊界，不能被送去翻譯，也不能消失。
func TestBlankLinesAreNotTranslated(t *testing.T) {
	doc := Parse("第一段。\n\n第二段。\n", 0)

	for _, b := range doc.Batches() {
		if strings.TrimSpace(b) == "" {
			t.Errorf("空白批次被送去翻譯：%q", b)
		}
	}
	if len(doc.Batches()) != 2 {
		t.Fatalf("批次數 = %d, want 2：%q", len(doc.Batches()), doc.Batches())
	}

	// 批次本身`不`帶頭尾空白：模型與輸出清洗幾乎都會 trim，
	// 把換行交給引擎保管，等於把結構押在引擎的善意上。
	for _, b := range doc.Batches() {
		if b != strings.TrimSpace(b) {
			t.Errorf("批次帶著頭尾空白：%q", b)
		}
	}

	got, err := doc.Reassemble([]string{"First.", "Second."})
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if got != "First.\n\nSecond.\n" {
		t.Errorf("段落結構未保留：%q", got)
	}
}

// 真實情境：LLM 引擎會 trim 掉頭尾空白（輸出清洗也會）。
// 重組必須照樣還原結構，否則空行與結尾換行會安靜消失。
func TestReassembleSurvivesTrimmingEngine(t *testing.T) {
	in := "第一段。\n\n第二段。\n"
	doc := Parse(in, 0)

	translated := make([]string, 0, len(doc.Batches()))
	for _, b := range doc.Batches() {
		translated = append(translated, strings.TrimSpace("  "+b+"  \n"))
	}

	got, err := doc.Reassemble(translated)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if got != in {
		t.Errorf("結構未還原\n want %q\n  got %q", in, got)
	}
}

func TestSplitsLongBlockAtSentenceBoundary(t *testing.T) {
	long := strings.Repeat("這是一個句子。", 60) // 每句 7 字
	doc := Parse(long, 100)

	batches := doc.Batches()
	if len(batches) < 2 {
		t.Fatalf("超過上限應切成多批，got %d", len(batches))
	}
	for i, b := range batches {
		if len([]rune(b)) > 100 && i != len(batches)-1 {
			t.Errorf("批次 %d 超出上限：%d 字", i, len([]rune(b)))
		}
		if !strings.HasSuffix(b, "。") {
			t.Errorf("批次 %d 未落在句界：%q", i, b[len(b)-9:])
		}
	}
}

// 單句超長時整句送出：從中間剖開會讓兩半都翻得莫名其妙。
func TestNeverSplitsInsideASentence(t *testing.T) {
	one := strings.Repeat("字", 500) + "。"
	doc := Parse(one, 100)

	if len(doc.Batches()) != 1 {
		t.Fatalf("單句不該被切開，got %d 批", len(doc.Batches()))
	}
}

// 小數點不是句尾。數字被拆到兩個批次，正是這條管線最不能接受的事。
func TestDoesNotBreakOnDecimalPoint(t *testing.T) {
	doc := Parse("毛利率是 3.5 個百分點。下一句。", 0)

	batches := doc.Batches()
	if len(batches) != 1 {
		t.Fatalf("同一段應為一批，got %d", len(batches))
	}
	if !strings.Contains(batches[0], "3.5") {
		t.Errorf("小數被切斷：%q", batches[0])
	}
}

func TestReassembleRejectsCountMismatch(t *testing.T) {
	doc := Parse("一。\n\n二。", 0)

	if _, err := doc.Reassemble([]string{"only one"}); err == nil {
		t.Fatal("段數不符必須報錯：少一段譯文代表某次請求靜靜失敗了")
	}
}

func TestParseUsesDefaultWhenMaxIsZero(t *testing.T) {
	text := strings.Repeat("句子。", 200) // 600 字，低於 DefaultMaxChars
	if got := len(Parse(text, 0).Batches()); got != 1 {
		t.Errorf("預設上限下應為單批，got %d", got)
	}
}
