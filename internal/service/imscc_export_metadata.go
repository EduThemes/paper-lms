package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// This file holds the symmetric writers for every metadata file
// imscc_course_metadata.go (and parts of imscc_parser.go) read on import.
// Together with the existing per-resource exporters in imscc_exporter.go
// they make the cartridge round-trippable.
//
// All writers are best-effort: a missing repo or an empty result simply
// skips that file rather than failing the whole export.

// --- course_settings.xml ---

// exportCourseSettings serializes Course → course_settings.xml. Mirrors the
// fields imscc_course_metadata.go reads.
type exportCourseSettings struct {
	XMLName     xml.Name `xml:"course"`
	Identifier  string   `xml:"identifier,attr"`
	XMLNS       string   `xml:"xmlns,attr"`
	Title       string   `xml:"title"`
	CourseCode  string   `xml:"course_code"`
	StartAt     string   `xml:"start_at"`
	ConcludeAt  string   `xml:"conclude_at"`
	IsPublic    string   `xml:"is_public"`
	License     string   `xml:"license"`
	DefaultView string   `xml:"default_view"`
	TabConfig   string   `xml:"tab_configuration,omitempty"`
}

func (e *IMSCCExporter) writeCourseSettingsXML(w *zip.Writer, course *models.Course) error {
	doc := exportCourseSettings{
		Identifier:  fmt.Sprintf("course_%d", course.ID),
		XMLNS:       "http://canvas.instructure.com/xsd/cccv1p0",
		Title:       course.Name,
		CourseCode:  course.CourseCode,
		IsPublic:    boolToString(course.IsPublic),
		License:     valueOr(course.License, "private"),
		DefaultView: valueOr(course.DefaultView, "modules"),
		TabConfig:   course.NavigationTabs,
	}
	if course.StartAt != nil {
		doc.StartAt = course.StartAt.UTC().Format(time.RFC3339)
	}
	if course.EndAt != nil {
		doc.ConcludeAt = course.EndAt.UTC().Format(time.RFC3339)
	}
	return writeXMLFile(w, "course_settings/course_settings.xml", doc)
}

// writeSyllabusHTML emits the course's syllabus body verbatim as the
// "course_settings/syllabus.html" entry. Skipped when the body is empty.
func (e *IMSCCExporter) writeSyllabusHTML(w *zip.Writer, course *models.Course) error {
	if strings.TrimSpace(course.SyllabusBody) == "" {
		return nil
	}
	full := "<!DOCTYPE html><html><head><meta charset=\"UTF-8\"><title>" +
		xmlEscape(course.Name) + " — Syllabus</title></head><body>" +
		course.SyllabusBody + "</body></html>"
	return writeRawFile(w, "course_settings/syllabus.html", []byte(full))
}

// writeCanvasExportTxt writes the static marker file Canvas treats as a
// "this is a Canvas export" sentinel. Constant content; importers ignore it.
func (e *IMSCCExporter) writeCanvasExportTxt(w *zip.Writer) error {
	return writeRawFile(w, "course_settings/canvas_export.txt",
		[]byte("Q: What did the panda say when he was forced out of his natural habitat?\nA: This is un-BEAR-able\n"))
}

// --- module_meta.xml ---

type exportModuleMeta struct {
	XMLName xml.Name              `xml:"modules"`
	XMLNS   string                `xml:"xmlns,attr"`
	Modules []exportModuleMetaMod `xml:"module"`
}

type exportModuleMetaMod struct {
	Identifier                string                 `xml:"identifier,attr"`
	Title                     string                 `xml:"title"`
	WorkflowState             string                 `xml:"workflow_state"`
	Position                  int                    `xml:"position"`
	RequireSequentialProgress string                 `xml:"require_sequential_progress"`
	Items                     []exportModuleMetaItem `xml:"items>item"`
}

type exportModuleMetaItem struct {
	Identifier    string `xml:"identifier,attr"`
	IdentifierRef string `xml:"identifierref,omitempty"`
	ContentType   string `xml:"content_type"`
	Title         string `xml:"title"`
	WorkflowState string `xml:"workflow_state"`
	Position      int    `xml:"position"`
	Indent        int    `xml:"indent"`
	NewTab        string `xml:"new_tab,omitempty"`
	URL           string `xml:"url,omitempty"`
}

