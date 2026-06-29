package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

var _ Client = (*client)(nil)

type client struct{}

func New() (Client, error) {
	return &client{}, nil
}

const configTemplate = `server {
	listen {{.Port}};
	server_name {{.Domain}};
	root {{.Root}};
	index index.html;
}
`

type configData struct {
	Port   int
	Domain string
	Root   string
}

func (c *client) EnableSite(name string, port int, domain, root string) error {
	tmpl, err := template.New("site").Parse(configTemplate)
	if err != nil {
		return fmt.Errorf("enabling nginx site: parsing template: %w", err)
	}

	confPath := availablePath(name)
	f, err := os.Create(confPath)
	if err != nil {
		return fmt.Errorf("enabling nginx site: creating config: %w", err)
	}
	if err := tmpl.Execute(f, configData{Port: port, Domain: domain, Root: root}); err != nil {
		f.Close()
		return fmt.Errorf("enabling nginx site: writing config: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("enabling nginx site: closing config: %w", err)
	}

	linkPath := enabledPath(name)
	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("enabling nginx site: removing existing symlink: %w", err)
		}
	}

	if err := os.Symlink(confPath, linkPath); err != nil {
		return fmt.Errorf("enabling nginx site: creating symlink: %w", err)
	}

	return nil
}

func (c *client) DisableSite(name string) error {
	if err := os.Remove(enabledPath(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("disabling nginx site: %w", err)
	}
	return nil
}

func (c *client) RemoveSiteConfig(name string) error {
	if err := os.Remove(availablePath(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing nginx site config: %w", err)
	}
	return nil
}

func (c *client) SiteEnabled(name string) (bool, error) {
	if _, err := os.Stat(enabledPath(name)); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("checking nginx site enabled: %w", err)
	}
	return true, nil
}

func (c *client) Reload(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "nginx", "-s", "reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reloading nginx: %w", err)
	}
	return nil
}

func (c *client) Close() error {
	return nil
}

var availablePath = func(name string) string {
	return "/etc/nginx/sites-available/" + name + ".conf"
}

var enabledPath = func(name string) string {
	return "/etc/nginx/sites-enabled/" + name + ".conf"
}
