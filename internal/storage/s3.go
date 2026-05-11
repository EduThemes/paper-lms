package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config holds S3-compatible storage configuration.
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string // For MinIO, Cloudflare R2, etc.
	AccessKey string
	SecretKey string
	URLExpiry time.Duration // Presigned URL validity (default 1 hour)
}

// S3Backend stores files in an S3-compatible object store.
type S3Backend struct {
	client *s3.Client
	bucket string
	expiry time.Duration
}

// NewS3Backend creates an S3Backend with the given configuration.
func NewS3Backend(ctx context.Context, cfg S3Config) (*S3Backend, error) {
	if cfg.URLExpiry == 0 {
		cfg.URLExpiry = 1 * time.Hour
	}

	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to load AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for MinIO, R2
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Backend{
		client: client,
		bucket: cfg.Bucket,
		expiry: cfg.URLExpiry,
	}, nil
}

func (b *S3Backend) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
		Body:   r,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := b.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("storage: s3 put failed for key %s: %w", key, err)
	}
	return nil
}

func (b *S3Backend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 get failed for key %s: %w", key, err)
	}
	return output.Body, nil
}

func (b *S3Backend) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("storage: s3 delete failed for key %s: %w", key, err)
	}
	return nil
}

func (b *S3Backend) URL(ctx context.Context, key string) (string, error) {
	presigner := s3.NewPresignClient(b.client)
	output, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(b.expiry))
	if err != nil {
		return "", fmt.Errorf("storage: s3 presign failed for key %s: %w", key, err)
	}
	return output.URL, nil
}
