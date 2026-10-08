package service

// ContentMigrationRunner is the worker for content_migrations (G067).
// A migration is created in the "queued" state; the runner claims it
// (queued → running, atomically, so two workers never run one row),
// runs it, and records the outcome:
//
//   - course_copy: CourseCopyService copies the selected content of the
//     source course, with link rewriting and date shifting;
//   - common_cartridge / canvas_cartridge with an uploaded package: the
//     IMSCC parser imports it, then the selection's excluded types are
//     pruned and dates are shifted.
//
// New migrations start right away on their own goroutine; the minute
// scheduler's sweep (ProcessPending) picks up any that were queued while
// the server restarted and fails runs whose worker died.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ContentMigrationRunner runs queued content migrations.
type ContentMigrationRunner struct {
	migrations repository.ContentMigrationRepository
	queue      repository.ContentMigrationQueue
	courses    repository.CourseRepository
	copier     *CourseCopyService
	parser     *IMSCCParser
	dates      ContentDateRepos
	dispatch   func(func())
}

// NewContentMigrationRunner wires the worker. parser may be nil (package
// migrations then fail with a clear message).
func NewContentMigrationRunner(
	migrations repository.ContentMigrationRepository,
	queue repository.ContentMigrationQueue,
	courses repository.CourseRepository,
	copier *CourseCopyService,
	parser *IMSCCParser,
	dates ContentDateRepos,
) *ContentMigrationRunner {
	return &ContentMigrationRunner{
		migrations: migrations,
		queue:      queue,
		courses:    courses,
		copier:     copier,
		parser:     parser,
		dates:      dates,
		dispatch:   func(f func()) { go f() },
	}
}

// SetDispatcher replaces how Enqueue starts a run (tests run inline).
func (r *ContentMigrationRunner) SetDispatcher(fn func(func())) {
	if fn != nil {
		r.dispatch = fn
	}
}

// Runnable reports whether the runner processes this migration type.
func (r *ContentMigrationRunner) Runnable(m *models.ContentMigration) bool {
	if m == nil {
		return false
	}
	switch m.MigrationType {
	case "course_copy":
		return m.SourceCourseID != nil && *m.SourceCourseID != 0
	case "common_cartridge", "canvas_cartridge":
		return m.Attachment != ""
	}
	return false
}

// Enqueue starts a queued migration in the background.
func (r *ContentMigrationRunner) Enqueue(id uint) {
	r.dispatch(func() {
		if err := r.Run(context.Background(), id); err != nil {
			slog.Warn("content migration failed", "content_migration_id", id, "err", err)
		}
	})
}

// Run claims and executes one queued migration. A migration another
// worker claimed (or that is not queued) is skipped without error.
func (r *ContentMigrationRunner) Run(ctx context.Context, id uint) error {
	ok, err := r.queue.ClaimMigration(ctx, id)
	if err != nil || !ok {
		return err
	}
	m, err := r.migrations.FindByID(ctx, id)
	if err != nil {
		return err
	}

	lastProgress := -1
	progress := func(p int) {
		if p-lastProgress < 5 && p != 100 {
			return
		}
		lastProgress = p
		m.Progress = p
		_ = r.migrations.Update(ctx, m)
	}

	results, runErr := r.execute(ctx, m, progress)
	now := time.Now()
	m.FinishedAt = &now
	if results != nil {
		m.Results = datatypes.NewJSONType(*results)
	}
	if runErr != nil {
		m.WorkflowState = models.ContentMigrationFailed
		m.ErrorMessage = runErr.Error()
	} else {
		m.WorkflowState = models.ContentMigrationCompleted
		m.Progress = 100
		m.ErrorMessage = ""
	}
	if err := r.migrations.Update(ctx, m); err != nil {
		return err
	}
	return runErr
}

func (r *ContentMigrationRunner) execute(ctx context.Context, m *models.ContentMigration, progress func(int)) (*models.MigrationResults, error) {
	settings := m.MigrationSettings.Data()
	switch m.MigrationType {
	case "course_copy":
		if r.copier == nil {
			return nil, errors.New("course copy is not configured")
		}
		if m.SourceCourseID == nil || *m.SourceCourseID == 0 {
			return nil, errors.New("source_course_id is required")
		}
		res, err := r.copier.Copy(ctx, *m.SourceCourseID, m.CourseID, m.UserID, settings, progress)
		out := &models.MigrationResults{}
		if res != nil {
			out.Counts = res.Counts
			out.Warnings = res.Warnings
			if res.Shifted > 0 {
				out.Counts["dates_shifted"] = res.Shifted
			}
		}
		return out, err
	case "common_cartridge", "canvas_cartridge":
		if r.parser == nil {
			return nil, errors.New("package import is not configured")
		}
		if m.Attachment == "" {
			return nil, errors.New("no package was uploaded")
		}
		progress(10)
		res, err := r.parser.ParsePackage(ctx, m.CourseID, m.UserID, m.Attachment)
		if err != nil {
			if res != nil {
				r.parser.CleanupFailedImport(context.WithoutCancel(ctx), m.CourseID, res.CreatedEntities)
			}
			return nil, err
		}
		progress(80)
		out := ImportResultSummary(res)
		out.Warnings = append(out.Warnings, r.PostProcessImport(ctx, m.CourseID, settings, res)...)
		return out, nil
	}
	return nil, fmt.Errorf("migration type %q is not processed by the worker", m.MigrationType)
}

