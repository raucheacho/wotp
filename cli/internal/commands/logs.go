package commands

import (
	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/docker"
	"github.com/wotp/cli/internal/ui"
)

// NewLogsCmd creates the `wotp logs` command.
func NewLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "Stream container logs",
		Long:  "Streams logs from the wotp-core container in real-time.",
		RunE:  runLogs,
	}
}

func runLogs(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	if !docker.IsRunning(projectDir) {
		ui.Blank()
		ui.Error("No running containers found.")
		ui.Dim("  Run 'wotp start' first.")
		ui.Blank()
		return nil
	}

	return docker.Logs(projectDir)
}
