package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chris576/vigil/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	process.Store
	p   process.Process
	err error
}

func (m *mockStore) Load(name string) (process.Process, error) {
	return m.p, m.err
}

func TestUpdate_ErrNotScript(t *testing.T) {
	store := &mockStore{p: process.Process{Name: "app", WorkingDir: t.TempDir()}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrNotScript)
}

func TestUpdate_ErrLocked(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".vigil.lock")
	os.WriteFile(lockPath, []byte("12345\n"), 0644)

	store := &mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: "/nonexistent/update.sh",
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrLocked)
}

func TestUpdate_ErrNoPackage(t *testing.T) {
	dir := t.TempDir()
	store := &mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: "/nonexistent/update.sh",
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "")
	assert.ErrorIs(t, err, ErrNoPackage)
}

func TestUpdate_ErrIntegrity(t *testing.T) {
	dir := t.TempDir()
	incomingDir := filepath.Join(dir, "incoming")
	os.MkdirAll(incomingDir, 0755)

	pkgPath := filepath.Join(incomingDir, "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("corrupt data"), 0644)

	hash := sha256.Sum256([]byte("different data"))
	sumPath := pkgPath + ".sha256"
	os.WriteFile(sumPath, []byte(hex.EncodeToString(hash[:])), 0644)

	store := &mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: "/nonexistent/update.sh",
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrIntegrity)
}

func TestUpdate_ScriptExtractFails(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
case "$1" in
	extract) exit 1 ;;
	*) exit 0 ;;
esac
`)

	store := &mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrScriptFailed)
	assert.NoDirExists(t, filepath.Join(dir, "releases", "v1.0.0"))
}

func TestUpdate_ScriptDepsFails(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
case "$1" in
	deps) exit 1 ;;
	*) exit 0 ;;
esac
`)

	store := &mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrScriptFailed)
	assert.NoDirExists(t, filepath.Join(dir, "releases", "v1.0.0"))
}

func TestUpdate_Success(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
case "$1" in
	extract) mkdir -p "$3/app" && echo "extracted" > "$3/app/server.js" ;;
	deps) mkdir -p "$2/node_modules" && echo "ok" > "$2/node_modules/ready" ;;
	migrate) echo "migrated" > "$2/.migrated" ;;
	health-check) exit 0 ;;
	*) exit 1 ;;
esac
`)

	sharedDir := filepath.Join(dir, "shared")
	os.WriteFile(filepath.Join(sharedDir, ".env"), []byte("KEY=val\n"), 0644)

	restarted := false
	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
		KeepReleases: 2,
	}}, func(ctx context.Context, name string) error {
		restarted = true
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.NoError(t, err)
	assert.True(t, restarted)

	releaseDir := filepath.Join(dir, "releases", "v1.0.0")
	assert.DirExists(t, releaseDir)
	assert.FileExists(t, filepath.Join(releaseDir, "app", "server.js"))
	assert.FileExists(t, filepath.Join(releaseDir, "node_modules", "ready"))
	assert.FileExists(t, filepath.Join(releaseDir, ".migrated"))

	current, err := os.Readlink(filepath.Join(dir, "current"))
	require.NoError(t, err)
	assert.Equal(t, releaseDir, current)

	envSymlink := filepath.Join(releaseDir, ".env")
	linkTarget, err := os.Readlink(envSymlink)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sharedDir, ".env"), linkTarget)
}

func TestUpdate_AutoDetectVersion(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	pkgPath := filepath.Join(dir, "incoming", "v2.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "")
	require.NoError(t, err)

	releaseDir := filepath.Join(dir, "releases", "v2.0.0")
	assert.DirExists(t, releaseDir)
}

func TestUpdate_CustomIncomingDir(t *testing.T) {
	dir := t.TempDir()
	incomingDir := filepath.Join(dir, "custom-incoming")
	os.MkdirAll(incomingDir, 0755)
	os.MkdirAll(filepath.Join(dir, "releases"), 0755)
	os.MkdirAll(filepath.Join(dir, "shared"), 0755)

	pkgPath := filepath.Join(incomingDir, "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
		IncomingDir:  incomingDir,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.NoError(t, err)
}

func TestUpdate_RollbackOnHealthCheckFailure(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	oldRelease := filepath.Join(dir, "releases", "v0.9.0")
	os.MkdirAll(oldRelease, 0755)
	os.WriteFile(filepath.Join(oldRelease, "server.js"), []byte("old"), 0644)

	currentSymlink := filepath.Join(dir, "current")
	os.Symlink(oldRelease, currentSymlink)

	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
case "$1" in
	extract) mkdir -p "$3" && echo "new" > "$3/server.js" ;;
	health-check) exit 1 ;;
	*) exit 0 ;;
esac
`)

	restartCount := 0
	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
	}}, func(ctx context.Context, name string) error {
		restartCount++
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrRolledBack)

	current, _ := os.Readlink(currentSymlink)
	assert.Equal(t, oldRelease, current, "symlink should point back to old release")
	assert.Equal(t, 2, restartCount, "restart called: once for new, once for rollback")
}