func (e *IMSCCExporter) writeModuleMetaXML(
	ctx context.Context,
	w *zip.Writer,
	courseID uint,
	result *ExportResult,
) error {
	if e.moduleRepo == nil {
		return nil
	}
	page := repository.PaginationParams{Page: 1, PerPage: 10000}
	mods, err := e.moduleRepo.ListByCourseID(ctx, courseID, page)
	if err != nil || len(mods.Items) == 0 {
		return nil
	}
	doc := exportModuleMeta{
		XMLNS: "http://canvas.instructure.com/xsd/cccv1p0",
	}
	for _, m := range mods.Items {
		mm := exportModuleMetaMod{
			Identifier:                fmt.Sprintf("module_%d", m.ID),
			Title:                     m.Name,
			WorkflowState:             valueOr(m.WorkflowState, "active"),
			Position:                  m.Position,
			RequireSequentialProgress: boolToString(m.RequireSequentialProgress),
		}
		items, ierr := e.moduleItemRepo.ListByModuleID(ctx, m.ID, page)
		if ierr == nil {
			for _, it := range items.Items {
				mi := exportModuleMetaItem{
					Identifier:    fmt.Sprintf("item_%d", it.ID),
					ContentType:   exportModuleItemContentType(it.ContentType),
					Title:         it.Title,
					WorkflowState: valueOr(string(it.WorkflowState), "active"),
					Position:      it.Position,
					Indent:        it.Indent,
					URL:           it.URL,
				}
				if it.NewTab {
					mi.NewTab = "true"
				}
				if it.ContentID != nil {
					mi.IdentifierRef = exportResourceID(it.ContentType, it.ContentID)
				}
				mm.Items = append(mm.Items, mi)
			}
		}
		doc.Modules = append(doc.Modules, mm)
	}
	if err := writeXMLFile(w, "course_settings/module_meta.xml", doc); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("module_meta.xml: %v", err))
		return err
	}
	return nil
}

// exportModuleItemContentType maps internal content_type strings to the
// Canvas-namespaced labels module_meta.xml uses (Quizzes::Quiz, etc.).
func exportModuleItemContentType(t string) string {
	switch t {
	case "Quiz":
		return "Quizzes::Quiz"
	case "DiscussionTopic":
		return "DiscussionTopic"
	}
	return t
}

// --- assignment_groups.xml ---

func (e *IMSCCExporter) writeAssignmentGroupsXML(
	ctx context.Context,
	w *zip.Writer,
	courseID uint,
	result *ExportResult,
) error {
	if e.assignmentGroupRepo == nil {
		return nil
	}
	page := repository.PaginationParams{Page: 1, PerPage: 1000}
	groups, err := e.assignmentGroupRepo.ListByCourseID(ctx, courseID, page)
	if err != nil || len(groups.Items) == 0 {
		return nil
	}
	type expGroup struct {
		Identifier  string `xml:"identifier,attr"`
		Title       string `xml:"title"`
		Position    int    `xml:"position"`
		GroupWeight string `xml:"group_weight"`
		Rules       string `xml:"rules,omitempty"`
	}
	doc := struct {
		XMLName xml.Name   `xml:"assignmentGroups"`
		XMLNS   string     `xml:"xmlns,attr"`
		Groups  []expGroup `xml:"assignmentGroup"`
	}{
		XMLNS: "http://canvas.instructure.com/xsd/cccv1p0",
	}
	for _, g := range groups.Items {
		doc.Groups = append(doc.Groups, expGroup{
			Identifier:  fmt.Sprintf("group_%d", g.ID),
			Title:       g.Name,
			Position:    g.Position,
			GroupWeight: strconv.FormatFloat(g.GroupWeight, 'f', -1, 64),
			Rules:       g.Rules,
		})
	}
	if err := writeXMLFile(w, "course_settings/assignment_groups.xml", doc); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("assignment_groups.xml: %v", err))
		return err
	}
	return nil
}

// --- rubrics.xml ---