// ImportResultSummary converts a package import result into migration
// results.
func ImportResultSummary(res *ImportResult) *models.MigrationResults {
	out := &models.MigrationResults{Counts: map[string]int{}}
	if res == nil {
		return out
	}
	out.Counts[CopyTypeModules] = res.ModulesCreated
	out.Counts[CopyTypePages] = res.PagesCreated
	out.Counts[CopyTypeAssignments] = res.AssignmentsCreated
	out.Counts[CopyTypeQuizzes] = res.QuizzesCreated
	out.Counts["quiz_questions"] = res.QuestionsCreated
	out.Counts[CopyTypeDiscussions] = res.DiscussionsCreated
	out.Counts["module_items"] = res.ModuleItemsCreated
	out.Counts[CopyTypeFiles] = res.FilesCreated
	out.Warnings = append(out.Warnings, res.Errors...)
	out.Warnings = append(out.Warnings, res.Warnings...)
	if len(out.Warnings) > 200 {
		out.Warnings = out.Warnings[:200]
	}
	return out
}

// importRefCopyType maps an imported entity onto the copy type it
// belongs to (for type-level selection of a package import).
func importRefCopyType(t string) string {
	switch t {
	case "Assignment":
		return CopyTypeAssignments
	case "Quiz", "QuizQuestion", "QuestionBank", "QuestionBankEntry":
		return CopyTypeQuizzes
	case "WikiPage":
		return CopyTypePages
	case "DiscussionTopic", "Announcement":
		return CopyTypeDiscussions
	case "ContextModule", "ContentTag":
		return CopyTypeModules
	case "Attachment":
		return CopyTypeFiles
	case "CalendarEvent":
		return CopyTypeEvents
	case "AssignmentGroup":
		return CopyTypeAssignmentGroups
	case "Rubric", "RubricAssociation":
		return CopyTypeRubrics
	}
	return ""
}

// PostProcessImport applies the selective-import and date-shift options
// to a finished package import: rows of excluded content types are
// removed again (with the module items that linked them), then the
// remaining dates are shifted. It returns non-fatal issues.
//
// Package imports select by content type only: the ids inside a package
// are not known before it is parsed.
func (r *ContentMigrationRunner) PostProcessImport(ctx context.Context, courseID uint, settings models.MigrationSettings, res *ImportResult) []string {
	if res == nil {
		return nil
	}
	var warnings []string
	if len(settings.IncludeOnlyTypes) > 0 || len(settings.ExcludeTypes) > 0 {
		sel := resolveCopySelection(models.MigrationSettings{
			IncludeOnlyTypes: settings.IncludeOnlyTypes,
			ExcludeTypes:     settings.ExcludeTypes,
		})
		removed := map[string]bool{}
		var drop, keep []EntityRef
		for _, ref := range res.CreatedEntities {
			t := importRefCopyType(ref.Type)
			if t != "" && !sel.all[t] {
				drop = append(drop, ref)
				removed[contentKey(ref.Type, ref.ID)] = true
			} else {
				keep = append(keep, ref)
			}
		}
		// Module items that link removed content go too.
		if r.parser != nil && r.parser.moduleItemRepo != nil {
			kept := keep[:0]
			for _, ref := range keep {
				if ref.Type == "ContentTag" {
					if it, err := r.parser.moduleItemRepo.FindByID(ctx, ref.ID); err == nil && it != nil && it.ContentID != nil &&
						removed[contentKey(it.ContentType, *it.ContentID)] {
						drop = append(drop, ref)
						continue
					}
				}
				kept = append(kept, ref)
			}
			keep = kept
		}
		if len(drop) > 0 && r.parser != nil {
			r.parser.CleanupFailedImport(ctx, courseID, drop)
		}
		res.CreatedEntities = keep
	}

	if opts := settings.DateShiftOptions; opts != nil {
		var target *models.Course
		if r.courses != nil {
			// accountID=0: auth-internal — the import's own target course.
			target, _ = r.courses.FindByID(ctx, courseID, 0)
		}
		shifter := NewDateShifter(opts, nil, target)
		if shifter == nil && opts.ShiftDates {
			warnings = append(warnings, "dates were not shifted: give the old and new start dates")
		}
		if _, err := ShiftEntityDates(ctx, r.dates, shifter, res.CreatedEntities); err != nil {
			warnings = append(warnings, fmt.Sprintf("date shift failed: %v", err))
		}
	}
	return warnings
}

// ProcessPending is the scheduler sweep for migrations: fail runs whose
// worker died, then run migrations still queued after a minute.
func (r *ContentMigrationRunner) ProcessPending(ctx context.Context) error {
	if _, err := r.queue.FailStaleMigrations(ctx, time.Now().Add(-staleJobAfter)); err != nil {
		return err
	}
	ids, err := r.queue.ListQueuedMigrationIDs(ctx, time.Now().Add(-time.Minute), 10)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := r.Run(ctx, id); err != nil {
			slog.Warn("content migration failed", "content_migration_id", id, "err", err)
		}
	}
	return nil
}
