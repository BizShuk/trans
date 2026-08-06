package cmd

import (
	"github.com/bizshuk/trans/config"
	"github.com/bizshuk/trans/sdk"
	"github.com/spf13/cobra"
)

var (
	textFrom        string
	textTo          string
	textModel       string
	textEndpoint    string
	textEngineModel string
	textPrompt      string
	textInput       string
	textOutput      string
)

var textCmd = &cobra.Command{
	Use:   "text [text]",
	Short: "跨語言翻譯（本機模型，文字不出這台機器）",
	Long: `把文字翻成目標語言。模型未指定時依語言對自動路由：
中文互轉走 OpenCC 字典，其餘走 NLLB。

所有模型都在本機執行：沒有 vendor、沒有帳單、不需要 API 金鑰。
代價是要先把對應的執行期準備好 —— 哪個模型需要什麼，` + "`translate models`" + ` 會說。

輸入來源依序是引數、--input、stdin；輸出預設走 stdout。`,
	Args: cobra.MaximumNArgs(1),
	RunE: runText,
}

func init() {
	textCmd.Flags().StringVar(&textFrom, "from", "", "來源語言，空 = 交給引擎判斷")
	textCmd.Flags().StringVar(&textTo, "to", "", "目標語言（必填），如 zh-TW、en、ja")
	textCmd.Flags().StringVar(&textModel, "model", "", "指定模型，空 = 依語言對自動路由")
	textCmd.Flags().StringVar(&textEndpoint, "endpoint", "", "覆寫引擎端點")
	textCmd.Flags().StringVar(&textEngineModel, "engine-model", "", "覆寫引擎端的模型 id")
	textCmd.Flags().StringVar(&textPrompt, "prompt", "", "附加指示，只有 decoder-only 模型會理它")
	textCmd.Flags().StringVarP(&textInput, "input", "i", "", "輸入檔，空 = stdin")
	textCmd.Flags().StringVarP(&textOutput, "output", "o", "", "輸出檔，空 = stdout")
	textCmd.MarkFlagRequired("to")
	RootCmd.AddCommand(textCmd)
}

func runText(c *cobra.Command, args []string) error {
	text, err := readInput(textInput, args)
	if err != nil {
		return err
	}

	gloss, err := loadGlossary()
	if err != nil {
		return err
	}

	// 先問 SDK 實際會用哪顆模型：端點與批次上限都取決於它，
	// 而在這裡自己複製一份路由規則，遲早會與 SDK 分岔。
	model := sdk.Model(textModel)
	if model == "" {
		model = sdk.Route(textFrom, textTo)
	}

	t, err := sdk.New(sdk.Options{
		Model:       model,
		From:        textFrom,
		To:          textTo,
		Prompt:      textPrompt,
		Endpoint:    firstNonEmpty(textEndpoint, endpointFor(string(model))),
		EngineModel: textEngineModel,
		Timeout:     config.RequestTimeout(),
		MaxChars:    maxCharsFor(string(model)),
		Glossary:    gloss,
	})
	if err != nil {
		return err
	}

	// 先確認引擎活著再送整份文件：一份長文分成幾十批，
	// 在第三十批才發現服務沒開，前面的時間全白花。
	if err := t.Health(c.Context()); err != nil {
		return err
	}

	out, err := t.Document(c.Context(), text)
	if err != nil {
		return err
	}
	return writeOutput(textOutput, out)
}

// endpointFor 依模型挑設定裡的端點：兩種引擎的預設埠不同，
// 讓使用者為了指定端點還要先知道模型跑在哪個引擎上，是多餘的負擔。
func endpointFor(model string) string {
	switch sdk.Model(model) {
	case sdk.ModelQwen25:
		return config.OpenAIEndpoint()
	case sdk.ModelNLLB600M, sdk.ModelOpusMT:
		return config.CT2Endpoint()
	default:
		return ""
	}
}

// maxCharsFor 給 decoder-only 模型較大的批次：它們吃得下更長的上下文，
// 跨句代名詞因此譯得更準。NMT 的上限硬得多。
func maxCharsFor(model string) int {
	switch sdk.Model(model) {
	case sdk.ModelQwen25:
		return config.SegmentMaxCharsLLM()
	default:
		return config.SegmentMaxChars()
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
