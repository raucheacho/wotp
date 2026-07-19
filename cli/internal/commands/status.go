package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/docker"
	"github.com/wotp/cli/internal/ui"
)

// NewStatusCmd creates the `wotp status` command.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show wotp instance status",
		Long:  "Displays connection state, uptime, and health information.",
		RunE:  runStatus,
	}
}

// healthResponse matches wotp-core's GET /v1/health response, which is
// instance-wide (no longer carries a single number's phone/status — see
// `wotp project keys <slug>` for per-number connection state).
type healthResponse struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(config.ConfigPath(projectDir))
	if err != nil {
		return err
	}

	// Check if containers are running
	if !docker.IsRunning(projectDir) {
		ui.Blank()
		ui.PrintStatus("stopped", 0)
		ui.Dim("  Run 'wotp start' to start the instance.")
		ui.Blank()
		return nil
	}

	// Call the health endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := fmt.Sprintf("http://localhost:%d/v1/health", cfg.API.Port)

	resp, err := client.Get(healthURL)
	if err != nil {
		ui.Blank()
		ui.PrintStatus("unreachable", 0)
		ui.Dim("  Container is running but API is not responding.")
		ui.Dimf("  Error: %v", err)
		ui.Blank()
		return nil
	}
	defer resp.Body.Close()

	var health healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		ui.Blank()
		ui.PrintStatus("unknown", 0)
		ui.Dim("  Could not parse health response.")
		ui.Blank()
		return nil
	}

	ui.PrintStatus(health.Status, health.UptimeSeconds)

	return nil
}
