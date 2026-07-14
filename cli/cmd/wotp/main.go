// wotp CLI — WhatsApp OTP, self-hosted, one command.
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/commands"
)

var version = "1.0.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "wotp",
		Short: "Wotp CLI — WhatsApp OTP, self-hosted, one command",
		Long: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#25D366")).Render("wotp") +
			" — Manage your self-hosted WhatsApp OTP instance.\n\n" +
			"Get started:\n" +
			"  wotp init my-project\n" +
			"  cd my-project\n" +
			"  wotp start",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Register all commands
	rootCmd.AddCommand(
		commands.NewInitCmd(),
		commands.NewStartCmd(),
		commands.NewStopCmd(),
		commands.NewStatusCmd(),
		commands.NewLogsCmd(),
		commands.NewRestartCmd(),
		commands.NewResetCmd(),
		commands.NewKeysCmd(),
		commands.NewUpdateCmd(),
		commands.NewDestroyCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %s\n\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true).Render("✗"),
			err,
		)
		os.Exit(1)
	}
}