func (e *IMSCCExporter) writeRubricsXML(
	ctx context.Context,
	w *zip.Writer,
	courseID uint,
	result *ExportResult,
) error {
	if e.rubricRepo == nil {
		return nil
	}
	page := repository.PaginationParams{Page: 1, PerPage: 1000}
	rubrics, err := e.rubricRepo.ListByContext(ctx, "Course", courseID, 0, page)
	if err != nil || len(rubrics.Items) == 0 {
		return nil
	}
	type expRating struct {
		ID          string `xml:"id,attr,omitempty"`
		Description string `xml:"description"`
		Points      string `xml:"points"`
	}
	type expCriterion struct {
		ID              string      `xml:"id,attr,omitempty"`
		Description     string      `xml:"description"`
		LongDescription string      `xml:"long_description,omitempty"`
		Points          string      `xml:"points"`
		Ratings         []expRating `xml:"ratings>rating"`
	}
	type expAssoc struct {
		AssociationType string `xml:"association_type"`
		AssociationRef  string `xml:"association_identifierref"`
		UseForGrading   string `xml:"use_for_grading"`
	}
	type expRubric struct {
		Identifier     string         `xml:"identifier,attr"`
		Title          string         `xml:"title"`
		Description    string         `xml:"description,omitempty"`
		PointsPossible string         `xml:"points_possible"`
		Criteria       []expCriterion `xml:"criteria>criterion"`
		Associations   []expAssoc     `xml:"associations>association,omitempty"`
	}
	doc := struct {
		XMLName xml.Name    `xml:"rubrics"`
		XMLNS   string      `xml:"xmlns,attr"`
		Rubrics []expRubric `xml:"rubric"`
	}{
		XMLNS: "http://canvas.instructure.com/xsd/cccv1p0",
	}

	for _, r := range rubrics.Items {
		// Decode the criteria JSON we stored on import.
		var crits []struct {
			ID              string  `json:"id"`
			Description     string  `json:"description"`
			LongDescription string  `json:"long_description,omitempty"`
			Points          float64 `json:"points"`
			Ratings         []struct {
				ID          string  `json:"id"`
				Description string  `json:"description"`
				Points      float64 `json:"points"`
			} `json:"ratings"`
		}
		_ = json.Unmarshal([]byte(r.Data), &crits)

		ec := make([]expCriterion, 0, len(crits))
		for _, c := range crits {
			ratings := make([]expRating, 0, len(c.Ratings))
			for _, rr := range c.Ratings {
				ratings = append(ratings, expRating{
					ID:          rr.ID,
					Description: rr.Description,
					Points:      strconv.FormatFloat(rr.Points, 'f', -1, 64),
				})
			}
			ec = append(ec, expCriterion{
				ID:              c.ID,
				Description:     c.Description,
				LongDescription: c.LongDescription,
				Points:          strconv.FormatFloat(c.Points, 'f', -1, 64),
				Ratings:         ratings,
			})
		}

		// Pull associations (Assignment) the import wrote.
		var assocs []expAssoc
		if e.rubricAssocRepo != nil {
			// We don't have a list-by-rubric API, so peek by association
			// (best-effort). Skipped in the round-trip when not available.
		}

		doc.Rubrics = append(doc.Rubrics, expRubric{
			Identifier:     fmt.Sprintf("rubric_%d", r.ID),
			Title:          r.Title,
			Description:    r.Description,
			PointsPossible: strconv.FormatFloat(r.PointsPossible, 'f', -1, 64),
			Criteria:       ec,
			Associations:   assocs,
		})
	}

	if err := writeXMLFile(w, "course_settings/rubrics.xml", doc); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("rubrics.xml: %v", err))
		return err
	}
	return nil
}

// --- learning_outcomes.xml ---

