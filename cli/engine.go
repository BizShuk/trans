package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bizshuk/trans/config"
	"github.com/bizshuk/trans/sdk"
	"github.com/spf13/cobra"
)

var engineCmd = &cobra.Command{
	Use:   "engine",
	Short: "檢查各模型的執行期是否就緒",
	Long: `逐一對每顆模型做健康檢查：服務起來沒、模型下載了沒。

本機模型的失敗九成是這兩件事，而它們的修法完全不同 ——
因此這裡分開報，並直接給出該執行的命令。`,
	Args: cobra.NoArgs,
	RunE: runEngineStatus,
}

var engineStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "檢查各模型的執行期是否就緒",
	Args:  cobra.NoArgs,
	RunE:  runEngineStatus,
}

func init() {
	engineCmd.AddCommand(engineStatusCmd)
	RootCmd.AddCommand(engineCmd)
}

func runEngineStatus(c *cobra.Command, _ []string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL\tENGINE\tSTATUS\tDETAIL")

	ready := 0
	for _, spec := range sdk.Models() {
		status, detail := probe(c.Context(), spec)
		if status == "ready" {
			ready++
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", spec.Model, spec.Engine, status, detail)
	}
	w.Flush()

	fmt.Printf("\n%d/%d 顆模型就緒\n", ready, len(sdk.Models()))
	return nil
}

func probe(ctx context.Context, spec sdk.Spec) (string, string) {
	t, err := sdk.New(sdk.Options{
		Model:    spec.Model,
		To:       "zh-TW",
		Endpoint: endpointFor(string(spec.Model)),
		Timeout:  config.RequestTimeout(),
	})
	if err != nil {
		return "error", err.Error()
	}
	if err := t.Health(ctx); err != nil {
		return "down", err.Error()
	}
	return "ready", orDash(spec.Requires)
}
