package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var modelsPullCmd = &cobra.Command{
	Use:   "pull [model]",
	Short: "下載 CTranslate2 模型；不給名稱則列出可下載的清單",
	Long: `下載並安裝 NMT 模型到 ~/.config/trans/models/。

已有 CTranslate2 版本的模型直接抓；只有原始權重的（OPUS-MT）在本地轉一次，
那條路徑需要 PyTorch，指令會在需要時告訴你怎麼裝。

decoder-only 模型（Qwen、MiniCPM）不走這裡 —— 它們由 ollama 管理：
    ollama pull qwen2.5:1.5b`,
	Args: cobra.MaximumNArgs(1),
	RunE: runModelsPull,
}

func init() {
	modelsCmd.AddCommand(modelsPullCmd)
}

func runModelsPull(c *cobra.Command, args []string) error {
	script, python, err := sidecarPaths()
	if err != nil {
		return err
	}

	pullArgs := []string{filepath.Join(script, "pull_model.py")}
	if len(args) == 0 {
		pullArgs = append(pullArgs, "--list")
	} else {
		pullArgs = append(pullArgs, args[0])
	}

	cmd := exec.CommandContext(c.Context(), python, pullArgs...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// sidecarPaths 找出 scripts/ 與該用哪個 python。
//
// 解析順序：已啟用的虛擬環境 → 使用者的共用 ~/.venv → 專案自帶的 .venv → PATH。
// 共用環境排在專案環境`之前`：同一台機器上養兩個都裝著 ctranslate2 的環境，
// 遲早會出現「命令列能跑、sidecar 不能跑」這種只差在誰的 python 的怪事。
func sidecarPaths() (scripts, python string, err error) {
	root, err := repoRoot()
	if err != nil {
		return "", "", err
	}
	scripts = filepath.Join(root, "scripts")

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(os.Getenv("VIRTUAL_ENV"), "bin", "python"),
		filepath.Join(home, ".venv", "bin", "python"),
		filepath.Join(root, ".venv", "bin", "python"),
	}
	for _, candidate := range candidates {
		if candidate == "" || strings.HasPrefix(candidate, string(filepath.Separator)+"bin") {
			continue
		}
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return scripts, candidate, nil
		}
	}

	fallback, lookErr := exec.LookPath("python3")
	if lookErr != nil {
		return "", "", fmt.Errorf("找不到可用的 python；請在虛擬環境中安裝相依：" +
			"uv pip install ctranslate2 transformers sentencepiece fastapi uvicorn huggingface_hub")
	}
	return scripts, fallback, nil
}

// repoRoot 由執行檔位置往上找 scripts/ct2_server.py。
//
// 用執行檔位置而非工作目錄：使用者從任何地方下命令都該能用，
// 而 sidecar 腳本是跟著這份原始碼走的。
func repoRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dir := filepath.Dir(exe)
	for range 6 {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "ct2_server.py")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("找不到 scripts/ct2_server.py；請在 translate 專案目錄下執行，"+
		"或設定 TRANSLATE_HOME（執行檔：%s）", exe)
}