func (e *IMSCCExporter) writeLearningOutcomesXML(
	ctx context.Context,
	w *zip.Writer,
	courseID uint,
	result *ExportResult,
) error {
	if e.outcomeGroupRepo == nil || e.outcomeRepo == nil {
		return nil
	}
	page := repository.PaginationParams{Page: 1, PerPage: 1000}
	groups, err := e.outcomeGroupRepo.ListByContext(ctx, "Course", courseID, 0, page)
	if err != nil || len(groups.Items) == 0 {
		return nil
	}
	outcomes, err := e.outcomeRepo.ListByContext(ctx, "Course", courseID, 0, page)
	if err != nil {
		return nil
	}

	type expOutcome struct {
		Identifier        string `xml:"identifier,attr"`
		Title             string `xml:"title"`
		DisplayName       string `xml:"display_name,omitempty"`
		Description       string `xml:"description,omitempty"`
		CalculationMethod string `xml:"calculation_method,omitempty"`
		CalculationInt    string `xml:"calculation_int,omitempty"`
		MasteryPoints     string `xml:"mastery_points,omitempty"`
		PointsPossible    string `xml:"points_possible,omitempty"`
	}
	type expGroup struct {
		Identifier  string       `xml:"identifier,attr"`
		Title       string       `xml:"title"`
		Description string       `xml:"description,omitempty"`
		Groups      []expGroup   `xml:"learningOutcomeGroup,omitempty"`
		Outcomes    []expOutcome `xml:"learningOutcome,omitempty"`
	}

	// Index outcomes by their group ID and groups by parent ID.
	outcomesByGroup := map[uint][]expOutcome{}
	for _, o := range outcomes.Items {
		outcomesByGroup[o.OutcomeGroupID] = append(outcomesByGroup[o.OutcomeGroupID], expOutcome{
			Identifier:        fmt.Sprintf("outcome_%d", o.ID),
			Title:             o.Title,
			DisplayName:       o.DisplayName,
			Description:       o.Description,
			CalculationMethod: o.CalculationMethod,
			CalculationInt:    strconv.Itoa(o.CalculationInt),
			MasteryPoints:     strconv.FormatFloat(o.MasteryPoints, 'f', -1, 64),
			PointsPossible:    strconv.FormatFloat(o.PointsPossible, 'f', -1, 64),
		})
	}
	groupsByParent := map[uint][]models.LearningOutcomeGroup{}
	for _, g := range groups.Items {
		parentID := uint(0)
		if g.ParentGroupID != nil {
			parentID = *g.ParentGroupID
		}
		groupsByParent[parentID] = append(groupsByParent[parentID], g)
	}

	var build func(g models.LearningOutcomeGroup) expGroup
	build = func(g models.LearningOutcomeGroup) expGroup {
		eg := expGroup{
			Identifier:  fmt.Sprintf("group_%d", g.ID),
			Title:       g.Title,
			Description: g.Description,
			Outcomes:    outcomesByGroup[g.ID],
		}
		for _, sub := range groupsByParent[g.ID] {
			eg.Groups = append(eg.Groups, build(sub))
		}
		return eg
	}

	doc := struct {
		XMLName  xml.Name     `xml:"learningOutcomes"`
		XMLNS    string       `xml:"xmlns,attr"`
		Groups   []expGroup   `xml:"learningOutcomeGroup,omitempty"`
		Outcomes []expOutcome `xml:"learningOutcome,omitempty"`
	}{
		XMLNS: "http://canvas.instructure.com/xsd/cccv1p0",
	}

	// Emit top-level groups (parent_id == 0 / nil) and any outcomes that
	// belong to those top-level groups recursively.
	for _, g := range groupsByParent[0] {
		doc.Groups = append(doc.Groups, build(g))
	}
	// Outcomes attached directly to context (no group) — rare but legal.
	doc.Outcomes = outcomesByGroup[0]

	if err := writeXMLFile(w, "course_settings/learning_outcomes.xml", doc); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("learning_outcomes.xml: %v", err))
		return err
	}
	return nil
}

// --- events.xml ---

func (e *IMSCCExporter) writeEventsXML(
	ctx context.Context,
	w *zip.Writer,
	courseID uint,
	result *ExportResult,
) error {
	if e.calendarEventRepo == nil {
		return nil
	}
	page := repository.PaginationParams{Page: 1, PerPage: 1000}
	events, err := e.calendarEventRepo.ListByContext(ctx, "Course", courseID, page)
	if err != nil || len(events.Items) == 0 {
		return nil
	}
	type expEvent struct {
		Identifier      string `xml:"identifier,attr"`
		Title           string `xml:"title"`
		Description     string `xml:"description,omitempty"`
		StartAt         string `xml:"start_at"`
		EndAt           string `xml:"end_at,omitempty"`
		LocationName    string `xml:"location_name,omitempty"`
		LocationAddress string `xml:"location_address,omitempty"`
		AllDay          string `xml:"all_day,omitempty"`
	}
	doc := struct {
		XMLName xml.Name   `xml:"events"`
		XMLNS   string     `xml:"xmlns,attr"`
		Events  []expEvent `xml:"event"`
	}{
		XMLNS: "http://canvas.instructure.com/xsd/cccv1p0",
	}
	for _, ev := range events.Items {
		exp := expEvent{
			Identifier:      fmt.Sprintf("event_%d", ev.ID),
			Title:           ev.Title,
			Description:     ev.Description,
			StartAt:         ev.StartAt.UTC().Format(time.RFC3339),
			LocationName:    ev.LocationName,
			LocationAddress: ev.LocationAddress,
		}
		if ev.EndAt != nil {
			exp.EndAt = ev.EndAt.UTC().Format(time.RFC3339)
		}
		if ev.AllDay {
			exp.AllDay = "true"
		}
		doc.Events = append(doc.Events, exp)
	}
	if err := writeXMLFile(w, "course_settings/events.xml", doc); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("events.xml: %v", err))
		return err
	}
	return nil
}

