package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
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

// SettingsLookupFunc is the type S3Backend accepts for resolving
// live config values from the Settings Engine. Function-typed (not
// interface-typed) for the same cycle-breaking reason documented on
// service.SettingsLookupFunc / auth.SettingsLookupFunc — the storage
// package must not import internal/service/settings (which transitively
// imports internal/auth for secretbox).
//
// Empty string + nil error means "no value in the resolution chain"
// (the boot-time snapshot wins for that field); non-nil error means
// "the lookup itself failed transiently."
type SettingsLookupFunc func(ctx context.Context, key string) (string, error)

// S3Backend stores files in an S3-compatible object store.
//
// Design choice (Wave 6 — full dual-cache):
//
// The backend holds a boot-time S3Config snapshot AND a SettingsLookupFunc.
// On every Put/Get/Delete/URL the backend resolves the 5 storage.s3.*
// keys via the lookup, merges them onto the boot snapshot (settings
// wins; empty falls through to boot), and compares the result to the
// last-resolved config. The underlying *s3.Client is rebuilt ONLY when
// the resolved config has changed.
//
// End state:
//   - env-only deployments: lookup returns the same env-resolved values
//     forever; lastResolved never changes; the SDK client is built once
//     at boot and reused for every request.
//   - settings-driven deployments: lookup returns the override; the
//     client rebuilds the first time after a super-admin saves new
//     creds, then stays cached until the next change.
//
// A nil lookup is allowed and means "env-only" — the boot client is
// used unconditionally with no per-request resolution. This keeps tests
// trivial and lets the local-storage default path skip the wiring.
type S3Backend struct {
	expiry time.Duration

	// boot is the snapshot resolved at NewS3Backend time (env + flags).
	// It's the floor of the resolution chain: any field the lookup
	// returns empty for falls through to this value.
	boot S3Config

	lookup SettingsLookupFunc

	mu           sync.RWMutex
	client       *s3.Client // current cached client (built from lastResolved)
	lastResolved S3Config   // the config that built `client`
}

// NewS3Backend creates an S3Backend with the given boot-time
// configuration and an optional Settings Engine lookup. Pass nil for
// `lookup` to disable per-request resolution (env-only mode).
func NewS3Backend(ctx context.Context, cfg S3Config, lookup SettingsLookupFunc) (*S3Backend, error) {
	if cfg.URLExpiry == 0 {
		cfg.URLExpiry = 1 * time.Hour
	}

	client, err := buildS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &S3Backend{
		expiry:       cfg.URLExpiry,
		boot:         cfg,
		lookup:       lookup,
		client:       client,
		lastResolved: cfg,
	}, nil
}

// SetSettingsLookup attaches (or clears) the Settings Engine lookup
// after construction. This exists because cmd/server/main.go wires the
// storage backend earlier in boot than settingsService — the closure
// that the lookup needs isn't available yet. Calling this with a
// non-nil lookup enables per-request resolution; calling it with nil
// reverts to env-only mode. Safe to call concurrently with in-flight
// operations.
func (b *S3Backend) SetSettingsLookup(lookup SettingsLookupFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lookup = lookup
}

// buildS3Client constructs an *s3.Client from a fully-resolved S3Config.
// Shared between boot-time construction and the per-request rebuild path.
func buildS3Client(ctx context.Context, cfg S3Config) (*s3.Client, error) {
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

	return s3.NewFromConfig(awsCfg, s3Opts...), nil
}

// resolveConfig builds the live S3Config by reading the 5 storage.s3.*
// keys via the lookup and merging them onto the boot snapshot. Empty
// lookup values fall through to the boot value for that field — this
// keeps env-only deployments working when the settings rows are absent.
//
// URLExpiry is not exposed via the Settings catalog today; it stays
// boot-pinned.
func (b *S3Backend) resolveConfig(ctx context.Context) (S3Config, error) {
	b.mu.RLock()
	lookup := b.lookup
	b.mu.RUnlock()

	if lookup == nil {
		return b.boot, nil
	}

	read := func(key, fallback string) (string, error) {
		v, err := lookup(ctx, key)
		if err != nil {
			return "", fmt.Errorf("storage: settings %s: %w", key, err)
		}
		if v == "" {
			return fallback, nil
		}
		return v, nil
	}

	bucket, err := read("storage.s3.bucket", b.boot.Bucket)
	if err != nil {
		return S3Config{}, err
	}
	region, err := read("storage.s3.region", b.boot.Region)
	if err != nil {
		return S3Config{}, err
	}
	endpoint, err := read("storage.s3.endpoint", b.boot.Endpoint)
	if err != nil {
		return S3Config{}, err
	}
	accessKey, err := read("storage.s3.access_key", b.boot.AccessKey)
	if err != nil {
		return S3Config{}, err
	}
	secretKey, err := read("storage.s3.secret_key", b.boot.SecretKey)
	if err != nil {
		return S3Config{}, err
	}

	return S3Config{
		Bucket:    bucket,
		Region:    region,
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		URLExpiry: b.boot.URLExpiry,
	}, nil
}

// configsEqual compares the five auth-bearing fields. URLExpiry is
// boot-pinned and not compared.
func configsEqual(a, b S3Config) bool {
	return a.Bucket == b.Bucket &&
		a.Region == b.Region &&
		a.Endpoint == b.Endpoint &&
		a.AccessKey == b.AccessKey &&
		a.SecretKey == b.SecretKey
}

// clientFor returns the cached *s3.Client and current bucket name,
// rebuilding the client only when the resolved config has changed
// since the last call. Lock-discipline: take RLock for the common
// "config unchanged" fast path; upgrade to Lock only when a rebuild
// is actually required.
func (b *S3Backend) clientFor(ctx context.Context) (*s3.Client, string, error) {
	resolved, err := b.resolveConfig(ctx)
	if err != nil {
		return nil, "", err
	}

	b.mu.RLock()
	if configsEqual(resolved, b.lastResolved) {
		client := b.client
		bucket := b.lastResolved.Bucket
		b.mu.RUnlock()
		return client, bucket, nil
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()
	// Re-check under write lock — another goroutine may have rebuilt
	// while we were waiting on the lock.
	if configsEqual(resolved, b.lastResolved) {
		return b.client, b.lastResolved.Bucket, nil
	}

	client, err := buildS3Client(ctx, resolved)
	if err != nil {
		return nil, "", err
	}
	b.client = client
	b.lastResolved = resolved
	return b.client, b.lastResolved.Bucket, nil
}

func (b *S3Backend) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	client, bucket, err := b.clientFor(ctx)
	if err != nil {
		return err
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   r,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if _, err := client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("storage: s3 put failed for key %s: %w", key, err)
	}
	return nil
}

func (b *S3Backend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	client, bucket, err := b.clientFor(ctx)
	if err != nil {
		return nil, err
	}

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 get failed for key %s: %w", key, err)
	}
	return output.Body, nil
}

func (b *S3Backend) Delete(ctx context.Context, key string) error {
	client, bucket, err := b.clientFor(ctx)
	if err != nil {
		return err
	}

	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("storage: s3 delete failed for key %s: %w", key, err)
	}
	return nil
}

func (b *S3Backend) URL(ctx context.Context, key string) (string, error) {
	client, bucket, err := b.clientFor(ctx)
	if err != nil {
		return "", err
	}

	presigner := s3.NewPresignClient(client)
	output, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(b.expiry))
	if err != nil {
		return "", fmt.Errorf("storage: s3 presign failed for key %s: %w", key, err)
	}
	return output.URL, nil
}
