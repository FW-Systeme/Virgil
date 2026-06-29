package cli

import (
	"context"

	"github.com/chris576/vigil/internal/process"
)

type ctxKey string

const pmKey ctxKey = "pm"

func pmCtx(ctx context.Context, pm *process.Manager) context.Context {
	return context.WithValue(ctx, pmKey, pm)
}

func pmFromCtx(ctx context.Context) (*process.Manager, bool) {
	pm, ok := ctx.Value(pmKey).(*process.Manager)
	return pm, ok
}