// --- files_meta.xml + web_resources/* ---

// writeAttachments writes each course attachment to web_resources/<filename>
// (matching the Canvas exporter convention) and emits a files_meta.xml index
// alongside. Returns a path→exported-filename map so the token emitter (in
// 2B) can rewrite /api/v1/files/N/download URLs back to $IMS-CC-FILEBASE$
// references.
func (e *IMSCCExporter) writeAttachments(
	ctx context.Context,
	w *zip.Writer,
	courseID uint,
	result *ExportResult,
) (map[uint]string, error) {
	if e.attachmentRepo == nil || e.fileService == nil {
		return nil, nil
	}
	page := repository.PaginationParams{Page: 1, PerPage: 5000}
	atts, err := e.attachmentRepo.ListByContext(ctx, "Course", courseID, page)
	if err != nil || len(atts.Items) == 0 {
		return nil, nil
	}

	urlByID := make(map[uint]string, len(atts.Items))

	type expFile struct {
		Identifier  string `xml:"identifier,attr"`
		DisplayName string `xml:"display_name"`
		Filename    string `xml:"filename"`
		ContentType string `xml:"content_type"`
		Size        string `xml:"size"`
		Hidden      string `xml:"hidden,omitempty"`
		Locked      string `xml:"locked,omitempty"`
	}
	doc := struct {
		XMLName xml.Name  `xml:"fileMeta"`
		XMLNS   string    `xml:"xmlns,attr"`
		Files   []expFile `xml:"file"`
	}{
		XMLNS: "http://canvas.instructure.com/xsd/cccv1p0",
	}

	for _, a := range atts.Items {
		zipPath := path.Join("web_resources", sanitizeAttachmentName(a.DisplayName))
		// Read file contents from the storage backend.
		// FOLLOW-UP (Wave 10): stamp settingsctx.WithAccountID(ctx, accountID)
		// here so per-tenant S3 buckets are honored on export. Requires
		// threading accountID through IMSCCExporter.ExportCourse + ContentExport
		// handler (which currently doesn't pass callerAccountID).
		rc, getErr := e.fileService.StorageBackend().Get(ctx, a.StoragePath)
		if getErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("could not read attachment %d (%q): %v", a.ID, a.DisplayName, getErr))
			continue
		}
		entry, createErr := w.Create(zipPath)
		if createErr != nil {
			rc.Close()
			result.Errors = append(result.Errors, fmt.Sprintf("could not create zip entry %q: %v", zipPath, createErr))
			continue
		}
		if _, copyErr := io.Copy(entry, rc); copyErr != nil {
			rc.Close()
			result.Errors = append(result.Errors, fmt.Sprintf("could not copy attachment %d into zip: %v", a.ID, copyErr))
			continue
		}
		_ = rc.Close()
		urlByID[a.ID] = zipPath
		doc.Files = append(doc.Files, expFile{
			Identifier:  fmt.Sprintf("attach_%d", a.ID),
			DisplayName: a.DisplayName,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        strconv.FormatInt(a.Size, 10),
		})
	}

	if len(doc.Files) > 0 {
		if err := writeXMLFile(w, "course_settings/files_meta.xml", doc); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("files_meta.xml: %v", err))
		}
	}
	return urlByID, nil
}

// --- per-quiz assessment_meta.xml ---

