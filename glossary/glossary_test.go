package glossary

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 長詞優先是這個套件唯一容易寫錯、也最容易安靜出錯的地方：
// 短規則先套用之後，長規則永遠輪不到，而輸出看起來仍然「有換」。
func TestLongerRuleWinsRegardlessOfOrder(t *testing.T) {
	g := New([]Rule{
		{From: "毛利", To: "gross"},
		{From: "毛利率", To: "gross margin"},
	})

	if got := g.Apply("本季毛利率上升"); got != "本季gross margin上升" {
		t.Errorf("長詞未優先：%q", got)
	}
	if got := g.Apply("本季毛利上升"); got != "本季gross上升" {
		t.Errorf("短詞規則應仍然生效：%q", got)
	}
}

// 替換過的字不該被另一條規則再吃一次。
func TestDoesNotCascade(t *testing.T) {
	g := New([]Rule{
		{From: "計算機", To: "電腦"},
		{From: "電腦", To: "computer"},
	})

	if got := g.Apply("計算機"); got != "電腦" {
		t.Errorf("替換結果被二次替換：%q", got)
	}
}

func TestParseTSVWithCommentsAndBlanks(t *testing.T) {
	input := `# 台股法說會用語
計算機	電腦
質量	品質

# 空行與註解都要忽略
內存	記憶體
`
	g, err := Parse(bufio.NewScanner(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if g.Len() != 3 {
		t.Fatalf("規則數 = %d, want 3", g.Len())
	}
	if got := g.Apply("計算機的質量"); got != "電腦的品質" {
		t.Errorf("Apply = %q", got)
	}
}

// 手打時很容易按成空白，為此報錯只是刁難。
func TestParseAcceptsSpaceSeparator(t *testing.T) {
	g, err := Parse(bufio.NewScanner(strings.NewReader("計算機 電腦")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := g.Apply("計算機"); got != "電腦" {
		t.Errorf("Apply = %q", got)
	}
}

func TestParseReportsLineNumber(t *testing.T) {
	_, err := Parse(bufio.NewScanner(strings.NewReader("好的\t行\n壞行\t有\t三欄以上是可以的\n")))
	if err != nil {
		t.Fatalf("三欄應以第一個 TAB 切開，不該報錯：%v", err)
	}

	_, err = Parse(bufio.NewScanner(strings.NewReader("好的\t行\n只有一欄\n")))
	if err == nil {
		t.Fatal("缺少替換欄應報錯")
	}
	if !strings.Contains(err.Error(), "第 2 行") {
		t.Errorf("錯誤訊息要指出行號：%v", err)
	}
}

// 沒有術語表是常態，不是故障。
func TestLoadMissingFileIsNotAnError(t *testing.T) {
	g, err := Load(filepath.Join(t.TempDir(), "nope.txt"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if g.Len() != 0 {
		t.Errorf("不存在的檔案應回空表，got %d 條", g.Len())
	}
	if got := g.Apply("原文"); got != "原文" {
		t.Errorf("空表應原樣回傳：%q", got)
	}
}

func TestLoadFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glossary.txt")
	if err := os.WriteFile(path, []byte("法說會\t法人說明會\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	g, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := g.Apply("今天的法說會"); got != "今天的法人說明會" {
		t.Errorf("Apply = %q", got)
	}
}

func TestNilGlossaryIsUsable(t *testing.T) {
	var g *Glossary
	if got := g.Apply("原文"); got != "原文" {
		t.Errorf("nil 術語表應可安全使用：%q", got)
	}
	if g.Len() != 0 || g.Rules() != nil {
		t.Error("nil 術語表的查詢應回零值")
	}
}
