package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/docker"
	"github.com/wotp/cli/internal/keys"
	"github.com/wotp/cli/internal/ui"
)

// NewUpdateCmd creates the `wotp update` command.
func NewUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update to the latest wotp image",
		Long:  "Pulls the latest Docker images and restarts the containers.",
		RunE:  runUpdate,
	}
}

func runUpdate(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(config.ConfigPath(projectDir))
	if err != nil {
		return err
	}

	ui.Blank()
	ui.Title(fmt.Sprintf("Updating %s...", ui.Brand(cfg.Project.Name)))
	ui.Blank()

	// Re-render compose (in case template changed)
	if err := docker.RenderCompose(cfg, projectDir); err != nil {
		return fmt.Errorf("rendering docker-compose: %w", err)
	}

	// Pull latest images
	stop := ui.Spinner("Pulling latest images...")
	err = docker.Pull(projectDir)
	stop()
	if err != nil {
		ui.Error("Failed to pull images")
		return fmt.Errorf("pulling images: %w", err)
	}
	ui.Success("Images updated")

	// Stop existing containers
	if docker.IsRunning(projectDir) {
		stop = ui.Spinner("Stopping containers...")
		err = docker.Stop(projectDir)
		stop()
		if err != nil {
			ui.Error("Failed to stop containers")
			return fmt.Errorf("stopping containers: %w", err)
		}
		ui.Success("Containers stopped")
	}

	// Start with new images
	stop = ui.Spinner("Starting containers...")
	err = docker.Up(projectDir)
	stop()
	if err != nil {
		ui.Error("Failed to start containers")
		return fmt.Errorf("starting containers: %w", err)
	}
	ui.Success("Container started")

	// Read API keys
	anonKey, serviceKey, err := keys.ReadEnvFile(config.EnvPath(projectDir))
	if err != nil {
		return fmt.Errorf("reading API keys: %w", err)
	}

	ui.PrintStartBanner(cfg.API.Port, anonKey, serviceKey)

	return nil
}
