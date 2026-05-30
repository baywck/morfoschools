package app

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (a *App) uploadTenantLogoToR2(ctx context.Context, key, contentType string, body io.Reader) (string, error) {
	if a.cfg.R2Endpoint == "" || a.cfg.R2AccessKey == "" || a.cfg.R2SecretKey == "" || a.cfg.R2Bucket == "" {
		return "", fmt.Errorf("R2 storage is not configured")
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(a.cfg.R2Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(a.cfg.R2AccessKey, a.cfg.R2SecretKey, ""),
		UsePathStyle: true,
	})
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(a.cfg.R2Bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("no-store, max-age=0, must-revalidate"),
	})
	if err != nil {
		return "", err
	}
	publicBase := strings.TrimRight(a.cfg.R2PublicURL, "/")
	if publicBase == "" {
		publicBase = strings.TrimRight(a.cfg.R2Endpoint, "/") + "/" + a.cfg.R2Bucket
	}
	return publicBase + "/" + key, nil
}

func tenantLogoObjectKey(tenantName, filename, contentType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
			ext = exts[0]
		}
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg":
	default:
		ext = ".bin"
	}
	slug := slugifySubjectName(tenantName)
	if slug == "" {
		slug = "tenant"
	}
	return "morfoschools/tenants/logo/" + slug + ext
}

func allowedLogoContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/svg+xml":
		return true
	default:
		return false
	}
}
