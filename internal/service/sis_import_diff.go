package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// errSISAborted marks a batch stopped by its change threshold.
var errSISAborted = errors.New("sis batch aborted by change threshold")

// sisDiffKeys names the columns that identify a row per import type
// (Canvas DiffGenerator). Types not listed are never diffed.
var sisDiffKeys = map[string][]string{
	"users":             {"user_id"},
	"terms":             {"term_id"},
	"courses":           {"course_id"},
	"sections":          {"section_id"},
	"enrollments":       {"course_id", "section_id", "user_id", "role", "associated_user_id"},
	"group_categories":  {"group_category_id"},
	"groups":            {"group_id"},
	"group_memberships": {"group_id", "user_id"},
	"user_observers":    {"observer_id", "student_id"},
	"logins":            {"login_id"},
	"admins":            {"user_id", "role"},
}

// sisDeletedStatus is the status a vanished row is re-imported with.
var sisDeletedStatus = map[string]string{
	"users":             "deleted",
	"terms":             "deleted",
	"courses":           "deleted",
	"sections":          "deleted",
	"enrollments":       "deleted",
	"group_categories":  "deleted",
	"groups":            "deleted",
	"group_memberships": "deleted",
	"user_observers":    "deleted",
	"admins":            "deleted",
}

func sisRowKey(importType string, row sisRow) string {
	cols := sisDiffKeys[importType]
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = strings.ToLower(row.get(c))
	}
	return strings.Join(parts, "\x1f")
}

// sisRowFingerprint is a column-order-independent rendering of a row.
func sisRowFingerprint(row sisRow) string {
	keys := make([]string, 0, len(row.values))
	for k := range row.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strings.TrimSpace(row.values[k]))
		b.WriteByte('\x1f')
	}
	return b.String()
}

// applyDiffing rewrites the parsed files to the changes since the previous
// batch with the same diffing identifier (Canvas diffing_data_set_identifier):
// unchanged rows are dropped and rows that disappeared are re-imported as
// deleted. Without a previous batch the full files are imported.
func (r *sisRun) applyDiffing(parsed []*sisParsedFile) error {
	var prev models.SISBatch
	err := r.db().Where("account_id = ? AND diffing_data_set_identifier = ? AND id < ? AND workflow_state IN ?",
		r.accountID, *r.batch.DiffingDataSetIdentifier, r.batch.ID, []string{"imported", "imported_with_messages"}).
		Order("id DESC").First(&prev).Error
	if err != nil {
		return nil // first batch of the data set: full import
	}
	var prevFiles []models.SISBatchFile
	if err := r.db().Where("sis_batch_id = ?", prev.ID).Order("id").Find(&prevFiles).Error; err != nil {
		return err
	}
	prevByType := map[string][]sisRow{}
	for _, f := range prevFiles {
		pf, err := parseSISFile(f)
		if err != nil {
			continue
		}
		prevByType[pf.importType] = append(prevByType[pf.importType], pf.rows...)
	}
	r.batch.DiffedAgainstBatchID = &prev.ID

	for _, pf := range parsed {
		if _, ok := sisDiffKeys[pf.importType]; !ok {
			continue
		}
		old := prevByType[pf.importType]
		if len(old) == 0 {
			continue
		}
		oldByKey := make(map[string]sisRow, len(old))
		for _, row := range old {
			oldByKey[sisRowKey(pf.importType, row)] = row
		}
		var kept []sisRow
		seen := map[string]bool{}
		for _, row := range pf.rows {
			key := sisRowKey(pf.importType, row)
			seen[key] = true
			if o, ok := oldByKey[key]; ok && sisRowFingerprint(o) == sisRowFingerprint(row) {
				continue // unchanged since the last batch
			}
			kept = append(kept, row)
		}
		deleted := 0
		if status, ok := sisDeletedStatus[pf.importType]; ok {
			for key, o := range oldByKey {
				if seen[key] {
					continue
				}
				vals := make(map[string]string, len(o.values)+1)
				for k, v := range o.values {
					vals[k] = v
				}
				vals["status"] = status
				kept = append(kept, sisRow{line: 0, values: vals})
				deleted++
			}
		}
		if t := r.batch.ChangeThreshold; t != nil && deleted > 0 && deleted*100 > *t*len(old) {
			r.fail(0, pf.name, fmt.Sprintf("diffing would delete %d of %d %s rows, more than the %d%% change threshold; the batch was not imported", deleted, len(old), pf.importType, *t))
			return errSISAborted
		}
		pf.rows = kept
	}
	return nil
}

