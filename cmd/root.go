// Package cmd 組裝 translate 的命令樹。
//
// 這一層只做旗標解析與輸出格式：任何語言判斷或引擎選擇都屬於 SDK。
// 違反了就會出現「CLI 這樣翻、SDK 那樣翻」的分岔，而那種 bug 極難查。
package cmd

import (
	"fmt"
	"io"
	"os"

	gosdkcmd "github.com/bizshuk/gosdk/cmd"
	"github.com/spf13/cobra"
)

// RootCmd 是 CLI 的根命令。
var RootCmd = &cobra.Command{
	Use:   "translate",
	Short: "語言轉換：中文字形與地區用語、跨語言翻譯",
	Long: `translate 在本機完成兩類工作：

  轉換 (conversion)  同一語言的字形與地區用語互換，如 简体 → 繁體（台灣用語）
  翻譯 (translation) 跨語言，如 en → zh-TW

中文轉換走 OpenCC 字典：決定性、毫秒級、不改寫句子。
跨語言走本機模型，文字不出這台機器，也不需要任何 API 金鑰。`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute 執行根命令。
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.AddCommand(gosdkcmd.ConfigCmd)
}

// readInput 取得輸入：`-i` 指定檔案，否則讀 stdin，
// 讓這個命令能站在 pipe 中間。
func readInput(path string, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("讀取 %s：%w", path, err)
		}
		return string(raw), nil
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("讀取 stdin：%w", err)
	}
	return string(raw), nil
}

// writeOutput 輸出結果：`-o` 指定檔案，否則寫 stdout。
func writeOutput(path, text string) error {
	if path == "" {
		_, err := os.Stdout.WriteString(text)
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return fmt.Errorf("寫入 %s：%w", path, err)
	}
	return nil
}
