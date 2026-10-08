package handlers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// CourseStorageQuotas is what the course and file handlers need from the
// storage quotas (G072 course quota, G120 account defaults and
// overrides). There is one source of truth, the storage_quotas table:
// *service.StorageQuotaService implements it.
type CourseStorageQuotas interface {
	Info(ctx context.Context, contextType string, contextID, accountID uint) (*service.QuotaInfo, error)
	ContextOverrideMB(ctx context.Context, contextType string, contextID uint) (*int64, error)
	SetContextQuota(ctx context.Context, contextType string, contextID, accountID uint, mb int64) (*service.QuotaInfo, error)
	CheckQuota(ctx context.Context, contextType string, contextID uint, addBytes int64) error
}

// maxCourseStorageQuotaMB caps an admin-set course quota (1 TB).
const maxCourseStorageQuotaMB = 1024 * 1024

// SetStorageQuotas wires the course storage quota (storage_quota_mb on
// GET/PUT /courses/:id).
func (h *CourseHandler) SetStorageQuotas(q CourseStorageQuotas) *CourseHandler {
	h.quotas = q
	return h
}

// courseJSONWithQuota is courseToJSON plus storage_quota_mb: the course's
// own quota in MB, or nil when the account default applies. The course
// was loaded tenant-scoped by the caller.
func (h *CourseHandler) courseJSONWithQuota(c *fiber.Ctx, courseID uint, m fiber.Map) fiber.Map {
	if h.quotas == nil {
		return m
	}
	mb, err := h.quotas.ContextOverrideMB(c.Context(), "Course", courseID)
	if err != nil {
		mb = nil
	}
	m["storage_quota_mb"] = mb
	return m
}

// applyStorageQuotaMB sets the course's own quota from a storage_quota_mb
// value (G072). Only admins may change it (DB-backed role); null clears
// it back to the account default. The course was loaded tenant-scoped,
// so accountID 0 (unscoped) is safe here. Returns wrote=true after
// writing an error response.
func applyStorageQuotaMB(c *fiber.Ctx, users repository.UserRepository, quotas CourseStorageQuotas, courseID uint, raw json.RawMessage) (bool, error) {
	if middleware.RoleRank(middleware.EffectiveRole(c, users)) < 1 {
		return true, responses.Forbidden(c, "Only an admin can change the course storage quota")
	}
	if quotas == nil {
		return true, responses.BadRequest(c, "Storage quotas are not configured")
	}
	mb := int64(-1) // clear the override
	if s := strings.TrimSpace(string(raw)); s != "null" && s != `""` {
		if err := json.Unmarshal(raw, &mb); err != nil || mb <= 0 || mb > maxCourseStorageQuotaMB {
			return true, responses.BadRequest(c, "storage_quota_mb must be a positive number of megabytes")
		}
	}
	if _, err := quotas.SetContextQuota(c.Context(), "Course", courseID, 0, mb); err != nil {
		return true, responses.InternalError(c, "Could not save the course storage quota")
	}
	return false, nil
}
