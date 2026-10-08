package handlers

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// maxCourseStorageQuotaMB caps an admin-set course quota (1 TB).
const maxCourseStorageQuotaMB = 1024 * 1024

// storageQuotaMB is the course's own quota in MB for the course JSON
// (nil = the account default applies).
func storageQuotaMB(c *models.Course) *int64 {
	if c == nil || c.StorageQuota == nil {
		return nil
	}
	mb := *c.StorageQuota / (1024 * 1024)
	return &mb
}

// applyStorageQuotaMB sets course.StorageQuota from a storage_quota_mb
// value (G072). Only admins may change it (DB-backed role); null clears
// it back to the account default. Returns wrote=true after writing an
// error response.
func applyStorageQuotaMB(c *fiber.Ctx, users repository.UserRepository, course *models.Course, raw json.RawMessage) (bool, error) {
	if middleware.RoleRank(middleware.EffectiveRole(c, users)) < 1 {
		return true, responses.Forbidden(c, "Only an admin can change the course storage quota")
	}
	s := strings.TrimSpace(string(raw))
	if s == "null" || s == `""` {
		course.StorageQuota = nil
		return false, nil
	}
	var mb int64
	if err := json.Unmarshal(raw, &mb); err != nil || mb <= 0 || mb > maxCourseStorageQuotaMB {
		return true, responses.BadRequest(c, "storage_quota_mb must be a positive number of megabytes")
	}
	bytes := mb * 1024 * 1024
	course.StorageQuota = &bytes
	return false, nil
}