func TestUpdate_CleanupOldReleases(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	for _, v := range []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0"} {
		os.MkdirAll(filepath.Join(dir, "releases", v), 0755)
	}

	currentSymlink := filepath.Join(dir, "current")
	os.Symlink(filepath.Join(dir, "releases", "v1.3.0"), currentSymlink)

	pkgPath := filepath.Join(dir, "incoming", "v2.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
		KeepReleases: 3,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "v2.0.0")
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(dir, "releases", "v2.0.0"))
	assert.DirExists(t, filepath.Join(dir, "releases", "v1.3.0"))
	assert.DirExists(t, filepath.Join(dir, "releases", "v1.2.0"))
	assert.NoDirExists(t, filepath.Join(dir, "releases", "v1.1.0"))
	assert.NoDirExists(t, filepath.Join(dir, "releases", "v1.0.0"))
}

func TestUpdate_SharedSymlinks(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	sharedDir := filepath.Join(dir, "shared")
	os.WriteFile(filepath.Join(sharedDir, ".env"), []byte("KEY=val\n"), 0644)
	os.WriteFile(filepath.Join(sharedDir, "config.json"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(sharedDir, "data"), 0755)

	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.NoError(t, err)

	releaseDir := filepath.Join(dir, "releases", "v1.0.0")
	for _, name := range []string{".env", "config.json", "data"} {
		target, err := os.Readlink(filepath.Join(releaseDir, name))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(sharedDir, name), target)
	}
}

func TestUpdate_RestartFailure(t *testing.T) {
	dir := t.TempDir()
	setupUpdateDir(t, dir)

	oldRelease := filepath.Join(dir, "releases", "v0.9.0")
	os.MkdirAll(oldRelease, 0755)
	currentSymlink := filepath.Join(dir, "current")
	os.Symlink(oldRelease, currentSymlink)

	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("pkg data"), 0644)

	script := filepath.Join(dir, "update.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	restartCount := 0
	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		WorkingDir:   dir,
		UpdateScript: script,
	}}, func(ctx context.Context, name string) error {
		restartCount++
		if restartCount == 1 {
			return fmt.Errorf("restart failed")
		}
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restart")

	current, _ := os.Readlink(currentSymlink)
	assert.Equal(t, oldRelease, current, "symlink rolled back after restart failure")
}

func TestUpdate_NoWorkingDir(t *testing.T) {
	svc := NewService(&mockStore{p: process.Process{
		Name:         "app",
		UpdateScript: "/nonexistent",
	}}, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "working_dir")
}

func TestFindVersion(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	os.WriteFile(filepath.Join(dir, "v1.0.0.tar.gz"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(dir, "v1.0.0.tar.gz.sha256"), []byte("sum"), 0644)

	v, err := findVersion(dir)
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", v)
}

func TestFindVersion_Empty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	_, err := findVersion(dir)
	assert.ErrorIs(t, err, ErrNoPackage)
}

func TestVerifyIntegrity_Mismatch(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "v1.0.0.tar.gz")
	os.WriteFile(pkg, []byte("data"), 0644)

	hash := sha256.Sum256([]byte("wrong"))
	os.WriteFile(pkg+".sha256", []byte(hex.EncodeToString(hash[:])), 0644)

	err := verifyIntegrity(pkg)
	assert.ErrorIs(t, err, ErrIntegrity)
}

func TestVerifyIntegrity_Match(t *testing.T) {
	dir := t.TempDir()
	data := []byte("data")
	pkg := filepath.Join(dir, "v1.0.0.tar.gz")
	os.WriteFile(pkg, data, 0644)

	hash := sha256.Sum256(data)
	os.WriteFile(pkg+".sha256", []byte(hex.EncodeToString(hash[:])), 0644)

	err := verifyIntegrity(pkg)
	assert.NoError(t, err)
}

func TestVerifyIntegrity_NoSumFile(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "v1.0.0.tar.gz")
	os.WriteFile(pkg, []byte("data"), 0644)

	err := verifyIntegrity(pkg)
	assert.NoError(t, err)
}

