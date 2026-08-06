package cli

import (
	"github.com/bizshuk/trans/config"
	translate "github.com/bizshuk/trans/sdk"
	"github.com/spf13/cobra"
)

var (
	zhTo      string
	zhProfile string
	zhInput   string
	zhOutput  string
)

var zhCmd = &cobra.Command{
	Use:   "zh [text]",
	Short: "中文字形與地區用語轉換（简体 ↔ 繁體）",
	Long: `以 OpenCC 字典做中文轉換：決定性、毫秒級、不改寫句子，也不需要任何模型。

預設 profile 為 s2twp —— p 代表 phrase，會一併換掉地區用語
（软件 → 軟體、视频 → 影片）。少了它就只是換筆畫。

有歧義的孤立詞（計算機、質量）OpenCC 刻意不換，那需要專案自己的術語表。

輸入來源依序是引數、--input、stdin；輸出預設走 stdout，因此它能站在 pipe 中間。`,
	Args: cobra.MaximumNArgs(1),
	RunE: runZH,
}

func init() {
	zhCmd.Flags().StringVar(&zhTo, "to", "", "目標：tw|cn|hk 或 zh-TW|zh-CN|zh-Hant，空 = 依設定")
	zhCmd.Flags().StringVar(&zhProfile, "profile", "", "直接指定 OpenCC profile，覆寫 --to 的推論")
	zhCmd.Flags().StringVarP(&zhInput, "input", "i", "", "輸入檔，空 = stdin")
	zhCmd.Flags().StringVarP(&zhOutput, "output", "o", "", "輸出檔，空 = stdout")
	RootCmd.AddCommand(zhCmd)
}

func runZH(c *cobra.Command, args []string) error {
	text, err := readInput(zhInput, args)
	if err != nil {
		return err
	}

	target := zhTo
	if target == "" {
		target = config.ZHTarget()
	}
	profile := zhProfile
	if profile == "" && zhTo == "" {
		// 使用者沒指定方向時才套用設定裡的 profile；
		// 明說了 --to 就以 --to 的推論為準，兩個來源不該互相打架。
		profile = config.ZHProfile()
	}

	gloss, err := loadGlossary()
	if err != nil {
		return err
	}

	t, err := translate.New(translate.Options{
		Model:    translate.ModelOpenCC,
		To:       target,
		Profile:  profile,
		MaxChars: config.SegmentMaxChars(),
		Glossary: gloss,
	})
	if err != nil {
		return err
	}

	out, err := t.Document(c.Context(), text)
	if err != nil {
		return err
	}
	return writeOutput(zhOutput, out)
}
