package systemd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitPath(t *testing.T) {
	t.Parallel()
	path := unitPath("myapp")
	assert.Equal(t, "/etc/systemd/system/myapp.service", path)
}

func TestCreateUnitFile(t *testing.T) {
	dir := t.TempDir()
	orig := unitPath
	unitPath = func(name string) string {
		return filepath.Join(dir, name+".service")
	}
	defer func() { unitPath = orig }()

	c := &client{}
	content := []byte("[Unit]\nDescription=test\n")
	err := c.CreateUnitFile("testapp", content)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "testapp.service"))
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestRemoveUnitFile(t *testing.T) {
	dir := t.TempDir()
	orig := unitPath
	unitPath = func(name string) string {
		return filepath.Join(dir, name+".service")
	}
	defer func() { unitPath = orig }()

	c := &client{}

	t.Run("non-existent returns nil", func(t *testing.T) {
		err := c.RemoveUnitFile("nonexistent")
		assert.NoError(t, err)
	})

	t.Run("existing file removed", func(t *testing.T) {
		path := filepath.Join(dir, "exists.service")
		err := os.WriteFile(path, []byte("content"), 0600)
		require.NoError(t, err)

		err = c.RemoveUnitFile("exists")
		require.NoError(t, err)

		_, err = os.Stat(path)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestJobWait_Done(t *testing.T) {
	err := jobWait(context.Background(), func(ch chan<- string) (int, error) {
		ch <- "done"
		return 0, nil
	})
	require.NoError(t, err)
}

func TestJobWait_Failed(t *testing.T) {
	err := jobWait(context.Background(), func(ch chan<- string) (int, error) {
		ch <- "failed"
		return 0, nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job failed")
}

func TestJobWait_FnError(t *testing.T) {
	err := jobWait(context.Background(), func(ch chan<- string) (int, error) {
		return 0, fmt.Errorf("dbus error")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dbus error")
}

func TestJobWait_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := jobWait(ctx, func(ch chan<- string) (int, error) {
		return 0, nil
	})
	require.Error(t, err)
}

func TestNilConn_StartUnit(t *testing.T) {
	c := &client{conn: nil}
	err := c.StartUnit(context.Background(), "test")
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoConn)
}

func TestNilConn_StopUnit(t *testing.T) {
	c := &client{conn: nil}
	err := c.StopUnit(context.Background(), "test")
	require.Error(t, err)
}

func TestNilConn_RestartUnit(t *testing.T) {
	c := &client{conn: nil}
	err := c.RestartUnit(context.Background(), "test")
	require.Error(t, err)
}

func TestNilConn_EnableUnit(t *testing.T) {
	c := &client{conn: nil}
	err := c.EnableUnit(context.Background(), "test")
	require.Error(t, err)
}

func TestNilConn_DisableUnit(t *testing.T) {
	c := &client{conn: nil}
	err := c.DisableUnit(context.Background(), "test")
	require.Error(t, err)
}

func TestNilConn_UnitStatus(t *testing.T) {
	c := &client{conn: nil}
	_, _, err := c.UnitStatus(context.Background(), "test")
	require.Error(t, err)
}

func TestNilConn_Reload(t *testing.T) {
	c := &client{conn: nil}
	err := c.Reload(context.Background())
	require.Error(t, err)
}

func TestNilConn_Close(t *testing.T) {
	c := &client{conn: nil}
	assert.NotPanics(t, func() {
		err := c.Close()
		assert.NoError(t, err)
	})
}

func TestClient_jobWait_NilConn(t *testing.T) {
	c := &client{conn: nil}
	assert.NotPanics(t, func() {
		_ = c.jobWait(context.Background(), func(ch chan<- string) (int, error) {
			return 0, errNoConn
		})
	})
}

func TestLogs_NilConn(t *testing.T) {
	orig := journalctlCmd
	journalctlCmd = "echo"
	defer func() { journalctlCmd = orig }()

	c := &client{conn: nil}
	r, err := c.Logs(context.Background(), "test-app", 50, false)
	require.NoError(t, err)
	require.NotNil(t, r)
	r.Close()
}

func TestLogs_Args(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "journalctl")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\""), 0755) //nolint:gosec
	require.NoError(t, err)

	orig := journalctlCmd
	journalctlCmd = script
	defer func() { journalctlCmd = orig }()

	c := &client{}

	t.Run("default no lines no follow", func(t *testing.T) {
		r, err := c.Logs(context.Background(), "test-app", 0, false)
		require.NoError(t, err)
		data, readErr := io.ReadAll(r)
		require.NoError(t, readErr)
		r.Close()
		args := string(data)
		assert.Contains(t, args, "-u test-app.service")
		assert.Contains(t, args, "-o cat")
		assert.NotContains(t, args, "-n")
		assert.NotContains(t, args, "-f")
	})

	t.Run("with lines", func(t *testing.T) {
		r, err := c.Logs(context.Background(), "test-app", 50, false)
		require.NoError(t, err)
		data, readErr := io.ReadAll(r)
		require.NoError(t, readErr)
		r.Close()
		args := string(data)
		assert.Contains(t, args, "-u test-app.service")
		assert.Contains(t, args, "-n 50")
		assert.NotContains(t, args, "-f")
	})

	t.Run("with follow", func(t *testing.T) {
		r, err := c.Logs(context.Background(), "test-app", 50, true)
		require.NoError(t, err)
		data, readErr := io.ReadAll(r)
		require.NoError(t, readErr)
		r.Close()
		args := string(data)
		assert.Contains(t, args, "-u test-app.service")
		assert.Contains(t, args, "-n 50")
		assert.Contains(t, args, "-f")
	})
}

func TestSetupLogging_AddsDirectives(t *testing.T) {
	dir := t.TempDir()
	origUnit := unitPath
	origRotate := logrotateDir
	origSysctl := systemctlCmd
	unitPath = func(name string) string { return filepath.Join(dir, name+".service") }
	logrotateDir = dir
	systemctlCmd = "echo"
	defer func() {
		unitPath = origUnit
		logrotateDir = origRotate
		systemctlCmd = origSysctl
	}()

	unitContent := `[Unit]
Description=test

[Service]
ExecStart=/usr/bin/test

[Install]
WantedBy=multi-user.target
`
	err := os.WriteFile(filepath.Join(dir, "test-app.service"), []byte(unitContent), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.SetupLogging(context.Background(), "test-app", "/var/log/test.log", "10M", 3)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "test-app.service"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "StandardOutput=append:/var/log/test.log")
	assert.Contains(t, content, "StandardError=append:/var/log/test.log")

	rotateData, err := os.ReadFile(filepath.Join(dir, "vigil-test-app"))
	require.NoError(t, err)
	assert.Contains(t, string(rotateData), "/var/log/test.log")
}

func TestSetupLogging_NoServiceSection(t *testing.T) {
	dir := t.TempDir()
	origUnit := unitPath
	origRotate := logrotateDir
	origSysctl := systemctlCmd
	unitPath = func(name string) string { return filepath.Join(dir, name+".service") }
	logrotateDir = dir
	systemctlCmd = "echo"
	defer func() {
		unitPath = origUnit
		logrotateDir = origRotate
		systemctlCmd = origSysctl
	}()

	unitContent := `[Unit]
Description=test
`
	err := os.WriteFile(filepath.Join(dir, "missing-section.service"), []byte(unitContent), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.SetupLogging(context.Background(), "missing-section", "/var/log/test.log", "10M", 3)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "missing-section.service"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "[Service]")
	assert.Contains(t, content, "StandardOutput=append:/var/log/test.log")
	assert.Contains(t, content, "StandardError=append:/var/log/test.log")
}

func TestSetupLogging_ExistingDirectives(t *testing.T) {
	dir := t.TempDir()
	origUnit := unitPath
	origRotate := logrotateDir
	unitPath = func(name string) string { return filepath.Join(dir, name+".service") }
	logrotateDir = dir
	defer func() {
		unitPath = origUnit
		logrotateDir = origRotate
	}()

	unitContent := `[Service]
StandardOutput=journal
StandardError=journal
`
	err := os.WriteFile(filepath.Join(dir, "already.service"), []byte(unitContent), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.SetupLogging(context.Background(), "already", "/var/log/test.log", "10M", 3)
	require.NoError(t, err)

	// Should not modify file
	data, err := os.ReadFile(filepath.Join(dir, "already.service"))
	require.NoError(t, err)
	assert.Equal(t, unitContent, string(data))
}

func TestSetupLogging_NonExistentUnit(t *testing.T) {
	c := &client{}
	err := c.SetupLogging(context.Background(), "nonexistent", "/var/log/test.log", "10M", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading unit file")
}

func TestRemoveLogging_RemovesDirectives(t *testing.T) {
	dir := t.TempDir()
	origUnit := unitPath
	origRotate := logrotateDir
	origSysctl := systemctlCmd
	unitPath = func(name string) string { return filepath.Join(dir, name+".service") }
	logrotateDir = dir
	systemctlCmd = "echo"
	defer func() {
		unitPath = origUnit
		logrotateDir = origRotate
		systemctlCmd = origSysctl
	}()

	unitContent := `[Service]
StandardOutput=append:/var/log/test.log
StandardError=append:/var/log/test.log
ExecStart=/usr/bin/test
`
	err := os.WriteFile(filepath.Join(dir, "test-app.service"), []byte(unitContent), 0600)
	require.NoError(t, err)

	rotatePath := filepath.Join(dir, "vigil-test-app")
	err = os.WriteFile(rotatePath, []byte("logrotate content"), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.RemoveLogging(context.Background(), "test-app")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "test-app.service"))
	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, "StandardOutput")
	assert.NotContains(t, content, "StandardError")
	assert.Contains(t, content, "ExecStart")

	_, err = os.Stat(rotatePath)
	assert.True(t, os.IsNotExist(err))
}

func TestReloadAndRestart_Error(t *testing.T) {
	dir := t.TempDir()
	origUnit := unitPath
	origRotate := logrotateDir
	origSysctl := systemctlCmd
	unitPath = func(name string) string { return filepath.Join(dir, name+".service") }
	logrotateDir = dir
	systemctlCmd = "false" // exits non-zero
	defer func() {
		unitPath = origUnit
		logrotateDir = origRotate
		systemctlCmd = origSysctl
	}()

	unitContent := `[Service]
ExecStart=/usr/bin/test
`
	err := os.WriteFile(filepath.Join(dir, "fail-restart.service"), []byte(unitContent), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.SetupLogging(context.Background(), "fail-restart", "/var/log/test.log", "10M", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon-reload failed")
}

func TestRemoveLogging_ErrorOnRemoveRotate(t *testing.T) {
	dir := t.TempDir()
	origUnit := unitPath
	origRotate := logrotateDir
	origSysctl := systemctlCmd
	unitPath = func(name string) string { return filepath.Join(dir, name+".service") }
	logrotateDir = filepath.Join(dir, "sub")
	systemctlCmd = "echo"
	defer func() {
		unitPath = origUnit
		logrotateDir = origRotate
		systemctlCmd = origSysctl
	}()
	// Make "sub" a file so os.Remove on sub/vigil-test-app fails with not a directory
	err := os.WriteFile(logrotateDir, []byte("not-a-dir"), 0600)
	require.NoError(t, err)

	unitContent := `[Service]
StandardOutput=append:/var/log/test.log
ExecStart=/usr/bin/test
`
	err = os.WriteFile(filepath.Join(dir, "test-app.service"), []byte(unitContent), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.RemoveLogging(context.Background(), "test-app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing logrotate config")
}

func TestRemoveLogging_Idempotent(t *testing.T) {
	dir := t.TempDir()
	origUnit := unitPath
	origRotate := logrotateDir
	origSysctl := systemctlCmd
	unitPath = func(name string) string { return filepath.Join(dir, name+".service") }
	logrotateDir = dir
	systemctlCmd = "echo"
	defer func() {
		unitPath = origUnit
		logrotateDir = origRotate
		systemctlCmd = origSysctl
	}()

	unitContent := `[Service]
ExecStart=/usr/bin/test
`
	err := os.WriteFile(filepath.Join(dir, "clean.service"), []byte(unitContent), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.RemoveLogging(context.Background(), "clean")
	require.NoError(t, err)

	// Verify unit file unchanged
	data, err := os.ReadFile(filepath.Join(dir, "clean.service"))
	require.NoError(t, err)
	assert.Equal(t, unitContent, string(data))
}
