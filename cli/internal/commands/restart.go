package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/docker"
	"github.com/wotp/cli/internal/keys"
	"github.com/wotp/cli/internal/ui"
)

// NewRestartCmd creates the `wotp restart` command.
func NewRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the wotp instance",
		Long:  "Stops and restarts the containers without losing session data.",
		RunE:  runRestart,
	}
}

func runRestart(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(config.ConfigPath(projectDir))
	if err != nil {
		return err
	}

	ui.Blank()
	ui.Title(fmt.Sprintf("Restarting %s...", ui.Brand(cfg.Project.Name)))
	ui.Blank()

	// Stop
	if docker.IsRunning(projectDir) {
		stop := ui.Spinner("Stopping containers...")
		err = docker.Stop(projectDir)
		stop()
		if err != nil {
			ui.Error("Failed to stop containers")
			return fmt.Errorf("stopping containers: %w", err)
		}
		ui.Success("Containers stopped")
	}

	// Re-render compose (config may have changed)
	if err := docker.RenderCompose(cfg, projectDir); err != nil {
		return fmt.Errorf("rendering docker-compose: %w", err)
	}

	if err := docker.EnsureDataDirs(projectDir); err != nil {
		return fmt.Errorf("creating data directories: %w", err)
	}

	// Start
	stop := ui.Spinner("Starting containers...")
	err = docker.Up(projectDir)
	stop()
	if err != nil {
		ui.Error("Failed to start containers")
		return fmt.Errorf("starting containers: %w", err)
	}
	ui.Success("Container started")

	// Read API keys
	anonKey, serviceKey, _, err := keys.ReadEnvFile(config.EnvPath(projectDir))
	if err != nil {
		return fmt.Errorf("reading API keys: %w", err)
	}

	ui.PrintStartBanner(cfg.API.Port, anonKey, serviceKey)

	return nil
}
