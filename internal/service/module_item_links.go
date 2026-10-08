package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Module item content types for files and LTI tools (Canvas ContentTag
// content_type values).
const (
	ModuleItemContentAttachment   = "Attachment"
	ModuleItemContentExternalTool = "ContextExternalTool"
)

// ErrModuleToolUnavailable is returned when an External Tool item's tool
// is gone, inactive or not installed for the item's course.
var ErrModuleToolUnavailable = errors.New("external tool is not available in this course")

// SetLinkRepos wires the repositories File and External Tool module
// items need (G064).
func (s *ModuleService) SetLinkRepos(attachments repository.AttachmentRepository, tools repository.ContextExternalToolRepository, devKeys repository.DeveloperKeyRepository, courses repository.CourseRepository) {
	s.attachmentRepo = attachments
	s.toolRepo = tools
	s.devKeyRepo = devKeys
	s.courseRepo = courses
}

// ValidateFileItem checks that attachmentID is a live file of courseID
// in the caller's tenant and returns its display name.
func (s *ModuleService) ValidateFileItem(ctx context.Context, attachmentID, courseID, accountID uint) (string, error) {
	if s.attachmentRepo == nil {
		return "", ErrModuleContentNotInCourse
	}
	att, err := s.attachmentRepo.FindByID(ctx, attachmentID, accountID)
	if err != nil || att == nil || att.ContextType != "Course" || att.ContextID != courseID || att.WorkflowState == "deleted" {
		return "", ErrModuleContentNotInCourse
	}
	return att.DisplayName, nil
}

// ExternalToolForCourse loads toolID and verifies it is an active tool
// installed in courseID or in the course's account (the two contexts a
// course can launch from), inside the caller's tenant.
func (s *ModuleService) ExternalToolForCourse(ctx context.Context, toolID, courseID, accountID uint) (*models.ContextExternalTool, error) {
	if s.toolRepo == nil {
		return nil, ErrModuleToolUnavailable
	}
	tool, err := s.toolRepo.FindByID(ctx, toolID, accountID)
	if err != nil || tool == nil || tool.WorkflowState != "active" {
		return nil, ErrModuleToolUnavailable
	}
	switch tool.ContextType {
	case "Course":
		if tool.ContextID != courseID {
			return nil, ErrModuleToolUnavailable
		}
	case "Account":
		if s.courseRepo == nil {
			return nil, ErrModuleToolUnavailable
		}
		course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
		if err != nil || course == nil || course.AccountID != tool.ContextID {
			return nil, ErrModuleToolUnavailable
		}
	default:
		return nil, ErrModuleToolUnavailable
	}
	return tool, nil
}

// ValidateToolLaunchURL checks an External Tool item's launch URL. It is
// optional (the tool's own URL is used when blank); when given it must
// be an absolute http(s) URL on the tool's domain, so an item cannot
// point a tool launch at an arbitrary site. Paper never fetches it — the
// browser is sent there by the tool's OIDC flow.
func ValidateToolLaunchURL(tool *models.ContextExternalTool, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return tool.URL, nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", fmt.Errorf("launch url must be an absolute http(s) URL")
	}
	allowed := strings.ToLower(strings.TrimSpace(tool.Domain))
	if allowed == "" && tool.URL != "" {
		if tu, err := url.Parse(tool.URL); err == nil {
			allowed = strings.ToLower(tu.Hostname())
		}
	}
	host := strings.ToLower(u.Hostname())
	if allowed != "" && host != allowed && !strings.HasSuffix(host, "."+allowed) {
		return "", fmt.Errorf("launch url must be on the tool's domain (%s)", allowed)
	}
	return u.String(), nil
}

// ToolLaunch is what the browser posts to /api/v1/lti/oidc/login to
// start an LTI 1.3 launch of an External Tool module item.
type ToolLaunch struct {
	ClientID       string `json:"client_id"`
	TargetLinkURI  string `json:"target_link_uri"`
	LTIMessageHint string `json:"lti_message_hint"`
	ToolName       string `json:"tool_name"`
}

// ExternalToolLaunch builds the launch parameters for an External Tool
// item of courseID. The message hint carries the course, a resource
// link id derived from the item, and the tool's developer key, which
// the authorization step re-validates (course membership, install).
func (s *ModuleService) ExternalToolLaunch(ctx context.Context, item *models.ContentTag, courseID, accountID uint) (*ToolLaunch, error) {
	if item.ContentType != ModuleItemContentExternalTool || item.ContentID == nil {
		return nil, ErrModuleToolUnavailable
	}
	tool, err := s.ExternalToolForCourse(ctx, *item.ContentID, courseID, accountID)
	if err != nil {
		return nil, err
	}
	if s.devKeyRepo == nil {
		return nil, ErrModuleToolUnavailable
	}
	key, err := s.devKeyRepo.FindByID(ctx, tool.DeveloperKeyID, accountID)
	if err != nil || key == nil || key.WorkflowState != "active" || !key.IsLTIKey {
		return nil, ErrModuleToolUnavailable
	}
	target := item.URL
	if target == "" {
		target = tool.URL
	}
	return &ToolLaunch{
		ClientID:       key.ClientID,
		TargetLinkURI:  target,
		LTIMessageHint: fmt.Sprintf("%d:module_item_%d:%d", courseID, item.ID, key.ID),
		ToolName:       tool.Name,
	}, nil
}
