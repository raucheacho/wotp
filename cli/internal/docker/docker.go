// Package docker handles Docker Compose template rendering and execution.
package docker

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/wotp/cli/internal/config"
)

//go:embed template.yml
var templateFS embed.FS


// TemplateData holds the values injected into the docker-compose template.
type TemplateData struct {
	Version string
	Port    int
}

// RenderCompose renders the embedded docker-compose template with config values
// and writes it to .wotp/docker-compose.yml.
func RenderCompose(cfg config.Config, projectDir string) error {
	tmplBytes, err := templateFS.ReadFile("template.yml")
	if err != nil {
		return fmt.Errorf("reading embedded template: %w", err)
	}

	tmpl, err := template.New("compose").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("parsing compose template: %w", err)
	}

	data := TemplateData{
		Version: config.AppVersion,
		Port:    cfg.API.Port,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("rendering compose template: %w", err)
	}

	composePath := config.ComposePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(composePath), 0o755); err != nil {
		return fmt.Errorf("creating runtime directory: %w", err)
	}

	if err := os.WriteFile(composePath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing docker-compose.yml: %w", err)
	}

	return nil
}

// EnsureDataDirs creates the data directory mounted into the container.
// wotp-core manages its own layout underneath it (control.db, data.db,
// session.db — see core/internal/project.Load); the CLI only needs the
// mount point itself to exist before `docker compose up`.
func EnsureDataDirs(projectDir string) error {
	dir := config.DataDir(projectDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating data directory %s: %w", dir, err)
	}
	return nil
}

// composeCmd builds an exec.Cmd for docker compose with the correct -f flag.
func composeCmd(projectDir string, args ...string) *exec.Cmd {
	composePath := config.ComposePath(projectDir)
	fullArgs := append([]string{"compose", "-f", composePath}, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = config.RuntimeDir(projectDir)
	return cmd
}

// Pull pulls the Docker images defined in the compose file.
func Pull(projectDir string) error {
	cmd := composeCmd(projectDir, "pull")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Up starts the containers in detached mode.
func Up(projectDir string) error {
	cmd := composeCmd(projectDir, "up", "-d", "--remove-orphans")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Stop stops the containers without removing them.
func Stop(projectDir string) error {
	cmd := composeCmd(projectDir, "stop")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Down stops and removes containers, networks. If removeVolumes is true, also removes volumes.
func Down(projectDir string, removeVolumes bool) error {
	args := []string{"down"}
	if removeVolumes {
		args = append(args, "-v")
	}
	cmd := composeCmd(projectDir, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Logs streams logs from the containers.
func Logs(projectDir string) error {
	cmd := composeCmd(projectDir, "logs", "-f", "--tail", "100")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// IsRunning checks if containers are currently running.
func IsRunning(projectDir string) bool {
	composePath := config.ComposePath(projectDir)
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return false
	}
	cmd := composeCmd(projectDir, "ps", "-q")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(bytes.TrimSpace(output)) > 0
}

// HasCoreImageLocally checks if the core image is already available locally.
func HasCoreImageLocally() bool {
	imageName := fmt.Sprintf("ghcr.io/raucheacho/wotp:%s", config.AppVersion)
	cmd := exec.Command("docker", "image", "inspect", imageName)
	return cmd.Run() == nil
}
