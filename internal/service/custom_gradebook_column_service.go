package service

import (
	"context"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// CustomGradebookColumnService implements Canvas-compatible custom gradebook
// columns business logic. Instructor-only RBAC is enforced at the handler/router
// layer; this service focuses on validation and atomic operations.
type CustomGradebookColumnService struct {
	colRepo  postgres.CustomGradebookColumnRepository
	dataRepo postgres.CustomColumnDatumRepository
}

func NewCustomGradebookColumnService(
	colRepo postgres.CustomGradebookColumnRepository,
	dataRepo postgres.CustomColumnDatumRepository,
) *CustomGradebookColumnService {
	return &CustomGradebookColumnService{colRepo: colRepo, dataRepo: dataRepo}
}

// MaxContentBytes mirrors Canvas's ~4KB limit on custom column datum content.
const MaxContentBytes = 4096

// ListColumns returns all non-deleted columns ordered by position.
func (s *CustomGradebookColumnService) ListColumns(ctx context.Context, courseID uint, includeHidden bool) ([]models.CustomGradebookColumn, error) {
	return s.colRepo.ListByCourse(ctx, courseID, includeHidden)
}

// CreateColumn appends a new column at the end of the course's column list.
func (s *CustomGradebookColumnService) CreateColumn(ctx context.Context, courseID uint, in *models.CustomGradebookColumn) (*models.CustomGradebookColumn, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, errors.New("title is required")
	}
	if len(in.Title) > 255 {
		return nil, errors.New("title must be 255 characters or fewer")
	}
	in.CourseID = courseID
	in.WorkflowState = "active"
	if in.Position <= 0 {
		next, err := s.colRepo.NextPosition(ctx, courseID)
		if err != nil {
			return nil, err
		}
		in.Position = next
	}
	if err := s.colRepo.Create(ctx, in); err != nil {
		return nil, err
	}
	return in, nil
}

// UpdateColumn applies a partial update to a column.
func (s *CustomGradebookColumnService) UpdateColumn(ctx context.Context, courseID, columnID uint, title *string, hidden, readOnly, teacherNotes *bool, position *int) (*models.CustomGradebookColumn, error) {
	col, err := s.colRepo.FindByID(ctx, columnID)
	if err != nil {
		return nil, errors.New("column not found")
	}
	if col.CourseID != courseID {
		return nil, errors.New("column does not belong to this course")
	}
	if title != nil {
		t := strings.TrimSpace(*title)
		if t == "" {
			return nil, errors.New("title is required")
		}
		if len(t) > 255 {
			return nil, errors.New("title must be 255 characters or fewer")
		}
		col.Title = t
	}
	if hidden != nil {
		col.Hidden = *hidden
	}
	if readOnly != nil {
		col.ReadOnly = *readOnly
	}
	if teacherNotes != nil {
		col.TeacherNotes = *teacherNotes
	}
	if position != nil && *position > 0 {
		col.Position = *position
	}
	if err := s.colRepo.Update(ctx, col); err != nil {
		return nil, err
	}
	return col, nil
}

// DeleteColumn soft-deletes the column. Existing datum rows remain (Canvas
// behavior: hidden, restorable). Hard removal of orphan data is left to a
// separate cleanup tool.
func (s *CustomGradebookColumnService) DeleteColumn(ctx context.Context, courseID, columnID uint) error {
	col, err := s.colRepo.FindByID(ctx, columnID)
	if err != nil {
		return errors.New("column not found")
	}
	if col.CourseID != courseID {
		return errors.New("column does not belong to this course")
	}
	return s.colRepo.Delete(ctx, columnID)
}

// Reorder updates positions of the listed column IDs in order. IDs not in the
// list keep their existing position.
func (s *CustomGradebookColumnService) Reorder(ctx context.Context, courseID uint, ids []uint) error {
	if len(ids) == 0 {
		return errors.New("order is required")
	}
	// Validate all IDs belong to the course.
	cols, err := s.colRepo.ListByCourse(ctx, courseID, true)
	if err != nil {
		return err
	}
	owned := make(map[uint]bool, len(cols))
	for _, c := range cols {
		owned[c.ID] = true
	}
	for _, id := range ids {
		if !owned[id] {
			return errors.New("one or more columns do not belong to this course")
		}
	}
	return s.colRepo.Reorder(ctx, courseID, ids)
}

// ListData returns the per-user values for a single column.
func (s *CustomGradebookColumnService) ListData(ctx context.Context, courseID, columnID uint) ([]models.CustomColumnDatum, error) {
	col, err := s.colRepo.FindByID(ctx, columnID)
	if err != nil {
		return nil, errors.New("column not found")
	}
	if col.CourseID != courseID {
		return nil, errors.New("column does not belong to this course")
	}
	return s.dataRepo.ListByColumnID(ctx, columnID)
}

// SetCell writes (upserts) a single (column, user) cell.
func (s *CustomGradebookColumnService) SetCell(ctx context.Context, courseID, columnID, userID uint, content string) (*models.CustomColumnDatum, error) {
	col, err := s.colRepo.FindByID(ctx, columnID)
	if err != nil {
		return nil, errors.New("column not found")
	}
	if col.CourseID != courseID {
		return nil, errors.New("column does not belong to this course")
	}
	if col.ReadOnly {
		return nil, errors.New("column is read-only")
	}
	if len(content) > MaxContentBytes {
		return nil, errors.New("content exceeds 4KB limit")
	}
	d := &models.CustomColumnDatum{
		CustomGradebookColumnID: columnID,
		UserID:                  userID,
		Content:                 content,
	}
	if err := s.dataRepo.Upsert(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// BulkUpdateEntry is one cell in a bulk update payload (CSV-shaped:
// column_id × user_id → content).
type BulkUpdateEntry struct {
	ColumnID uint   `json:"column_id"`
	UserID   uint   `json:"user_id"`
	Content  string `json:"content"`
}

// BulkUpdate applies many cell updates. All column IDs must belong to the
// course and none may be read-only.
func (s *CustomGradebookColumnService) BulkUpdate(ctx context.Context, courseID uint, entries []BulkUpdateEntry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	cols, err := s.colRepo.ListByCourse(ctx, courseID, true)
	if err != nil {
		return 0, err
	}
	allowed := make(map[uint]models.CustomGradebookColumn, len(cols))
	for _, c := range cols {
		allowed[c.ID] = c
	}
	out := make([]models.CustomColumnDatum, 0, len(entries))
	for _, e := range entries {
		col, ok := allowed[e.ColumnID]
		if !ok {
			return 0, errors.New("column does not belong to this course")
		}
		if col.ReadOnly {
			return 0, errors.New("column is read-only")
		}
		if len(e.Content) > MaxContentBytes {
			return 0, errors.New("content exceeds 4KB limit")
		}
		out = append(out, models.CustomColumnDatum{
			CustomGradebookColumnID: e.ColumnID,
			UserID:                  e.UserID,
			Content:                 e.Content,
		})
	}
	if err := s.dataRepo.BulkUpsert(ctx, out); err != nil {
		return 0, err
	}
	return len(out), nil
}
