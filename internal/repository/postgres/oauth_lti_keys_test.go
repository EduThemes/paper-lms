package postgres_test

// DB-backed tests for the integrations parity wave: OAuth2 authorization
// codes (B192), the persisted LTI platform key (B181) and the client
// secret hashing migration (B191). Gated on PARITY_DB_URL / DATABASE_URL
// via freshDB.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestOAuthAuthorizationCodeRepo_ConsumeIsSingleUse(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := postgres.NewOAuthAuthorizationCodeRepository(g)

	live := &models.OAuthAuthorizationCode{
		CodeHash: "hash-live", UserID: 10, DeveloperKeyID: 5, ClientID: "dk_example",
		RedirectURI: "https://app.example.com/cb", Scopes: "[]", ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	stale := &models.OAuthAuthorizationCode{
		CodeHash: "hash-stale", UserID: 10, DeveloperKeyID: 5, ClientID: "dk_example",
		RedirectURI: "https://app.example.com/cb", ExpiresAt: time.Now().Add(-time.Minute),
	}
	for _, c := range []*models.OAuthAuthorizationCode{live, stale} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	got, err := repo.Consume(ctx, "hash-live")
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if got.UserID != 10 || got.ClientID != "dk_example" || got.RedirectURI != "https://app.example.com/cb" {
		t.Fatalf("consumed row mismatch: %+v", got)
	}
	if _, err := repo.Consume(ctx, "hash-live"); err == nil {
		t.Fatal("a code must not be redeemable twice")
	}

	if err := repo.DeleteExpired(ctx); err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if _, err := repo.Consume(ctx, "hash-stale"); err == nil {
		t.Fatal("expired codes must be purged")
	}
}

func TestLTIPlatformKeyRepo_CreateIfAbsentNeverOverwrites(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := postgres.NewLTIPlatformKeyRepository(g)

	first := &models.LTIPlatformKey{Name: "default", KeyID: "kid-first", PrivateKeyEncrypted: []byte{1, 2, 3}}
	second := &models.LTIPlatformKey{Name: "default", KeyID: "kid-second", PrivateKeyEncrypted: []byte{4, 5, 6}}
	if err := repo.CreateIfAbsent(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := repo.CreateIfAbsent(ctx, second); err != nil {
		t.Fatalf("create second (race loser) must not error: %v", err)
	}
	row, err := repo.FindByName(ctx, "default")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if row.KeyID != "kid-first" {
		t.Fatalf("the first replica's key must win; got %q", row.KeyID)
	}
}

func TestMigration000075_HashesPlaintextClientSecrets(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedTenantAccountForWallet(t, g, 1)

	if err := g.Exec(`INSERT INTO developer_keys (account_id, name, client_id, client_secret, workflow_state)
		VALUES (1, 'Legacy App', 'dk_legacy', 'legacy-plain-secret', 'active')`).Error; err != nil {
		t.Fatalf("seed developer key: %v", err)
	}

	up, err := os.ReadFile("../../db/migrations/000075_hash_developer_key_secrets.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	// Re-run (the chain already ran it once on an empty table); it is idempotent.
	for i := 0; i < 2; i++ {
		if err := g.Exec(string(up)).Error; err != nil {
			t.Fatalf("apply migration: %v", err)
		}
	}

	var key models.DeveloperKey
	if err := g.Where("client_id = ?", "dk_legacy").First(&key).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if key.ClientSecret != service.HashClientSecret("legacy-plain-secret") {
		t.Fatalf("expected the Go-compatible hash, got %q", key.ClientSecret)
	}
	if !service.VerifyClientSecret(&key, "legacy-plain-secret") {
		t.Fatal("existing clients must keep authenticating after the migration")
	}
}
