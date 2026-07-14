package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/docker"
	"github.com/wotp/cli/internal/ui"
)

// NewStopCmd creates the `wotp stop` command.
func NewStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the wotp instance",
		Long:  "Gracefully stops containers without removing them or their volumes/session data.",
		RunE:  runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(config.ConfigPath(projectDir))
	if err != nil {
		return err
	}

	ui.Blank()
	stop := ui.Spinner(fmt.Sprintf("Stopping %s...", cfg.Project.Name))
	err = docker.Stop(projectDir)
	stop()
	if err != nil {
		ui.Error("Failed to stop containers")
		return fmt.Errorf("stopping containers: %w", err)
	}

	ui.Success("Containers stopped")
	ui.Dim("  Session and data preserved. Run 'wotp start' to resume.")
	ui.Blank()

	return nil
}
