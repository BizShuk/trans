package cli

import (
	"fmt"
	"time"

	benchmarkpkg "github.com/bizshuk/trans/benchmark"
	"github.com/bizshuk/trans/config"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(newBenchmarkCommand())
}

func newBenchmarkCommand() *cobra.Command {
	var sessionID string
	var outputRoot string

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "以 50 句共用 corpus 比較所有已知 Model",
		Long: `依 Model registry 執行同一組 50 句 Sample corpus。

輸出採 Session → Model → result；不可用的 Model 會保留明確原因，不會 fallback。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBenchmark(cmd, sessionID, outputRoot)
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID（預設使用 UTC timestamp）")
	cmd.Flags().StringVar(&outputRoot, "output", "benchmark/sessions", "Session output root")
	return cmd
}

func runBenchmark(cmd *cobra.Command, sessionID, outputRoot string) error {
	if sessionID == "" {
		sessionID = benchmarkSessionID(time.Now())
	}
	samples, err := benchmarkpkg.Samples()
	if err != nil {
		return err
	}

	report := benchmarkpkg.Run(cmd.Context(), benchmarkpkg.Options{
		SessionID: sessionID,
		Samples:   samples,
		Endpoints: map[string]string{
			"ct2":       config.CT2Endpoint(),
			"openaiapi": config.OpenAIEndpoint(),
		},
		Timeout: config.RequestTimeout(),
	})
	sessionDir, err := benchmarkpkg.Write(outputRoot, report)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), sessionDir)
	return err
}

func benchmarkSessionID(now time.Time) string {
	return now.UTC().Format("20060102T150405.000000000Z")
}