// quizAssessmentMetaXML serializes the assessment_meta.xml that lives next
// to a quiz's QTI body. Mirrors the canvasQuizMeta struct on the import
// side so applyQuizMeta can read its own output.
func quizAssessmentMetaXML(q models.Quiz) ([]byte, error) {
	type expQuizMeta struct {
		XMLName            xml.Name `xml:"quiz"`
		Identifier         string   `xml:"identifier,attr"`
		XMLNS              string   `xml:"xmlns,attr"`
		Title              string   `xml:"title"`
		Description        string   `xml:"description"`
		QuizType           string   `xml:"quiz_type"`
		PointsPossible     string   `xml:"points_possible"`
		TimeLimit          string   `xml:"time_limit"`
		AllowedAttempts    string   `xml:"allowed_attempts"`
		ShuffleAnswers     string   `xml:"shuffle_answers"`
		ScoringPolicy      string   `xml:"scoring_policy"`
		HideResults        string   `xml:"hide_results"`
		ShowCorrectAnswers string   `xml:"show_correct_answers"`
		OneQuestionAtATime string   `xml:"one_question_at_a_time"`
		CantGoBack         string   `xml:"cant_go_back"`
		Available          string   `xml:"available"`
		LockAt             string   `xml:"lock_at"`
		UnlockAt           string   `xml:"unlock_at"`
		DueAt              string   `xml:"due_at"`
	}
	doc := expQuizMeta{
		Identifier:         fmt.Sprintf("quiz_%d", q.ID),
		XMLNS:              "http://canvas.instructure.com/xsd/cccv1p0",
		Title:              q.Title,
		Description:        q.Description,
		QuizType:           valueOr(q.QuizType, "assignment"),
		AllowedAttempts:    strconv.Itoa(q.AllowedAttempts),
		ShuffleAnswers:     boolToString(q.ShuffleAnswers),
		ScoringPolicy:      valueOr(q.ScoringPolicy, "keep_highest"),
		HideResults:        q.HideResults,
		ShowCorrectAnswers: boolToString(q.ShowCorrectAnswers),
		OneQuestionAtATime: boolToString(q.OneQuestionAtATime),
		CantGoBack:         boolToString(q.CantGoBack),
		Available:          boolToString(q.Published),
	}
	if q.PointsPossible != nil {
		doc.PointsPossible = strconv.FormatFloat(*q.PointsPossible, 'f', -1, 64)
	}
	if q.TimeLimit != nil {
		doc.TimeLimit = strconv.Itoa(*q.TimeLimit)
	}
	if q.LockAt != nil {
		doc.LockAt = q.LockAt.UTC().Format(time.RFC3339)
	}
	if q.UnlockAt != nil {
		doc.UnlockAt = q.UnlockAt.UTC().Format(time.RFC3339)
	}
	if q.DueAt != nil {
		doc.DueAt = q.DueAt.UTC().Format(time.RFC3339)
	}
	return xml.MarshalIndent(doc, "", "  ")
}

// --- helpers ---

// writeXMLFile marshals doc with an XML declaration and writes it to the
// given zip path.
func writeXMLFile(w *zip.Writer, name string, doc interface{}) error {
	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	return writeRawFile(w, name, append([]byte(xml.Header), body...))
}

func writeRawFile(w *zip.Writer, name string, body []byte) error {
	entry, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = entry.Write(body)
	return err
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func valueOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// buildWikiHTMLDocument wraps a page body in a minimal HTML document with
// the page title in <title>. This matches what Canvas writes (so the
// importer's extractDocumentTitle / extractBodyHTML can both fire on the
// round-tripped output).
func buildWikiHTMLDocument(title, body string) string {
	return "<!DOCTYPE html><html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=utf-8\"><title>" +
		xmlEscape(title) +
		"</title><meta name=\"identifier\" content=\"\"><meta name=\"editing_roles\" content=\"teachers\"><meta name=\"workflow_state\" content=\"active\"></head><body>" +
		body + "</body></html>"
}

// sanitizeAttachmentName trims path-traversing characters from a display
// name so it's safe to use as a zip path. Spaces are kept (Canvas does too).
func sanitizeAttachmentName(name string) string {
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimLeft(name, "/")
	if name == "" {
		return "file"
	}
	return name
}
