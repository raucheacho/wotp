package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/wotp/cli/internal/config"
	"github.com/wotp/cli/internal/keys"
	"github.com/wotp/cli/internal/ui"
)

// NewProjectCmd creates the `wotp project` command group. Unlike every other
// wotp command (which manipulates local files/Docker directly), these talk
// HTTP to the already-running instance using its root key — project
// management lives in wotp-core, not in the CLI.
func NewProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects on this wotp instance",
		Long:  "Projects are isolated tenants inside a single wotp instance, each with their own WhatsApp numbers, API keys, and message history.",
	}
	cmd.AddCommand(
		newProjectCreateCmd(),
		newProjectListCmd(),
		newProjectRmCmd(),
		newProjectAddNumberCmd(),
		newProjectKeysCmd(),
	)
	return cmd
}

type projectInfo struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type projectNumber struct {
	JID       string `json:"jid"`
	Phone     string `json:"phone"`
	Connected bool   `json:"connected"`
}

// projectClient talks to a running wotp-core instance's instance-admin API.
type projectClient struct {
	baseURL string
	rootKey string
	http    *http.Client
}

func newProjectClient() (*projectClient, error) {
	projectDir, err := config.FindProjectDir()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(config.ConfigPath(projectDir))
	if err != nil {
		return nil, err
	}
	_, _, rootKey, err := keys.ReadEnvFile(config.EnvPath(projectDir))
	if err != nil {
		return nil, err
	}
	if rootKey == "" {
		return nil, fmt.Errorf("no root key found in wotp/.wotp/.env — re-run `wotp init` (or add WOTP_ROOT_KEY manually) then `wotp restart`")
	}

	return &projectClient{
		baseURL: fmt.Sprintf("http://localhost:%d", cfg.API.Port),
		rootKey: rootKey,
		http:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// do sends an authenticated request to wotp-core and decodes the JSON
// response into out (if non-nil).
func (c *projectClient) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.rootKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling wotp-core at %s: %w (is the instance running? try `wotp start`)", c.baseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("%s", apiErr.Error)
		}
		return fmt.Errorf("wotp-core returned %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func (c *projectClient) findBySlug(slug string) (*projectInfo, error) {
	var projects []projectInfo
	if err := c.do(http.MethodGet, "/v1/projects", nil, &projects); err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i].Slug == slug {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("no project with slug %q found (run `wotp project list`)", slug)
}

func newProjectCreateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create <slug>",
		Short: "Create a new project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			if name == "" {
				name = slug
			}

			client, err := newProjectClient()
			if err != nil {
				return err
			}

			var resp struct {
				Project    projectInfo `json:"project"`
				AnonKey    string      `json:"anon_key"`
				ServiceKey string      `json:"service_key"`
			}
			if err := client.do(http.MethodPost, "/v1/projects", map[string]string{"slug": slug, "name": name}, &resp); err != nil {
				return err
			}

			ui.Blank()
			ui.Success(fmt.Sprintf("Project %s created", ui.Brand(resp.Project.Slug)))
			ui.Blank()
			ui.PrintKeys(resp.AnonKey, resp.ServiceKey)
			ui.Blank()
			ui.Info("Next step:")
			ui.Dim(fmt.Sprintf("  wotp project add-number %s", slug))
			ui.Blank()
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Display name (defaults to the slug)")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects on this instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newProjectClient()
			if err != nil {
				return err
			}

			var projects []projectInfo
			if err := client.do(http.MethodGet, "/v1/projects", nil, &projects); err != nil {
				return err
			}

			ui.Blank()
			ui.Title("Projects")
			ui.Blank()
			if len(projects) == 0 {
				ui.Dim("  No projects yet. Run `wotp project create <slug>`.")
			}
			for _, p := range projects {
				ui.KeyValue(p.Slug, p.Name)
			}
			ui.Blank()
			return nil
		},
	}
}

func newProjectRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <slug>",
		Short: "Delete a project and all its data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			client, err := newProjectClient()
			if err != nil {
				return err
			}
			p, err := client.findBySlug(slug)
			if err != nil {
				return err
			}

			ui.Blank()
			ui.DangerBox(fmt.Sprintf("This will permanently delete project %q:\nits WhatsApp numbers, OTP/message history, and API keys.", slug))
			ui.Blank()
			if !ui.DoubleConfirmPrompt("Delete this project?", slug) {
				ui.Dim("  Cancelled.")
				ui.Blank()
				return nil
			}

			if err := client.do(http.MethodDelete, "/v1/projects/"+p.ID, nil, nil); err != nil {
				return err
			}
			ui.Success(fmt.Sprintf("Project %s deleted", slug))
			return nil
		},
	}
}

func newProjectAddNumberCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-number <slug>",
		Short: "Pair a new WhatsApp number for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			client, err := newProjectClient()
			if err != nil {
				return err
			}
			p, err := client.findBySlug(slug)
			if err != nil {
				return err
			}

			// Each project is capped at one number — the server rejects this
			// outright (409, whatsapp.ErrAlreadyPaired) if one is already
			// paired. No client-side check needed; the API error is clear.
			if err := client.do(http.MethodPost, "/v1/projects/"+p.ID+"/numbers/pair", nil, nil); err != nil {
				return err
			}

			ui.Blank()
			ui.Success("Pairing started")
			ui.Blank()
			ui.Title("Scan this QR code with WhatsApp (Settings → Linked Devices):")
			ui.Blank()
			ui.Infof("Open %s/dashboard, select project %q, and scan the QR code shown there.", client.baseURL, slug)
			ui.Blank()
			return nil
		},
	}
}

func newProjectKeysCmd() *cobra.Command {
	var regenAnon, regenService bool
	cmd := &cobra.Command{
		Use:   "keys <slug>",
		Short: "Show a project's numbers, or regenerate its API keys",
		Long:  "Anon/service keys are only ever shown in plaintext once, at `wotp project create` time. Use --regenerate-anon or --regenerate-service to issue a new one.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			client, err := newProjectClient()
			if err != nil {
				return err
			}
			p, err := client.findBySlug(slug)
			if err != nil {
				return err
			}

			if regenAnon || regenService {
				tier := "anon"
				if regenService {
					tier = "service"
				}
				var resp struct {
					Key string `json:"key"`
				}
				if err := client.do(http.MethodPost, "/v1/projects/"+p.ID+"/keys/regenerate", map[string]string{"tier": tier}, &resp); err != nil {
					return err
				}
				ui.Blank()
				ui.Success(fmt.Sprintf("New %s key for %s:", tier, slug))
				ui.KeyValue(tier, resp.Key)
				ui.Blank()
				ui.Warning("The previous key stopped working immediately. Update any clients using it.")
				ui.Blank()
				return nil
			}

			var numbers []projectNumber
			if err := client.do(http.MethodGet, "/v1/projects/"+p.ID+"/numbers", nil, &numbers); err != nil {
				return err
			}

			ui.Blank()
			ui.Title(fmt.Sprintf("Project %s", ui.Brand(slug)))
			ui.Blank()
			if len(numbers) == 0 {
				ui.Dim(fmt.Sprintf("  No numbers paired yet. Run `wotp project add-number %s`.", slug))
			}
			for _, n := range numbers {
				status := "disconnected"
				if n.Connected {
					status = "connected"
				}
				ui.KeyValue(n.Phone, status)
			}
			ui.Blank()
			ui.Dim("  Anon/service keys are shown once at creation. Use --regenerate-anon / --regenerate-service to issue new ones.")
			ui.Blank()
			return nil
		},
	}
	cmd.Flags().BoolVar(&regenAnon, "regenerate-anon", false, "Issue a new anon key, invalidating the previous one")
	cmd.Flags().BoolVar(&regenService, "regenerate-service", false, "Issue a new service key, invalidating the previous one")
	return cmd
}
