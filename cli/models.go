package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	translate "github.com/bizshuk/trans/sdk"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "列出可用的模型與它們所需的引擎",
	Long: `列出每顆模型跑在哪個引擎上、需要什麼執行期前提。

REQUIRES 欄位是 Health 失敗時的第一線索：它直接說出該去啟動什麼。`,
	Args: cobra.NoArgs,
	RunE: runModels,
}

func init() {
	RootCmd.AddCommand(modelsCmd)
}

func runModels(*cobra.Command, []string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL\tENGINE\tREQUIRES\tDESCRIPTION")
	for _, m := range translate.Models() {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Model, m.Engine, orDash(m.Requires), m.Description)
	}
	return w.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
