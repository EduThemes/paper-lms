// Integration test for the LDAP bind-password backfill cmd.
//
// Mirrors the project's PARITY_DB_URL skip pattern (see
// internal/repository/postgres/user_test.go and
// internal/service/gamification/seed_test.go). When neither
// PARITY_DB_URL nor DATABASE_URL is set, the test skips.
//
// Pins the idempotency contract: after one successful encryption run,
// the second invocation finds zero rows to encrypt.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// TestSelectBackfillCandidates_Idempotent pins the load-bearing
// guarantee: a second run after a successful pass returns zero rows.
//
// Fixture: one tenant, two LDAP providers. Provider A is already
// encrypted (plaintext column empty); provider B has only plaintext.
// First scan returns {B}. After encrypting B, second scan returns {}.
func TestSelectBackfillCandidates_Idempotent(t *testing.T) {
	setEncryptionKey(t)
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()

	// Tenant.
	tenant := models.Account{Name: "Test Tenant", WorkflowState: "active"}
	if err := g.WithContext(ctx).Create(&tenant).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}

	// Provider A: already-encrypted, no plaintext. Must be skipped.
	encA, err := auth.Encrypt([]byte("already-encrypted-secret-A"))
	if err != nil {
		t.Fatalf("encrypt provider A fixture: %v", err)
	}
	providerA := models.AuthenticationProvider{
		AccountID:                 tenant.ID,
		AuthType:                  "ldap",
		LDAPHost:                  "ldap-a.test",
		LDAPBindDN:                "cn=svc,dc=a,dc=test",
		LDAPBindPassword:          "", // plaintext column empty
		LDAPBindPasswordEncrypted: encA,
		WorkflowState:             "active",
	}
	if err := g.WithContext(ctx).Create(&providerA).Error; err != nil {
		t.Fatalf("seed provider A: %v", err)
	}

	// Provider B: plaintext-only. The candidate.
	providerB := models.AuthenticationProvider{
		AccountID:                 tenant.ID,
		AuthType:                  "ldap",
		LDAPHost:                  "ldap-b.test",
		LDAPBindDN:                "cn=svc,dc=b,dc=test",
		LDAPBindPassword:          "plaintext-secret-B",
		LDAPBindPasswordEncrypted: nil,
		WorkflowState:             "active",
	}
	if err := g.WithContext(ctx).Create(&providerB).Error; err != nil {
		t.Fatalf("seed provider B: %v", err)
	}

	// Negative-control: a deleted provider with a plaintext password
	// must also be skipped (the workflow_state guard).
	deletedC := models.AuthenticationProvider{
		AccountID:        tenant.ID,
		AuthType:         "ldap",
		LDAPHost:         "ldap-c.test",
		LDAPBindPassword: "should-not-touch",
		WorkflowState:    "deleted",
	}
	if err := g.WithContext(ctx).Create(&deletedC).Error; err != nil {
		t.Fatalf("seed provider C: %v", err)
	}

	// Negative-control: a non-LDAP provider (SAML, OIDC, etc.) shouldn't
	// be in scope even if it has stray data in unrelated columns.
	samlD := models.AuthenticationProvider{
		AccountID:     tenant.ID,
		AuthType:      "saml",
		WorkflowState: "active",
	}
	if err := g.WithContext(ctx).Create(&samlD).Error; err != nil {
		t.Fatalf("seed provider D: %v", err)
	}

	// --- Pass 1: should find exactly provider B. ---
	candidates, err := selectBackfillCandidates(ctx, g, 0, 0)
	if err != nil {
		t.Fatalf("pass 1 select: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("pass 1: want 1 candidate, got %d", len(candidates))
	}
	if candidates[0].ID != providerB.ID {
		t.Fatalf("pass 1: want provider B (id=%d), got id=%d", providerB.ID, candidates[0].ID)
	}

	// Encrypt the candidate the same way the cmd does.
	ct, err := auth.Encrypt([]byte(candidates[0].LDAPBindPassword))
	if err != nil {
		t.Fatalf("encrypt candidate: %v", err)
	}
	if err := g.WithContext(ctx).
		Model(&models.AuthenticationProvider{}).
		Where("id = ?", candidates[0].ID).
		Update("ldap_bind_password_encrypted", ct).Error; err != nil {
		t.Fatalf("update candidate: %v", err)
	}

	// --- Pass 2: idempotency — no rows remain. ---
	candidates2, err := selectBackfillCandidates(ctx, g, 0, 0)
	if err != nil {
		t.Fatalf("pass 2 select: %v", err)
	}
	if len(candidates2) != 0 {
		t.Fatalf("pass 2: want 0 candidates (idempotent), got %d", len(candidates2))
	}

	// Round-trip — the ciphertext we wrote decrypts back to the original.
	var after models.AuthenticationProvider
	if err := g.WithContext(ctx).First(&after, providerB.ID).Error; err != nil {
		t.Fatalf("reload provider B: %v", err)
	}
	plain, err := auth.Decrypt(after.LDAPBindPasswordEncrypted)
	if err != nil {
		t.Fatalf("decrypt provider B: %v", err)
	}
	if string(plain) != "plaintext-secret-B" {
		t.Fatalf("round-trip mismatch: want %q, got %q", "plaintext-secret-B", string(plain))
	}
}

