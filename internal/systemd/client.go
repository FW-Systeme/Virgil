package systemd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coreos/go-systemd/v22/dbus"
)

var (
	_        Client = (*client)(nil)
	errNoConn       = errors.New("systemd dbus connection not available")
)

type client struct {
	conn *dbus.Conn
}

func New() (Client, error) {
	conn, err := dbus.NewSystemConnectionContext(context.Background())
	if err != nil {
		return nil, err
	}
	return &client{conn: conn}, nil
}

var unitPath = func(name string) string {
	return filepath.Join("/etc/systemd/system", name+".service")
}

func (c *client) StartUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.jobWait(ctx, func(ch chan<- string) (int, error) {
		return c.conn.StartUnitContext(ctx, name+".service", "replace", ch)
	})
}

func (c *client) StopUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.jobWait(ctx, func(ch chan<- string) (int, error) {
		return c.conn.StopUnitContext(ctx, name+".service", "replace", ch)
	})
}

func (c *client) RestartUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.jobWait(ctx, func(ch chan<- string) (int, error) {
		return c.conn.ReloadOrRestartUnitContext(ctx, name+".service", "replace", ch)
	})
}

func jobWait(ctx context.Context, fn func(chan<- string) (int, error)) error {
	ch := make(chan string, 1)
	_, err := fn(ch)
	if err != nil {
		return err
	}
	select {
	case result := <-ch:
		if result != "done" {
			return fmt.Errorf("job failed: %s", result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *client) jobWait(ctx context.Context, fn func(chan<- string) (int, error)) error {
	return jobWait(ctx, fn)
}

func (c *client) EnableUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	_, _, err := c.conn.EnableUnitFilesContext(ctx, []string{unitPath(name)}, false, false)
	return err
}

func (c *client) DisableUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	_, err := c.conn.DisableUnitFilesContext(ctx, []string{unitPath(name)}, false)
	return err
}

func (c *client) UnitStatus(ctx context.Context, name string) (activeState, subState string, err error) {
	if c.conn == nil {
		return "", "", errNoConn
	}
	units, err := c.conn.ListUnitsByNamesContext(ctx, []string{name + ".service"})
	if err != nil {
		return "", "", err
	}
	if len(units) == 0 {
		return "inactive", "dead", nil
	}
	return units[0].ActiveState, units[0].SubState, nil
}

func (c *client) CreateUnitFile(name string, content []byte) error {
	return os.WriteFile(unitPath(name), content, 0644) //nolint:gosec
}

func (c *client) RemoveUnitFile(name string) error {
	err := os.Remove(unitPath(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c *client) Reload(ctx context.Context) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.conn.ReloadContext(ctx)
}

func (c *client) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}
