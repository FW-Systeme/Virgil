package process

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStore struct {
	Store
	dir string
}

func newTestStore(t *testing.T) testStore {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	s, err := NewStore()
	require.NoError(t, err)
	return testStore{Store: s, dir: filepath.Join(dir, ".config", "vigil", "apps")}
}

func TestStore_SaveAndLoad_NodeType(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	p := Process{
		Name:       "my-node-app",
		Type:       TypeNode,
		Port:       3000,
		Entry:      "index.js",
		BuildDir:   "dist",
		EnvFile:    ".env",
		WorkingDir: "/home/node/app",
		CreatedAt:  now,
		Enabled:    true,
	}

	err := store.Save(p)
	require.NoError(t, err)

	loaded, err := store.Load("my-node-app")
	require.NoError(t, err)
	assert.Equal(t, p, loaded)
}

func TestStore_SaveAndLoad_StaticType(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	p := Process{
		Name:        "my-static-site",
		Type:        TypeStatic,
		Port:        8080,
		NginxDomain: "example.com",
		NginxPath:   "/var/www/example",
		CreatedAt:   now,
		Enabled:     true,
	}

	err := store.Save(p)
	require.NoError(t, err)

	loaded, err := store.Load("my-static-site")
	require.NoError(t, err)
	assert.Equal(t, p, loaded)
}

func TestStore_Load_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Load("does-not-exist")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestStore_Delete_Existing(t *testing.T) {
	store := newTestStore(t)

	p := Process{Name: "delete-me", Type: TypeNode, Port: 4000, CreatedAt: fixedTime(), Enabled: true}
	err := store.Save(p)
	require.NoError(t, err)

	_, err = store.Load("delete-me")
	require.NoError(t, err)

	err = store.Delete("delete-me")
	require.NoError(t, err)

	_, err = store.Load("delete-me")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestStore_Delete_NonExistent(t *testing.T) {
	store := newTestStore(t)
	err := store.Delete("never-created")
	require.NoError(t, err)
}

func TestStore_List_Empty(t *testing.T) {
	store := newTestStore(t)
	processes, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, processes)
}

func TestStore_List_Multiple(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	processes := []Process{
		{Name: "alpha", Type: TypeNode, Port: 3001, CreatedAt: now, Enabled: true},
		{Name: "beta", Type: TypeStatic, Port: 8081, CreatedAt: now, Enabled: false},
		{Name: "gamma", Type: TypeNode, Port: 3002, CreatedAt: now, Enabled: true},
	}

	for _, p := range processes {
		err := store.Save(p)
		require.NoError(t, err)
	}

	loaded, err := store.List()
	require.NoError(t, err)
	require.Len(t, loaded, 3)

	loadedMap := make(map[string]Process, len(loaded))
	for _, p := range loaded {
		loadedMap[p.Name] = p
	}

	for _, expected := range processes {
		got, ok := loadedMap[expected.Name]
		if assert.True(t, ok, "process %q should be in List result", expected.Name) {
			assert.Equal(t, expected, got)
		}
	}
}

func TestStore_Save_Overwrite(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	original := Process{Name: "updatable", Type: TypeNode, Port: 3000, Entry: "v1.js", CreatedAt: now, Enabled: true}
	err := store.Save(original)
	require.NoError(t, err)

	updated := Process{Name: "updatable", Type: TypeNode, Port: 9999, Entry: "v2.js", CreatedAt: now.Add(time.Hour), Enabled: false}
	err = store.Save(updated)
	require.NoError(t, err)

	loaded, err := store.Load("updatable")
	require.NoError(t, err)
	assert.Equal(t, updated, loaded)
}

func TestStore_AppPath(t *testing.T) {
	store := newTestStore(t)

	got, err := store.AppPath("my-app")
	require.NoError(t, err)

	want := filepath.Join(store.dir, "my-app.json")
	assert.Equal(t, want, got)
}

func TestStore_SaveAndLoad_MinimalFields(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	p := Process{Name: "minimal", Type: TypeNode, Port: 0, CreatedAt: now, Enabled: false}
	err := store.Save(p)
	require.NoError(t, err)

	loaded, err := store.Load("minimal")
	require.NoError(t, err)
	assert.Equal(t, p, loaded)
}

func TestStore_SaveAndLoad_EmptyName(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	p := Process{Name: "", Type: TypeNode, Port: 5000, CreatedAt: now, Enabled: true}
	err := store.Save(p)
	require.NoError(t, err)

	loaded, err := store.Load("")
	require.NoError(t, err)
	assert.Equal(t, p, loaded)

	path, err := store.AppPath("")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(store.dir, ".json"), path)
}

func TestStore_SaveAndLoad_SpecialName(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	p := Process{Name: "app-with.dots_and-dashes", Type: TypeNode, Port: 6000, CreatedAt: now, Enabled: true}
	err := store.Save(p)
	require.NoError(t, err)

	loaded, err := store.Load("app-with.dots_and-dashes")
	require.NoError(t, err)
	assert.Equal(t, p, loaded)
}

func TestStore_List_IgnoresNonJSONFiles(t *testing.T) {
	store := newTestStore(t)

	nonJSON := filepath.Join(store.dir, "ignored.txt")
	err := os.WriteFile(nonJSON, []byte("garbage"), 0600)
	require.NoError(t, err)

	processes, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, processes)
}

