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
	ui.DangerBox("This will delete ALL instance data: every project's WhatsApp session,\nOTP/message history, and API keys. You will need to scan a new QR\ncode and recreate any additional projects afterwards.")
	ui.Blank()

	if !ui.ConfirmPrompt("Delete all data and restart?") {
		ui.Dim("  Cancelled.")
		ui.Blank()
		return nil
	}

	// Delete the entire data directory (control.db + every project's
	// data.db/session.db — wotp-core manages its own layout underneath it,
	// see core/internal/project/registry.go).
	dataDir := config.DataDir(projectDir)
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("deleting data directory: %w", err)
	}
	ui.Success("Instance data deleted")

	// Restart the containers
	ui.Blank()
	return runRestart(cmd, args)
}
