package nginx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NB: Tests that access package-level availablePath/enabledPath vars
// CANNOT use t.Parallel() because setupTempDirs monkey-patches them.
// Only pure-function tests without path var access may be parallel.

func TestNew(t *testing.T) {
	t.Parallel()
	c, err := New()
	require.NoError(t, err)
	require.NotNil(t, c)

	_, ok := c.(*client)
	assert.True(t, ok)
}

func TestClose(t *testing.T) {
	t.Parallel()
	c := &client{}
	err := c.Close()
	assert.NoError(t, err)
}

func TestConfigTemplate(t *testing.T) {
	t.Parallel()
	tmpl, err := template.New("site").Parse(configTemplate)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, configData{Port: 8080, Domain: "example.com", Root: "/var/www"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "listen 8080;")
	assert.Contains(t, output, "server_name example.com;")
	assert.Contains(t, output, "root /var/www;")
	assert.Contains(t, output, "index index.html;")
}

// setupTempDirs creates temp dirs mimicking /etc/nginx structure.
// Returns cleanup func that restores original path functions.
// Caller MUST NOT use t.Parallel() — package-level vars are used.
func setupTempDirs(t *testing.T) (cli *client, availableDir, enabledDir string, cleanup func()) {
	t.Helper()

	availableDir = t.TempDir()
	enabledDir = t.TempDir()

	origAvailable := availablePath
	origEnabled := enabledPath

	availablePath = func(name string) string {
		return filepath.Join(availableDir, name+".conf")
	}
	enabledPath = func(name string) string {
		return filepath.Join(enabledDir, name+".conf")
	}

	cleanup = func() {
		availablePath = origAvailable
		enabledPath = origEnabled
	}

	return &client{}, availableDir, enabledDir, cleanup
}

func TestClient_EnableSite(t *testing.T) {
	c, availDir, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	err := c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.NoError(t, err)

	// Verify config file in sites-available
	confPath := filepath.Join(availDir, "myapp.conf")
	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "listen 8080;")
	assert.Contains(t, string(data), "server_name example.com;")
	assert.Contains(t, string(data), "root /var/www;")
	assert.Contains(t, string(data), "index index.html;")

	// Verify symlink in sites-enabled
	linkPath := filepath.Join(enabledDir, "myapp.conf")
	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, confPath, linkTarget)
}

func TestClient_EnableSite_ReplaceExisting(t *testing.T) {
	c, availDir, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	// Pre-populate both dirs
	confPath := filepath.Join(availDir, "myapp.conf")
	err := os.WriteFile(confPath, []byte("old"), 0644)
	require.NoError(t, err)

	linkPath := filepath.Join(enabledDir, "myapp.conf")
	err = os.Symlink(confPath, linkPath)
	require.NoError(t, err)

	// Re-enable with different config
	err = c.EnableSite("myapp", 3000, "new.example.com", "/var/new")
	require.NoError(t, err)

	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "listen 3000;")
	assert.Contains(t, string(data), "server_name new.example.com;")
}

func TestClient_EnableSite_DefaultPathFails(t *testing.T) {
	c := &client{}
	err := c.EnableSite("test-site-xyz", 8080, "example.com", "/var/www")
	assert.Error(t, err)
}

func TestClient_EnableSite_SymlinkCreateError(t *testing.T) {
	c, availDir, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	// Create config + symlink successfully first
	err := c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.NoError(t, err)

	// Remove the symlink so EnableSite hits Symlink (not Remove)
	linkPath := filepath.Join(enabledDir, "myapp.conf")
	err = os.Remove(linkPath)
	require.NoError(t, err)

	// Make enabledDir read-only — Symlink will fail
	err = os.Chmod(enabledDir, 0555)
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(enabledDir, 0755)
	}()

	err = c.EnableSite("myapp", 8080, "example.com", "/var/www")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating symlink")

	// Verify config file still exists (created before symlink attempt)
	confPath := filepath.Join(availDir, "myapp.conf")
	_, err = os.Stat(confPath)
	assert.NoError(t, err)
}