// applyBatchMode deletes the SIS-managed objects of the batch-mode term
// that this batch did not mention (Canvas batch mode): courses, their
// sections, and — when the batch carried an enrollments file — the
// enrollments of SIS users in SIS courses of the term.
func (r *sisRun) applyBatchMode() error {
	termID := *r.batch.BatchModeTermID
	var courseIDs []uint
	if r.sawCourses {
		if err := r.db().Model(&models.Course{}).
			Where("account_id = ? AND enrollment_term_id = ? AND sis_course_id IS NOT NULL AND workflow_state <> ?", r.accountID, termID, models.CourseDeleted).
			Pluck("id", &courseIDs).Error; err != nil {
			return err
		}
	}
	var sections []models.CourseSection
	if r.sawSections {
		if err := r.db().Where("sis_section_id IS NOT NULL AND workflow_state <> 'deleted' AND course_id IN (SELECT id FROM courses WHERE account_id = ? AND enrollment_term_id = ?)", r.accountID, termID).
			Find(&sections).Error; err != nil {
			return err
		}
	}
	var enrollments []models.Enrollment
	if r.sawEnrollments {
		if err := r.db().Where(`workflow_state <> 'deleted'
			AND course_id IN (SELECT id FROM courses WHERE account_id = ? AND enrollment_term_id = ? AND sis_course_id IS NOT NULL)
			AND user_id IN (SELECT id FROM users WHERE account_id = ? AND sis_user_id IS NOT NULL)`, r.accountID, termID, r.accountID).
			Find(&enrollments).Error; err != nil {
			return err
		}
	}

	var dropCourses []uint
	for _, id := range courseIDs {
		if !r.seenCourses[id] {
			dropCourses = append(dropCourses, id)
		}
	}
	var dropSections []models.CourseSection
	for _, s := range sections {
		if !r.seenSections[s.ID] {
			dropSections = append(dropSections, s)
		}
	}
	var dropEnrollments []models.Enrollment
	for _, e := range enrollments {
		if !r.seenEnrollments[e.ID] {
			dropEnrollments = append(dropEnrollments, e)
		}
	}

	total := len(courseIDs) + len(sections) + len(enrollments)
	drops := len(dropCourses) + len(dropSections) + len(dropEnrollments)
	if t := r.batch.ChangeThreshold; t != nil && drops > 0 && drops*100 > *t*total {
		r.fail(0, "", fmt.Sprintf("batch mode would delete %d of %d objects in the term, more than the %d%% change threshold; nothing was deleted", drops, total, *t))
		return errSISAborted
	}

	if len(dropCourses) > 0 {
		if err := r.db().Model(&models.Course{}).Where("id IN ? AND account_id = ?", dropCourses, r.accountID).
			Update("workflow_state", models.CourseDeleted).Error; err != nil {
			return err
		}
	}
	for i := range dropSections {
		dropSections[i].WorkflowState = "deleted"
		if err := r.db().Save(&dropSections[i]).Error; err != nil {
			return err
		}
	}
	for i := range dropEnrollments {
		dropEnrollments[i].WorkflowState = models.EnrollmentDeleted
		// Save through the repo so Enrollment.AfterSave drops linked
		// observers' access too.
		if err := r.s.enrollmentRepo.Update(r.ctx, &dropEnrollments[i]); err != nil {
			return err
		}
	}
	r.counts["batch_mode_deleted"] = drops
	return nil
}