// TestSelectBackfillCandidates_TenantFilter verifies that --tenant
// scopes the scan to a single account_id.
func TestSelectBackfillCandidates_TenantFilter(t *testing.T) {
	setEncryptionKey(t)
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()

	tenant1 := models.Account{Name: "Tenant 1", WorkflowState: "active"}
	tenant2 := models.Account{Name: "Tenant 2", WorkflowState: "active"}
	if err := g.WithContext(ctx).Create(&tenant1).Error; err != nil {
		t.Fatalf("seed tenant 1: %v", err)
	}
	if err := g.WithContext(ctx).Create(&tenant2).Error; err != nil {
		t.Fatalf("seed tenant 2: %v", err)
	}

	p1 := models.AuthenticationProvider{AccountID: tenant1.ID, AuthType: "ldap", LDAPBindPassword: "t1-secret", WorkflowState: "active"}
	p2 := models.AuthenticationProvider{AccountID: tenant2.ID, AuthType: "ldap", LDAPBindPassword: "t2-secret", WorkflowState: "active"}
	if err := g.WithContext(ctx).Create(&p1).Error; err != nil {
		t.Fatalf("seed p1: %v", err)
	}
	if err := g.WithContext(ctx).Create(&p2).Error; err != nil {
		t.Fatalf("seed p2: %v", err)
	}

	got, err := selectBackfillCandidates(ctx, g, tenant2.ID, 0)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 candidate (tenant 2 only), got %d", len(got))
	}
	if got[0].AccountID != tenant2.ID {
		t.Fatalf("want account_id=%d, got %d", tenant2.ID, got[0].AccountID)
	}
}

// --- helpers ---

func setEncryptionKey(t *testing.T) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("gen key: %v", err)
	}
	t.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(raw))
	if err := auth.EnsureKeysLoaded(); err != nil {
		t.Fatalf("load keys: %v", err)
	}
}

func freshDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	parityURL := os.Getenv("PARITY_DB_URL")
	if parityURL == "" {
		parityURL = os.Getenv("DATABASE_URL")
	}
	if parityURL == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run encrypt-ldap-passwords integration tests")
	}

	adminURL := swapDatabase(t, parityURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}

	name := fmt.Sprintf("paper_lms_ldapenc_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		_ = admin.Close()
		t.Fatalf("create db %s: %v", name, err)
	}

	dbURL := swapDatabase(t, parityURL, name)
	bs, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open %s: %v", dbURL, err)
	}
	if _, err := bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		_ = bs.Close()
		t.Fatalf("create extension: %v", err)
	}
	_ = bs.Close()

	g, err := gorm.Open(pgdriver.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	if err := db.MigrateUp(g); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	cleanup := func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		_ = admin.Close()
	}
	return g, cleanup
}

func swapDatabase(t *testing.T, rawURL, dbName string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
}
