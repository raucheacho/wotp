package commands

import (
	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/keys"
	"github.com/wotp/cli/internal/ui"
)

// NewKeysCmd creates the `wotp keys` command.
func NewKeysCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "keys",
		Short: "Display API keys",
		Long:  "Reads the .env file and displays the anon and service API keys.",
		RunE:  runKeys,
	}
}

func runKeys(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	anonKey, serviceKey, err := keys.ReadEnvFile(config.EnvPath(projectDir))
	if err != nil {
		return err
	}

	ui.Blank()
	ui.Title("Wotp API Keys")
	ui.Blank()
	ui.PrintKeys(anonKey, serviceKey)
	ui.Blank()
	ui.Warning("Never commit these keys. They are stored in wotp/.env (gitignored).")
	ui.Blank()

	return nil
}