func TestStore_List_MalformedJSON(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	good := Process{Name: "good-app", Type: TypeNode, Port: 8000, CreatedAt: now, Enabled: true}
	err := store.Save(good)
	require.NoError(t, err)

	badPath := filepath.Join(store.dir, "bad.json")
	err = os.WriteFile(badPath, []byte("{invalid json"), 0600)
	require.NoError(t, err)

	processes, err := store.List()
	require.NoError(t, err)

	found := false
	for _, p := range processes {
		if p.Name == "good-app" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestStore_AutoCreateDirectory(t *testing.T) {
	store := newTestStore(t)

	info, err := os.Stat(store.dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
	}
}

func TestStore_Persistence_SameDirectory(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("HOME", dir)
	store1, err := NewStore()
	require.NoError(t, err)

	p := Process{Name: "persist-test", Type: TypeStatic, Port: 9000, CreatedAt: fixedTime(), Enabled: true}
	err = store1.Save(p)
	require.NoError(t, err)

	store2, err := NewStore()
	require.NoError(t, err)

	loaded, err := store2.Load("persist-test")
	require.NoError(t, err)
	assert.Equal(t, p, loaded)
}

func TestStore_MultipleOperations(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	processes := []Process{
		{Name: "a", Type: TypeNode, Port: 1, CreatedAt: now, Enabled: true},
		{Name: "b", Type: TypeStatic, Port: 2, CreatedAt: now, Enabled: false},
		{Name: "c", Type: TypeNode, Port: 3, CreatedAt: now, Enabled: true},
	}

	for _, p := range processes {
		require.NoError(t, store.Save(p))
	}

	require.NoError(t, store.Delete("b"))

	list, err := store.List()
	require.NoError(t, err)
	require.Len(t, list, 2)

	_, err = store.Load("b")
	require.Error(t, err)

	_, err = store.Load("a")
	require.NoError(t, err)
	_, err = store.Load("c")
	require.NoError(t, err)

	names := make(map[string]bool, 2)
	for _, p := range list {
		names[p.Name] = true
	}
	assert.True(t, names["a"])
	assert.True(t, names["c"])
	assert.False(t, names["b"])
}

func TestStore_Save_PermissionDenied(t *testing.T) {
	store := newTestStore(t)

	err := os.Chmod(store.dir, 0555)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chmod(store.dir, 0755) })

	p := Process{Name: "no-perm", Type: TypeNode, Port: 1111, CreatedAt: fixedTime(), Enabled: true}
	err = store.Save(p)
	require.Error(t, err)
}

func TestStore_Save_RenameError(t *testing.T) {
	store := newTestStore(t)

	p := Process{Name: "rename-blocked", Type: TypeNode, Port: 2222, CreatedAt: fixedTime(), Enabled: true}

	targetDir := filepath.Join(store.dir, "rename-blocked.json")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	err = store.Save(p)
	require.Error(t, err)
}

func TestStore_List_StorageDirIsFile(t *testing.T) {
	store := newTestStore(t)

	err := os.RemoveAll(store.dir)
	require.NoError(t, err)

	err = os.WriteFile(store.dir, []byte("not-a-dir"), 0600)
	require.NoError(t, err)

	_, err = store.List()
	require.Error(t, err)
}

func TestStore_Load_CorruptFile(t *testing.T) {
	store := newTestStore(t)

	badPath := filepath.Join(store.dir, "corrupt.json")
	err := os.WriteFile(badPath, []byte("{broken"), 0600)
	require.NoError(t, err)

	_, err = store.Load("corrupt")
	require.Error(t, err)
}

func TestStore_NewStore_MkdirAllError(t *testing.T) {
	dir := t.TempDir()

	vigilPath := filepath.Join(dir, ".config", "vigil")
	err := os.MkdirAll(filepath.Dir(vigilPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(vigilPath, []byte("not a directory"), 0600)
	require.NoError(t, err)

	t.Setenv("HOME", dir)
	_, err = NewStore()
	require.Error(t, err)
}

func TestStore_List_UnreadableJSONFile(t *testing.T) {
	store := newTestStore(t)

	unreadablePath := filepath.Join(store.dir, "secret.json")
	err := os.WriteFile(unreadablePath, []byte(`{"name":"secret"}`), 0000)
	require.NoError(t, err)

	processes, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, processes)
}

func TestStore_List_IgnoresSubdirectories(t *testing.T) {
	store := newTestStore(t)

	subDir := filepath.Join(store.dir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(subDir, "ignored.json"), []byte("{}"), 0600)
	require.NoError(t, err)

	processes, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, processes)
}

func TestStore_Save_Atomicity(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	p := Process{Name: "atomic-test", Type: TypeNode, Port: 7000, CreatedAt: now, Enabled: true}
	err := store.Save(p)
	require.NoError(t, err)

	matches, err := filepath.Glob(filepath.Join(store.dir, "atomic-test*.json"))
	require.NoError(t, err)
	assert.Len(t, matches, 1)

	tmpFiles, err := filepath.Glob(filepath.Join(store.dir, "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, tmpFiles)
}
