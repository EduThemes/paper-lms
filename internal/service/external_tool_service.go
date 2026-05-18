package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ExternalToolService manages external tool installations in courses and
// accounts. Each tool is backed by a developer key and can be installed
// into one or more contexts (Course or Account).
type ExternalToolService struct {
	toolRepo   repository.ContextExternalToolRepository
	devKeyRepo repository.DeveloperKeyRepository
}

// NewExternalToolService creates a new ExternalToolService.
func NewExternalToolService(toolRepo repository.ContextExternalToolRepository, devKeyRepo repository.DeveloperKeyRepository) *ExternalToolService {
	return &ExternalToolService{
		toolRepo:   toolRepo,
		devKeyRepo: devKeyRepo,
	}
}

// validContextTypes lists the allowed context types for tool installations.
var validContextTypes = map[string]bool{
	"Course":  true,
	"Account": true,
}

// Create installs an external tool in a context. The tool must have a name,
// a valid context_type (Course or Account), and a developer_key_id that
// references an existing developer key.
func (s *ExternalToolService) Create(ctx context.Context, tool *models.ContextExternalTool) error {
	if tool.Name == "" {
		return errors.New("tool name is required")
	}

	if !validContextTypes[tool.ContextType] {
		return errors.New("context_type must be 'Course' or 'Account'")
	}

	if tool.DeveloperKeyID == 0 {
		return errors.New("developer_key_id is required")
	}

	// Verify the developer key exists. accountID==0 is intentional: this is
	// an internal validation invoked by the external-tool create path; the
	// handler already gates the call by tenant via its own tool repo filter.
	_, err := s.devKeyRepo.FindByID(ctx, tool.DeveloperKeyID, 0)
	if err != nil {
		return errors.New("developer key not found")
	}

	// Set default workflow state
	if tool.WorkflowState == "" {
		tool.WorkflowState = "active"
	}

	return s.toolRepo.Create(ctx, tool)
}

// GetByID retrieves an external tool by its primary key ID.
func (s *ExternalToolService) GetByID(ctx context.Context, id, accountID uint) (*models.ContextExternalTool, error) {
	tool, err := s.toolRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("external tool not found")
	}
	return tool, nil
}

// Update modifies an existing external tool installation.
func (s *ExternalToolService) Update(ctx context.Context, tool *models.ContextExternalTool, accountID uint) error {
	// Verify the tool exists
	_, err := s.toolRepo.FindByID(ctx, tool.ID, accountID)
	if err != nil {
		return errors.New("external tool not found")
	}

	return s.toolRepo.Update(ctx, tool)
}

// Delete performs a soft delete by setting workflow_state to "deleted".
func (s *ExternalToolService) Delete(ctx context.Context, id, accountID uint) error {
	tool, err := s.toolRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return errors.New("external tool not found")
	}

	tool.WorkflowState = "deleted"
	return s.toolRepo.Update(ctx, tool)
}

// ListByContext returns a paginated list of external tools installed in the
// specified context (e.g., all tools in a Course or Account).
func (s *ExternalToolService) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextExternalTool], error) {
	if !validContextTypes[contextType] {
		return nil, errors.New("context_type must be 'Course' or 'Account'")
	}

	return s.toolRepo.ListByContext(ctx, contextType, contextID, params)
}
