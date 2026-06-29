package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogStore struct {
	LogStore
	dir string
}

func newTestLogStore(t *testing.T) testLogStore {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	s, err := NewLogStore()
	require.NoError(t, err)
	return testLogStore{LogStore: s, dir: filepath.Join(dir, ".config", "vigil", "logs")}
}

func TestLogStore_SaveAndLoad(t *testing.T) {
	store := newTestLogStore(t)

	cfg := LogConfig{
		Name:    "my-app",
		Enabled: true,
		LogPath: "/var/log/vigil/my-app.log",
		MaxSize: "10M",
		Rotate:  3,
	}
	err := store.Save(cfg)
	require.NoError(t, err)

	loaded, err := store.Load("my-app")
	require.NoError(t, err)
	assert.Equal(t, cfg, loaded)
}

func TestLogStore_Load_NotFound(t *testing.T) {
	store := newTestLogStore(t)
	_, err := store.Load("does-not-exist")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestLogStore_Delete_Existing(t *testing.T) {
	store := newTestLogStore(t)

	err := store.Save(LogConfig{Name: "delete-me", Enabled: true, LogPath: "/tmp", MaxSize: "10M", Rotate: 3})
	require.NoError(t, err)

	_, err = store.Load("delete-me")
	require.NoError(t, err)

	err = store.Delete("delete-me")
	require.NoError(t, err)

	_, err = store.Load("delete-me")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestLogStore_Delete_NonExistent(t *testing.T) {
	store := newTestLogStore(t)
	err := store.Delete("never-created")
	require.NoError(t, err)
}

func TestLogStore_List_Empty(t *testing.T) {
	store := newTestLogStore(t)
	configs, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, configs)
}

func TestLogStore_List_Multiple(t *testing.T) {
	store := newTestLogStore(t)

	configs := []LogConfig{
		{Name: "alpha", Enabled: true, LogPath: "/var/log/a.log", MaxSize: "10M", Rotate: 3},
		{Name: "beta", Enabled: false, LogPath: "/var/log/b.log", MaxSize: "5M", Rotate: 2},
	}

	for _, cfg := range configs {
		err := store.Save(cfg)
		require.NoError(t, err)
	}

	loaded, err := store.List()
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	loadedMap := make(map[string]LogConfig, len(loaded))
	for _, cfg := range loaded {
		loadedMap[cfg.Name] = cfg
	}

	for _, expected := range configs {
		got, ok := loadedMap[expected.Name]
		if assert.True(t, ok) {
			assert.Equal(t, expected, got)
		}
	}
}

func TestLogStore_Save_Overwrite(t *testing.T) {
	store := newTestLogStore(t)

	original := LogConfig{Name: "updatable", Enabled: true, LogPath: "/var/log/v1.log", MaxSize: "10M", Rotate: 3}
	err := store.Save(original)
	require.NoError(t, err)

	updated := LogConfig{Name: "updatable", Enabled: false, LogPath: "/var/log/v2.log", MaxSize: "50M", Rotate: 7}
	err = store.Save(updated)
	require.NoError(t, err)

	loaded, err := store.Load("updatable")
	require.NoError(t, err)
	assert.Equal(t, updated, loaded)
}

func TestLogStore_List_IgnoresNonJSONFiles(t *testing.T) {
	store := newTestLogStore(t)

	nonJSON := filepath.Join(store.dir, "ignored.txt")
	err := os.WriteFile(nonJSON, []byte("garbage"), 0600)
	require.NoError(t, err)

	configs, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, configs)
}

func TestLogStore_List_MalformedJSON(t *testing.T) {
	store := newTestLogStore(t)

	good := LogConfig{Name: "good-app", Enabled: true, LogPath: "/tmp", MaxSize: "10M", Rotate: 3}
	err := store.Save(good)
	require.NoError(t, err)

	badPath := filepath.Join(store.dir, "bad.json")
	err = os.WriteFile(badPath, []byte("{invalid json"), 0600)
	require.NoError(t, err)

	configs, err := store.List()
	require.NoError(t, err)

	found := false
	for _, cfg := range configs {
		if cfg.Name == "good-app" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestLogStore_NewLogStore_NonRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	s, err := NewLogStore()
	require.NoError(t, err)
	require.NotNil(t, s)

	expectedDir := filepath.Join(dir, ".config", "vigil", "logs")
	info, err := os.Stat(expectedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLogStore_Save_Atomicity(t *testing.T) {
	store := newTestLogStore(t)

	cfg := LogConfig{Name: "atomic-test", Enabled: true, LogPath: "/tmp", MaxSize: "10M", Rotate: 3}
	err := store.Save(cfg)
	require.NoError(t, err)

	matches, err := filepath.Glob(filepath.Join(store.dir, "atomic-test*.json"))
	require.NoError(t, err)
	assert.Len(t, matches, 1)

	tmpFiles, err := filepath.Glob(filepath.Join(store.dir, "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, tmpFiles)
}

func TestLogStore_Save_RenameError(t *testing.T) {
	store := newTestLogStore(t)

	cfg := LogConfig{Name: "rename-blocked", Enabled: true, LogPath: "/tmp", MaxSize: "10M", Rotate: 3}

	targetDir := filepath.Join(store.dir, "rename-blocked.json")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	err = store.Save(cfg)
	require.Error(t, err)
}

func TestLogStore_List_StorageDirIsFile(t *testing.T) {
	store := newTestLogStore(t)

	err := os.RemoveAll(store.dir)
	require.NoError(t, err)

	err = os.WriteFile(store.dir, []byte("not-a-dir"), 0600)
	require.NoError(t, err)

	_, err = store.List()
	require.Error(t, err)
}
