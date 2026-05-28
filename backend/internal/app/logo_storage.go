package app

import (
	"context"
	"io"
)

func (a *App) uploadTenantLogo(ctx context.Context, key, contentType string, body io.Reader) (string, error) {
	return a.uploadTenantLogoToR2(ctx, key, contentType, body)
}
