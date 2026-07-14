package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/ui"
)

// NewResetCmd creates the `wotp reset` command.
func NewResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset the WhatsApp session",
		Long:  "Deletes the WhatsApp session data, forcing a new QR code scan on next start. Requires confirmation.",
		RunE:  runReset,
	}
}

func runReset(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	ui.Blank()
	ui.DangerBox("This will delete your WhatsApp session.\nYou will need to scan a new QR code to reconnect.")
	ui.Blank()

	if !ui.ConfirmPrompt("Delete WhatsApp session and restart?") {
		ui.Dim("  Cancelled.")
		ui.Blank()
		return nil
	}

	// Delete session directory
	sessionDir := config.SessionDir(projectDir)
	if err := os.RemoveAll(sessionDir); err != nil {
		return fmt.Errorf("deleting session directory: %w", err)
	}
	ui.Success("WhatsApp session deleted")

	// Restart the containers
	ui.Blank()
	return runRestart(cmd, args)
}
