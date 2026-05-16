package handlers

import (
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/auth"
)

// HealthHandler handles health and readiness check endpoints.
type HealthHandler struct {
	db        *gorm.DB
	startTime time.Time
	version   string
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *gorm.DB, version string) *HealthHandler {
	return &HealthHandler{
		db:        db,
		startTime: time.Now(),
		version:   version,
	}
}

// Health returns the overall health status of the application.
// GET /health
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	dbOK := h.checkDB()

	status := "healthy"
	httpCode := fiber.StatusOK
	if !dbOK {
		status = "unhealthy"
		httpCode = fiber.StatusServiceUnavailable
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return c.Status(httpCode).JSON(fiber.Map{
		"status":      status,
		"version":     h.version,
		"uptime":      time.Since(h.startTime).String(),
		"uptime_secs": int(time.Since(h.startTime).Seconds()),
		"checks": fiber.Map{
			"database": boolToStatus(dbOK),
		},
		"runtime": fiber.Map{
			"go_version":  runtime.Version(),
			"goroutines":  runtime.NumGoroutine(),
			"alloc_mb":    memStats.Alloc / 1024 / 1024,
			"sys_mb":      memStats.Sys / 1024 / 1024,
			"num_gc":      memStats.NumGC,
		},
	})
}

// Ready returns whether the application is ready to serve traffic.
// Used by load balancers and Kubernetes readiness probes.
//
// 13.9 — deep checks. /health remains a liveness probe (process is
// running); /ready returns 503 unless every load-bearing dependency
// answers. Currently:
//   - DB ping
//   - Encryption keys loaded (MFA_ENCRYPTION_KEY parsed at boot)
// S3 head-bucket + OIDC discovery deferred until those backends are
// always-required.
//
// GET /ready
func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	dbOK := h.checkDB()
	keysOK := auth.EnsureKeysLoaded() == nil

	checks := fiber.Map{
		"database":         boolToStatus(dbOK),
		"encryption_keys":  boolToStatus(keysOK),
	}

	if !dbOK || !keysOK {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"ready":  false,
			"checks": checks,
		})
	}
	return c.JSON(fiber.Map{
		"ready":  true,
		"checks": checks,
	})
}

func (h *HealthHandler) checkDB() bool {
	sqlDB, err := h.db.DB()
	if err != nil {
		return false
	}
	if err := sqlDB.Ping(); err != nil {
		return false
	}
	return true
}

func boolToStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}
