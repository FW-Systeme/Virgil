package update

import (
	"context"
	"errors"
)

var (
	ErrLocked       = errors.New("update lock held")
	ErrNoPackage    = errors.New("no package found in incoming dir")
	ErrIntegrity    = errors.New("package integrity check failed")
	ErrSmokeTest    = errors.New("smoke test failed")
	ErrDepsFailed   = errors.New("dependency installation failed")
	ErrRolledBack   = errors.New("update failed, rolled back")
)

type Service interface {
	Update(ctx context.Context, name string, version string) error
}

type RestartFunc func(ctx context.Context, name string) error
