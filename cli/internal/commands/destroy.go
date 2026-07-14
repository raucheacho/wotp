package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/docker"
	"github.com/wotp/cli/internal/ui"
)

// NewDestroyCmd creates the `wotp destroy` command.
func NewDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the wotp instance",
		Long:  "Removes all containers, volumes, and runtime data. This is a destructive operation requiring double confirmation.",
		RunE:  runDestroy,
	}
}

func runDestroy(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(config.ConfigPath(projectDir))
	if err != nil {
		return err
	}

	ui.Blank()
	ui.DangerBox(fmt.Sprintf(
		"This will permanently destroy the wotp instance '%s'.\n"+
			"All containers, volumes, session data, and runtime files will be removed.\n"+
			"Your config.toml, .env, and seed/ will be preserved.",
		cfg.Project.Name,
	))
	ui.Blank()

	if !ui.DoubleConfirmPrompt(
		"Are you sure you want to destroy this instance?",
		"destroy",
	) {
		ui.Dim("  Cancelled.")
		ui.Blank()
		return nil
	}

	ui.Blank()

	// Stop and remove containers + volumes
	if docker.IsRunning(projectDir) {
		stop := ui.Spinner("Removing containers and volumes...")
		err = docker.Down(projectDir, true)
		stop()
		if err != nil {
			ui.Error("Failed to remove containers")
			return fmt.Errorf("destroying containers: %w", err)
		}
		ui.Success("Containers and volumes removed")
	}

	// Remove the .wotp/ runtime directory
	runtimeDir := config.RuntimeDir(projectDir)
	if err := os.RemoveAll(runtimeDir); err != nil {
		return fmt.Errorf("removing runtime directory: %w", err)
	}
	ui.Success("Runtime directory removed")

	ui.Blank()
	ui.Info("Instance destroyed. Config files preserved in wotp/.")
	ui.Dim("  Run 'wotp start' to recreate the instance.")
	ui.Blank()

	return nil
}
