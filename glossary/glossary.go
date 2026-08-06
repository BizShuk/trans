// Package glossary 是專案級的強制替換規則。
//
// 它補的是 OpenCC 刻意留下的空白：上游不換有歧義的孤立詞
// （`計算機` 在台灣也可以是計算器、`質量` 在物理語境就是質量），
// 但在特定領域裡那些詞只有一種讀法。哪些詞該強制替換是`領域知識`，
// 不是字典該猜的事，因此由使用者自己維護。
//
// 上游的 addition-dictionary 是`建置期`合併，沒有執行期 API，
// 所以這一層必須由本專案自己做。
package glossary

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Rule 是一條替換規則。
type Rule struct {
	From string
	To   string
}

// Glossary 是一組已編譯好的替換規則，可重複使用且併發安全。
type Glossary struct {
	rules    []Rule
	replacer *strings.Replacer
}

// New 由規則建立術語表。
//
// 規則依來源長度`由長到短`排序後才編譯：strings.Replacer 在同一個位置
// 取用第一個對得上的樣式，不排序的話 `毛利` 會先吃掉 `毛利率`，
// 讓長詞的規則永遠輪不到。
func New(rules []Rule) *Glossary {
	sorted := append([]Rule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return len([]rune(sorted[i].From)) > len([]rune(sorted[j].From))
	})

	pairs := make([]string, 0, len(sorted)*2)
	for _, r := range sorted {
		if r.From == "" {
			continue
		}
		pairs = append(pairs, r.From, r.To)
	}
	return &Glossary{rules: sorted, replacer: strings.NewReplacer(pairs...)}
}

// Apply 套用替換。空表時原樣回傳。
func (g *Glossary) Apply(text string) string {
	if g == nil || len(g.rules) == 0 {
		return text
	}
	return g.replacer.Replace(text)
}

// Rules 回傳排序後的規則，供 CLI 檢視。
func (g *Glossary) Rules() []Rule {
	if g == nil {
		return nil
	}
	return append([]Rule(nil), g.rules...)
}

// Len 回傳規則數。
func (g *Glossary) Len() int {
	if g == nil {
		return 0
	}
	return len(g.rules)
}

// Parse 讀取 TSV 格式的術語表：每行 `原文<TAB>替換`，`#` 開頭為註解。
//
// 用 TSV 而非 YAML，是為了與 OpenCC 字典同一種格式 ——
// 使用者從上游字典複製一行過來就能用，不必再學一套結構。
func Parse(r *bufio.Scanner) (*Glossary, error) {
	var rules []Rule
	line := 0

	for r.Scan() {
		line++
		text := strings.TrimSpace(r.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		parts := strings.SplitN(text, "\t", 2)
		if len(parts) != 2 {
			// 空白分隔也接受：手打時很容易按成空白，為此報錯只是刁難。
			parts = strings.Fields(text)
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("glossary: 第 %d 行格式錯誤，需要「原文<TAB>替換」：%q", line, text)
		}
		rules = append(rules, Rule{From: parts[0], To: parts[1]})
	}
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("glossary: read: %w", err)
	}
	return New(rules), nil
}

// Load 從檔案讀取術語表。檔案不存在時回 nil 而非錯誤 ——
// 沒有術語表是常態，不是故障。
func Load(path string) (*Glossary, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("glossary: open %s: %w", path, err)
	}
	defer f.Close()

	g, err := Parse(bufio.NewScanner(f))
	if err != nil {
		return nil, fmt.Errorf("%w（檔案：%s）", err, path)
	}
	return g, nil
}