func TestClient_EnableSite_SymlinkRemoveError(t *testing.T) {
	c, _, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	// Create config + symlink
	err := c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.NoError(t, err)

	// Make enabledDir read-only — Remove during replace will fail
	err = os.Chmod(enabledDir, 0555)
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(enabledDir, 0755)
	}()

	err = c.EnableSite("myapp", 8080, "example.com", "/var/www")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "removing existing symlink")
}

func TestClient_DisableSite(t *testing.T) {
	t.Run("non-existent returns nil", func(t *testing.T) {
		c := &client{}
		err := c.DisableSite("nonexistent")
		assert.NoError(t, err)
	})

	t.Run("existing symlink removed", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err := os.Symlink("/tmp/fake.conf", linkPath)
		require.NoError(t, err)

		err = c.DisableSite("myapp")
		require.NoError(t, err)

		_, err = os.Stat(linkPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("remove fails returns error", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err := os.Symlink("/tmp/fake.conf", linkPath)
		require.NoError(t, err)

		// Remove write permission — os.Remove will fail
		err = os.Chmod(enabledDir, 0555)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(enabledDir, 0755) }()

		err = c.DisableSite("myapp")
		assert.Error(t, err)
	})
}

func TestClient_RemoveSiteConfig(t *testing.T) {
	t.Run("non-existent returns nil", func(t *testing.T) {
		c := &client{}
		err := c.RemoveSiteConfig("nonexistent")
		assert.NoError(t, err)
	})

	t.Run("existing config removed", func(t *testing.T) {
		c, availDir, _, cleanup := setupTempDirs(t)
		defer cleanup()

		confPath := filepath.Join(availDir, "myapp.conf")
		err := os.WriteFile(confPath, []byte("config"), 0644)
		require.NoError(t, err)

		err = c.RemoveSiteConfig("myapp")
		require.NoError(t, err)

		_, err = os.Stat(confPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("remove fails returns error", func(t *testing.T) {
		c, availDir, _, cleanup := setupTempDirs(t)
		defer cleanup()

		confPath := filepath.Join(availDir, "myapp.conf")
		err := os.WriteFile(confPath, []byte("config"), 0644)
		require.NoError(t, err)

		// Remove write permission — os.Remove fails
		err = os.Chmod(availDir, 0555)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(availDir, 0755) }()

		err = c.RemoveSiteConfig("myapp")
		assert.Error(t, err)
	})
}

func TestClient_SiteEnabled(t *testing.T) {
	t.Run("non-existent returns false", func(t *testing.T) {
		c := &client{}
		enabled, err := c.SiteEnabled("nonexistent")
		assert.NoError(t, err)
		assert.False(t, enabled)
	})

	t.Run("existing symlink returns true", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		// Create a real target so os.Stat follows symlink and succeeds
		targetDir := t.TempDir()
		targetPath := filepath.Join(targetDir, "target.conf")
		err := os.WriteFile(targetPath, []byte("config"), 0644)
		require.NoError(t, err)

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err = os.Symlink(targetPath, linkPath)
		require.NoError(t, err)

		enabled, err := c.SiteEnabled("myapp")
		require.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("stat error returns error", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		// Create target in a dir, then remove search permission
		targetDir := t.TempDir()
		targetPath := filepath.Join(targetDir, "target.conf")
		err := os.WriteFile(targetPath, []byte("x"), 0644)
		require.NoError(t, err)

		// No execute bit = not searchable → os.Stat fails with EACCES
		err = os.Chmod(targetDir, 0644)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(targetDir, 0700) }()

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err = os.Symlink(targetPath, linkPath)
		require.NoError(t, err)

		_, err = c.SiteEnabled("myapp")
		assert.Error(t, err)
	})
}

func TestClient_Reload_CanceledContext(t *testing.T) {
	t.Parallel()
	c := &client{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Reload(ctx)
	assert.Error(t, err)
}

func TestPaths_default(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) string
		want string
	}{
		{name: "availablePath", fn: availablePath, want: "/etc/nginx/sites-available/myapp.conf"},
		{name: "enabledPath", fn: enabledPath, want: "/etc/nginx/sites-enabled/myapp.conf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn("myapp")
			assert.Equal(t, tt.want, got)
		})
	}
}
