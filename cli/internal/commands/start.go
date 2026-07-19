package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/docker"
	"github.com/wotp/cli/internal/keys"
	"github.com/wotp/cli/internal/ui"
)

// NewStartCmd creates the `wotp start` command.
func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the wotp instance",
		Long:  "Pulls Docker images, generates docker-compose.yml, starts containers, and displays connection information.",
		RunE:  runStart,
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load(config.ConfigPath(projectDir))
	if err != nil {
		return err
	}

	ui.Blank()
	ui.Title(fmt.Sprintf("Starting %s...", ui.Brand(cfg.Project.Name)))
	ui.Blank()

	// Render docker-compose.yml from template
	if err := docker.RenderCompose(cfg, projectDir); err != nil {
		return fmt.Errorf("rendering docker-compose: %w", err)
	}

	// Ensure data directories exist
	if err := docker.EnsureDataDirs(projectDir); err != nil {
		return fmt.Errorf("creating data directories: %w", err)
	}

	// Pull images if missing
	imageName := fmt.Sprintf("ghcr.io/raucheacho/wotp:%s", config.AppVersion)
	if docker.HasCoreImageLocally() {
		ui.Success(fmt.Sprintf("Local image %s found, skipping pull", imageName))
	} else {
		stop := ui.Spinner(fmt.Sprintf("Pulling %s...", imageName))
		err = docker.Pull(projectDir)
		stop()
		if err != nil {
			ui.Error("Failed to pull images")
			return fmt.Errorf("pulling images: %w", err)
		}
		ui.Success("Images pulled")
	}

	// Start containers
	stop := ui.Spinner("Starting containers...")
	err = docker.Up(projectDir)
	stop()
	if err != nil {
		ui.Error("Failed to start containers")
		return fmt.Errorf("starting containers: %w", err)
	}
	ui.Success("Container started")

	// Show QR instructions
	ui.Success("Waiting for WhatsApp session...")
	ui.PrintQRInstructions(cfg.API.Port)

	// Read API keys from .env
	anonKey, serviceKey, _, err := keys.ReadEnvFile(config.EnvPath(projectDir))
	if err != nil {
		return fmt.Errorf("reading API keys: %w", err)
	}

	// Print the startup banner
	ui.PrintStartBanner(cfg.API.Port, anonKey, serviceKey)

	return nil
}
