package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	gosdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/trans/glossary"
	"github.com/spf13/cobra"
)

// glossaryFile 是術語表的預設檔名，放在設定根目錄下。
//
// 它`不`走 viper：術語表是資料而非設定，混進 settings.json
// 只會讓兩者都難改。與 vid-note 的 sources.yaml 同一個道理。
const glossaryFile = "glossary.txt"

var glossaryPath string

var glossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "檢視專案術語表（強制替換規則）",
	Long: `術語表補的是 OpenCC 刻意留下的空白：上游不換有歧義的孤立詞
（計算機 在台灣也可以是計算器、質量 在物理語境就是質量），
但在特定領域裡那些詞只有一種讀法。

格式為每行「原文<TAB>替換」，# 開頭是註解。它在引擎轉換` + "`之後`" + `套用。`,
	Args: cobra.NoArgs,
	RunE: runGlossary,
}

func init() {
	RootCmd.PersistentFlags().StringVar(&glossaryPath, "glossary", "",
		"術語表檔案，空 = ~/.config/trans/"+glossaryFile)
	RootCmd.AddCommand(glossaryCmd)
}

func runGlossary(*cobra.Command, []string) error {
	path := glossaryLocation()
	g, err := glossary.Load(path)
	if err != nil {
		return err
	}
	if g.Len() == 0 {
		fmt.Printf("尚無術語表。建立 %s，每行寫「原文<TAB>替換」即可生效。\n", path)
		return nil
	}

	fmt.Printf("%s（%d 條，長詞優先）\n\n", path, g.Len())
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FROM\tTO")
	for _, r := range g.Rules() {
		fmt.Fprintf(w, "%s\t%s\n", r.From, r.To)
	}
	return w.Flush()
}

// loadGlossary 給各命令共用：載入失敗一律中止，
// 因為術語表存在卻沒被套用，產出看起來完全正常。
func loadGlossary() (*glossary.Glossary, error) {
	return glossary.Load(glossaryLocation())
}

func glossaryLocation() string {
	if glossaryPath != "" {
		return glossaryPath
	}
	return filepath.Join(gosdkconfig.GetAppConfigDir(), glossaryFile)
}
