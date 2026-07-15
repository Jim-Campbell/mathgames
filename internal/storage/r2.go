// Package storage is the Cloudflare R2 client for video clips, copied from
// ~/projects/food's internal/storage/r2.go (same R2_* env var names, so
// credentials are reusable between the apps).
package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Client struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

func NewR2Client(accountID, accessKey, secretKey, bucket, publicURL string) *R2Client {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	})

	return &R2Client{client: client, bucket: bucket, publicURL: publicURL}
}

// Upload stores data at key with contentType and returns its public URL.
// Per CLAUDE.md's video-clips build prompt, clips serve directly from this
// public URL (unguessable key) -- viewable by anyone who has it. A
// bearer-authed streaming proxy (range requests) is future hardening if
// that ever matters.
func (r *R2Client) Upload(ctx context.Context, key, contentType string, data io.Reader) (string, error) {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        data,
	})
	if err != nil {
		return "", fmt.Errorf("r2 put object: %w", err)
	}
	return r.publicURL + "/" + key, nil
}

func (r *R2Client) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("r2 delete object: %w", err)
	}
	return nil
}
