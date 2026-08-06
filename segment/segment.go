// Package segment 把一份文件拆成可送去翻譯的批次，並在譯完後接回原本的結構。
//
// 它是純函式，不認識任何引擎 —— 因此換模型不會改變輸出的段落形狀，
// 而段落形狀是後續處理（筆記、摘要）唯一能倚賴的線索。
package segment

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultMaxChars 是單一批次的字元上限。
//
// NMT 模型有 token 上限（NLLB 約 512），超過就會被截斷 —— 而截斷是`安靜的`：
// 回來的譯文讀起來完整，只是後半段不見了。寧可多送幾次。
const DefaultMaxChars = 1200

// OneSentence 讓每個句子自成一批。
//
// 專用 NMT（NLLB、OPUS-MT）是`句級`模型：一次餵兩句進去，實測會整句漏掉
// （NLLB 對 "Ship it before Friday. The gross margin fell…" 只譯了後半），
// 而漏掉的部分不會有任何錯誤訊息。
const OneSentence = 1

// Document 是拆解後的文件。
//
// 它由「可翻譯的文字段」與「原樣保留的分隔」交錯組成。
// 對齊發生在`批次層級`而非句子層級：一個批次整段換成譯文，
// 因此模型把三句合成兩句也不會讓重組錯位。
type Document struct {
	pieces []piece
}

type piece struct {
	// text 是非翻譯片段的原文，原樣輸出。
	text string

	// 翻譯片段拆成三段：core 送去翻譯，前後空白留在這裡。
	//
	// 空白不能跟著 core 一起送：模型（與清洗前綴的後處理）幾乎都會
	// trim 掉頭尾空白，於是換行與縮排在譯文裡安靜消失。把結構留在
	// 這一層，重組就不必仰賴引擎的善意。
	prefix, core, suffix string

	translatable bool
}

// Parse 拆解文件。maxChars 為 0 時使用 DefaultMaxChars。
func Parse(text string, maxChars int) *Document {
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}

	doc := &Document{}
	for _, block := range splitKeepingSeparators(text) {
		if strings.TrimSpace(block) == "" {
			doc.pieces = append(doc.pieces, piece{text: block})
			continue
		}
		for _, batch := range splitBySentence(block, maxChars) {
			doc.pieces = append(doc.pieces, newTranslatable(batch))
		}
	}
	return doc
}

// newTranslatable 把一段內容拆成「前空白 + 內文 + 後空白」。
func newTranslatable(batch string) piece {
	core := strings.TrimSpace(batch)
	if core == "" {
		return piece{text: batch}
	}
	start := strings.Index(batch, core)
	return piece{
		prefix:       batch[:start],
		core:         core,
		suffix:       batch[start+len(core):],
		translatable: true,
	}
}

// Batches 回傳要送去翻譯的字串，順序即為重組順序。
func (d *Document) Batches() []string {
	out := make([]string, 0, len(d.pieces))
	for _, p := range d.pieces {
		if p.translatable {
			out = append(out, p.core)
		}
	}
	return out
}

// Reassemble 把譯文按原結構接回。
//
// 數量不符一律報錯而不是盡力拼湊：少一段譯文代表某次請求靜靜失敗了，
// 硬拼出來的文件會讓人以為一切正常。
func (d *Document) Reassemble(translated []string) (string, error) {
	if want := len(d.Batches()); len(translated) != want {
		return "", fmt.Errorf("segment: 譯文段數 %d 與批次數 %d 不符", len(translated), want)
	}

	var b strings.Builder
	next := 0
	for _, p := range d.pieces {
		if !p.translatable {
			b.WriteString(p.text)
			continue
		}
		// 空白由這一層補回：引擎有沒有 trim 都不影響輸出的結構。
		b.WriteString(p.prefix)
		b.WriteString(translated[next])
		b.WriteString(p.suffix)
		next++
	}
	return b.String(), nil
}

// splitKeepingSeparators 以「內容行」與「空白行」為界切開，兩者都保留。
//
// 空白行是段落邊界，逐字稿的段落是說話者換氣與換題的痕跡，
// 壓平之後就再也回不來。
func splitKeepingSeparators(text string) []string {
	if text == "" {
		return nil
	}

	lines := strings.SplitAfter(text, "\n")
	var out []string
	var current strings.Builder
	currentBlank := strings.TrimSpace(lines[0]) == ""

	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank != currentBlank && current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
		currentBlank = blank
		current.WriteString(line)
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

// splitBySentence 把一段內容切成不超過 maxChars 的批次，句子不切開。
//
// 單句本身超長時只好整句送出：把一個句子從中間剖開，
// 兩半都會被翻得莫名其妙，那比超出上限更糟。
func splitBySentence(block string, maxChars int) []string {
	sentences := sentences(block)

	var out []string
	var current strings.Builder
	size := 0
	for _, s := range sentences {
		// 以 rune 而非 byte 計數：一個中文字三個 byte，
		// 用 byte 算會讓中文的批次只有英文的三分之一長。
		length := utf8.RuneCountInString(s)
		if size > 0 && size+length > maxChars {
			out = append(out, current.String())
			current.Reset()
			size = 0
		}
		current.WriteString(s)
		size += length
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

// sentenceEnders 中英文標點都算：來源常是中英混排的逐字稿。
const sentenceEnders = "。！？!?；;"

// sentences 以標點切句，標點與其後的行內空白留在句尾。
//
// 空白跟著前一句走，重組時才不會把縮排擠到下一句開頭。
func sentences(block string) []string {
	runes := []rune(block)

	var out []string
	start := 0
	for i := 0; i < len(runes); i++ {
		if !strings.ContainsRune(sentenceEnders, runes[i]) && !isASCIIPeriod(runes, i) {
			continue
		}

		end := i + 1
		for end < len(runes) && unicode.IsSpace(runes[end]) && runes[end] != '\n' {
			end++
		}
		out = append(out, string(runes[start:end]))
		start, i = end, end-1
	}

	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	if len(out) == 0 {
		return []string{block}
	}
	return out
}

// isASCIIPeriod 判斷這個句點是否為句尾。
//
// 小數點與縮寫（`3.5`、`U.S.`）不是句尾 —— 在它們身上斷句，
// 會讓數字被拆成兩個批次，而數字正是這條管線最在意的東西。
func isASCIIPeriod(runes []rune, i int) bool {
	if runes[i] != '.' {
		return false
	}
	if i > 0 && unicode.IsDigit(runes[i-1]) && i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
		return false
	}
	if i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) {
		return false
	}
	return true
}