func TestLinkShared(t *testing.T) {
	sharedDir := t.TempDir()
	releaseDir := t.TempDir()

	os.WriteFile(filepath.Join(sharedDir, "file1.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(sharedDir, "file2.txt"), []byte("b"), 0644)

	err := linkShared(sharedDir, releaseDir)
	require.NoError(t, err)

	target1, _ := os.Readlink(filepath.Join(releaseDir, "file1.txt"))
	assert.Equal(t, filepath.Join(sharedDir, "file1.txt"), target1)
	target2, _ := os.Readlink(filepath.Join(releaseDir, "file2.txt"))
	assert.Equal(t, filepath.Join(sharedDir, "file2.txt"), target2)
}

func TestCleanupReleases(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0"}
	for _, v := range versions {
		os.MkdirAll(filepath.Join(dir, v), 0755)
	}

	err := cleanupReleases(dir, "v1.4.0", 3)
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(dir, "v1.4.0"))
	assert.DirExists(t, filepath.Join(dir, "v1.3.0"))
	assert.DirExists(t, filepath.Join(dir, "v1.2.0"))
	assert.NoDirExists(t, filepath.Join(dir, "v1.1.0"))
	assert.NoDirExists(t, filepath.Join(dir, "v1.0.0"))
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1.0", "v1.0.0", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.10.0", "v1.9.0", 1},
		{"1.0.0", "v1.0.0", 0},
	}
	for _, tc := range tests {
		got := compareVersions(tc.a, tc.b)
		if tc.want < 0 {
			assert.True(t, got < 0, "expected %s < %s, got %d", tc.a, tc.b, got)
		} else if tc.want > 0 {
			assert.True(t, got > 0, "expected %s > %s, got %d", tc.a, tc.b, got)
		} else {
			assert.Equal(t, 0, got, "expected %s == %s", tc.a, tc.b)
		}
	}
}

func TestSwitchSymlink(t *testing.T) {
	dir := t.TempDir()
	symPath := filepath.Join(dir, "link")
	target := filepath.Join(dir, "target")

	err := switchSymlink(symPath, target)
	require.NoError(t, err)

	got, err := os.Readlink(symPath)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}

func TestRollbackSymlink_EmptyTarget(t *testing.T) {
	rollbackSymlink("/nonexistent/link", "")
}

func TestCleanupReleases_NoneToDelete(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1.0.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "v1.1.0"), 0755)

	err := cleanupReleases(dir, "v1.1.0", 5)
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "v1.0.0"))
	assert.DirExists(t, filepath.Join(dir, "v1.1.0"))
}

func TestCleanupReleases_NonExistentDir(t *testing.T) {
	err := cleanupReleases(t.TempDir()+"/nonexistent", "v1.0.0", 3)
	require.NoError(t, err)
}

func TestLinkShared_SharedNotExist(t *testing.T) {
	err := linkShared(t.TempDir()+"/nonexistent", t.TempDir())
	require.NoError(t, err)
}

func TestLinkShared_OverwritesExisting(t *testing.T) {
	sharedDir := t.TempDir()
	releaseDir := t.TempDir()

	os.WriteFile(filepath.Join(sharedDir, "config.json"), []byte(`{"shared":true}`), 0644)
	os.WriteFile(filepath.Join(releaseDir, "config.json"), []byte(`{"old":true}`), 0644)

	err := linkShared(sharedDir, releaseDir)
	require.NoError(t, err)

	target, err := os.Readlink(filepath.Join(releaseDir, "config.json"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sharedDir, "config.json"), target)
}

func TestLock_WritePID(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lock(dir)
	require.NoError(t, err)
	require.NotNil(t, unlock)

	lockPath := filepath.Join(dir, ".vigil.lock")
	data, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\n")

	unlock()
	_, err = os.Stat(lockPath)
	assert.True(t, os.IsNotExist(err))
}

func TestFindVersion_MissingDir(t *testing.T) {
	_, err := findVersion(t.TempDir() + "/nonexistent")
	assert.ErrorIs(t, err, ErrNoPackage)
}

func TestRunScript_ExitCode(t *testing.T) {
	script := filepath.Join(t.TempDir(), "test.sh")
	writeScript(t, script, `#!/bin/sh
exit 1
`)

	err := runScript(script, "test", "arg1")
	assert.ErrorIs(t, err, ErrScriptFailed)
}

func setupUpdateDir(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, "releases"), 0755)
	os.MkdirAll(filepath.Join(dir, "shared"), 0755)
	os.MkdirAll(filepath.Join(dir, "incoming"), 0755)
}

func writeScript(t *testing.T, path, content string) {
	t.Helper()
	os.WriteFile(path, []byte(content), 0755)

	abs, err := filepath.Abs(path)
	require.NoError(t, err)

	cmd := exec.Command("sh", "-c", fmt.Sprintf("command -v %s", abs))
	if err := cmd.Run(); err != nil {
		t.Skipf("script not executable with sh: %v", err)
	}
}
