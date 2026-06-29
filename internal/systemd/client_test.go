package systemd

import (
	"context"
	"fmt"
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
		err := os.WriteFile(path, []byte("content"), 0644)
		require.NoError(t, err)

		err = c.RemoveUnitFile("exists")
		assert.NoError(t, err)

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
