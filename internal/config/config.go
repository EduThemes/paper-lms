package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	Environment     string
	FrontendURL     string
	FileStoragePath string
	MaxUploadSize   int
	AutoMigrate     bool
	// Storage backend
	StorageBackend string // "local" or "s3"
	S3Bucket       string
	S3Region       string
	S3Endpoint     string // For MinIO, Cloudflare R2, etc.
	S3AccessKey    string
	S3SecretKey    string
	// SAML SSO
	SAMLEntityID string
	SAMLCertFile string
	SAMLKeyFile  string
	// SetupBootstrapToken gates POST /setup/complete. When set, the
	// request must echo it in the X-Setup-Token header. Empty means the
	// wizard is open (development default). In production with no admin
	// yet, leaving this empty triggers a SECURITY warning at boot — see
	// Validate().
	SetupBootstrapToken string
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "3000"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://paper:paper@localhost:5432/paper_lms?sslmode=disable"),
		JWTSecret:       getEnv("JWT_SECRET", "your-super-secret-key-change-this-in-production"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		FrontendURL:     getEnv("FRONTEND_URL", "http://localhost:5173"),
		FileStoragePath: getEnv("FILE_STORAGE_PATH", "./storage/files"),
		MaxUploadSize:   getEnvInt("MAX_UPLOAD_SIZE_MB", 500),
		SAMLEntityID:    getEnv("SAML_ENTITY_ID", ""),
		SAMLCertFile:    getEnv("SAML_CERT_FILE", ""),
		SAMLKeyFile:     getEnv("SAML_KEY_FILE", ""),
		AutoMigrate:     getEnv("AUTO_MIGRATE", "true") == "true",
		StorageBackend:  getEnv("STORAGE_BACKEND", "local"),
		S3Bucket:        getEnv("S3_BUCKET", ""),
		S3Region:        getEnv("S3_REGION", "us-east-1"),
		S3Endpoint:      getEnv("S3_ENDPOINT", ""),
		S3AccessKey:     getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:     getEnv("S3_SECRET_KEY", ""),
		SetupBootstrapToken: getEnv("SETUP_BOOTSTRAP_TOKEN", ""),
	}
}

const defaultJWTSecret = "your-super-secret-key-change-this-in-production"

// Validate checks that critical configuration values are set for production.
// In production, it will fatally exit if the JWT secret is the default value.
// In development, it will auto-generate a random secret if the default is detected.
func (c *Config) Validate() {
	if c.JWTSecret == defaultJWTSecret {
		if c.Environment == "production" {
			log.Fatal("FATAL: JWT_SECRET must be changed from the default value in production. Set the JWT_SECRET environment variable to a secure random string (at least 32 characters).")
		}
		// In development, generate a random secret and warn
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatal("FATAL: Could not generate random JWT secret: ", err)
		}
		c.JWTSecret = hex.EncodeToString(b)
		fmt.Println("WARNING: JWT_SECRET not set — using auto-generated secret. Sessions will not persist across restarts. Set JWT_SECRET for stable sessions.")
	}

	if c.Environment == "production" {
		// A short HMAC signing key is brute-forceable. The default-value
		// check above catches the placeholder; this catches a real-but-weak
		// secret. Kept a loud WARNING (not fatal) so it can't brick a running
		// deploy whose key length hasn't been verified — promote to log.Fatal
		// once operators confirm every environment uses a 32+ char secret.
		if len(c.JWTSecret) < 32 {
			log.Printf("WARNING: JWT_SECRET is only %d characters; a session-signing key should be at least 32 characters of high-entropy randomness. Rotate it.", len(c.JWTSecret))
		}
		if c.DatabaseURL == "postgres://paper:paper@localhost:5432/paper_lms?sslmode=disable" {
			log.Fatal("FATAL: DATABASE_URL must be configured for production. Do not use the default development database URL.")
		}
		if c.FrontendURL == "http://localhost:5173" || c.FrontendURL == "" {
			log.Fatal("FATAL: FRONTEND_URL must be configured for production. Set it to your production domain (e.g., https://app.paperlms.org).")
		}
		if c.AutoMigrate {
			fmt.Println("WARNING: AUTO_MIGRATE=true in production. Set AUTO_MIGRATE=false to use versioned SQL migrations instead of GORM AutoMigrate.")
		}
		if c.SetupBootstrapToken == "" {
			// Boot-time warning only. The handler still works because
			// the hasAdmin recheck closes the wizard once a real admin
			// exists; the risk is the window between deploy and
			// finish-wizard, which token gating eliminates.
			fmt.Println("WARNING: SETUP_BOOTSTRAP_TOKEN is unset in production. The /setup/complete wizard is open until the first admin is created — set this env var to require an X-Setup-Token header for protection against attackers racing your operator on a fresh deploy.")
		}
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
