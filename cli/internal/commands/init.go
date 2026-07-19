// Package commands implements all wotp CLI commands.
package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/keys"
	"github.com/wotp/cli/internal/ui"
)

// NewInitCmd creates the `wotp init <project-name>` command.
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [project-name]",
		Short: "Create a new wotp project",
		Long:  "Creates the wotp/ directory with all configuration files needed to run a Wotp instance. If project-name is not provided, initializes in the current directory.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	var projectName string
	var projectDir string
	var err error

	if len(args) > 0 {
		projectName = args[0]
		projectDir, err = filepath.Abs(projectName)
		if err != nil {
			return err
		}
	} else {
		projectDir, err = os.Getwd()
		if err != nil {
			return err
		}
		projectName = filepath.Base(projectDir)
	}

	// Generate project ref: name + random 4-char hex suffix
	suffix, err := randomHex(2)
	if err != nil {
		return fmt.Errorf("generating project ref: %w", err)
	}
	projectRef := fmt.Sprintf("%s-%s", projectName, suffix)

	wotpDir := config.WotpDir(projectDir)

	ui.Blank()
	ui.Title(fmt.Sprintf("Initializing %s...", ui.Brand(projectName)))
	ui.Blank()

	// Create directories
	dirs := []string{
		wotpDir,
		filepath.Join(wotpDir, "seed"),
		filepath.Join(wotpDir, ".wotp"),
		filepath.Join(wotpDir, ".wotp", "data"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	ui.Success("Created project structure")

	// Write config.toml
	cfg := config.DefaultConfig(projectName, projectRef)
	if err := config.Write(cfg, config.ConfigPath(projectDir)); err != nil {
		return err
	}
	ui.Success("Generated config.toml")

	// Write seed/templates.toml
	if err := writeTemplates(projectDir, projectName); err != nil {
		return err
	}
	ui.Success("Generated seed/templates.toml")

	// Generate API keys and write .env. The root key authorizes wotp-core's
	// instance-admin endpoints (`wotp project ...`) — wotp-core imports it
	// from WOTP_ROOT_KEY on first boot, same mechanism as anon/service.
	anonKey, serviceKey, err := keys.GenerateKeyPair()
	if err != nil {
		return err
	}
	rootKey, err := keys.GenerateKey(keys.RootPrefix)
	if err != nil {
		return err
	}
	if err := keys.WriteEnvFile(config.EnvPath(projectDir), anonKey, serviceKey, rootKey); err != nil {
		return err
	}
	ui.Blank()
	ui.PrintKeys(anonKey, serviceKey)
	ui.KeyValue("Root key", rootKey)
	ui.Blank()
	ui.Info("Next steps:")
	if len(args) > 0 {
		ui.Dim(fmt.Sprintf("  cd %s", projectName))
	}
	ui.Dim("  wotp start")
	ui.Blank()

	return nil
}

// writeTemplates writes the seed/templates.toml file matching spec §4.2.
func writeTemplates(projectDir, projectName string) error {
	// Use a capitalized version of the project name for templates
	displayName := projectName

	content := fmt.Sprintf(`[fr]
otp_message = "Votre code de vérification %s : {{code}}. Valable {{expiry}} minutes."

[darija]
otp_message = "Code dyalek f %s: {{code}}. Salih {{expiry}} d9ay9."

[en]
otp_message = "Your %s verification code: {{code}}. Valid for {{expiry}} minutes."
`, displayName, displayName, displayName)

	path := filepath.Join(config.WotpDir(projectDir), "seed", "templates.toml")
	return os.WriteFile(path, []byte(content), 0o644)
}


func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
