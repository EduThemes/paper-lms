package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// This file holds parsers for the course-level metadata files Canvas writes
// into a Common Cartridge alongside the per-resource content:
//
//   course_settings/course_settings.xml      → course.* (timezone, license, …)
//   course_settings/syllabus.html            → course.SyllabusBody
//   course_settings/rubrics.xml              → Rubric + RubricAssociation
//   course_settings/learning_outcomes.xml    → LearningOutcomeGroup + Outcome
//   course_settings/events.xml               → CalendarEvent
//
// All parsers are best-effort: a missing file is normal for trimmed
// exports, a parse failure surfaces as a Warning (not a hard error) so the
// rest of the import keeps going.

// --- course_settings.xml + syllabus.html ---

type ccCourseSettings struct {
	XMLName        xml.Name `xml:"course"`
	Title          string   `xml:"title"`
	CourseCode     string   `xml:"course_code"`
	IsPublic       string   `xml:"is_public"`
	License        string   `xml:"license"`
	DefaultView    string   `xml:"default_view"`
	StartAt        string   `xml:"start_at"`
	ConcludeAt     string   `xml:"conclude_at"`
	TabConfig      string   `xml:"tab_configuration"`
}

func (p *IMSCCParser) parseCourseSettingsXML(
	ctx context.Context,
	courseID uint,
	zipFiles map[string]*zip.File,
	result *ImportResult,
) {
	if p.courseRepo == nil {
		return
	}
	course, err := p.courseRepo.FindByID(ctx, courseID)
	if err != nil || course == nil {
		return
	}

	dirty := false
	if data, err := readZipFile(zipFiles, "course_settings/course_settings.xml"); err == nil && len(data) > 0 {
		var cs ccCourseSettings
		if xerr := xml.Unmarshal(data, &cs); xerr == nil {
			if t := strings.TrimSpace(cs.Title); t != "" && course.Name == "" {
				// Only adopt the cartridge's name when the destination course
				// hasn't been named yet — admins typically want their own
				// label to win, but a freshly-created shell course should
				// inherit the source title.
				course.Name = t
				dirty = true
			}
			if c := strings.TrimSpace(cs.CourseCode); c != "" && course.CourseCode == "" {
				course.CourseCode = c
				dirty = true
			}
			if l := strings.TrimSpace(cs.License); l != "" {
				course.License = l
				dirty = true
			}
			if dv := strings.TrimSpace(cs.DefaultView); dv != "" {
				course.DefaultView = dv
				dirty = true
			}
			if cs.IsPublic != "" {
				course.IsPublic = parseBoolFlag(cs.IsPublic)
				dirty = true
			}
			if t := parseCanvasTime(cs.StartAt); t != nil {
				course.StartAt = t
				dirty = true
			}
			if t := parseCanvasTime(cs.ConcludeAt); t != nil {
				course.EndAt = t
				dirty = true
			}
			if tc := strings.TrimSpace(cs.TabConfig); tc != "" && json.Valid([]byte(tc)) {
				course.NavigationTabs = tc
				dirty = true
			}
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse course_settings.xml: %v", xerr))
		}
	}

	if data, err := readZipFile(zipFiles, "course_settings/syllabus.html"); err == nil && len(data) > 0 {
		// Canvas writes the syllabus as a full HTML document; pull the body
		// the same way wiki pages are extracted so wrapping <html> tags
		// don't end up rendering as text.
		body := extractBodyHTML(string(data))
		if strings.TrimSpace(body) != "" {
			course.SyllabusBody = body
			dirty = true
		}
	}

	if dirty {
		if err := p.courseRepo.Update(ctx, course); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to apply course_settings.xml: %v", err))
		}
	}
}

// --- rubrics.xml ---

type ccRubrics struct {
	XMLName xml.Name   `xml:"rubrics"`
	Rubrics []ccRubric `xml:"rubric"`
}

type ccRubric struct {
	Identifier   string             `xml:"identifier,attr"`
	Title        string             `xml:"title"`
	Description  string             `xml:"description"`
	PointsPossible string           `xml:"points_possible"`
	Criteria     []ccRubricCriterion `xml:"criteria>criterion"`
	Associations []ccRubricAssoc    `xml:"associations>association"`
}

type ccRubricCriterion struct {
	ID              string          `xml:"id,attr"`
	Description     string          `xml:"description"`
	LongDescription string          `xml:"long_description"`
	Points          string          `xml:"points"`
	Ratings         []ccRubricRating `xml:"ratings>rating"`
}

type ccRubricRating struct {
	ID          string `xml:"id,attr"`
	Description string `xml:"description"`
	Points      string `xml:"points"`
}

type ccRubricAssoc struct {
	AssociationType string `xml:"association_type"`
	AssociationRef  string `xml:"association_identifierref"`
	UseForGrading   string `xml:"use_for_grading"`
	Purpose         string `xml:"purpose"`
}

func (p *IMSCCParser) parseRubricsXML(
	ctx context.Context,
	courseID uint,
	zipFiles map[string]*zip.File,
	result *ImportResult,
) {
	if p.rubricRepo == nil {
		return
	}
	data, err := readZipFile(zipFiles, "course_settings/rubrics.xml")
	if err != nil || len(data) == 0 {
		return
	}
	var doc ccRubrics
	if err := xml.Unmarshal(data, &doc); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse rubrics.xml: %v", err))
		return
	}

	for _, r := range doc.Rubrics {
		points := 0.0
		if r.PointsPossible != "" {
			if v, err := strconv.ParseFloat(r.PointsPossible, 64); err == nil {
				points = v
			}
		}
		// Encode criteria as Canvas's rubric Data JSON shape so the
		// existing /api/v1/rubrics endpoints render imported rubrics
		// without further translation.
		type ratingJSON struct {
			ID          string  `json:"id"`
			Description string  `json:"description"`
			Points      float64 `json:"points"`
		}
		type critJSON struct {
			ID              string       `json:"id"`
			Description     string       `json:"description"`
			LongDescription string       `json:"long_description,omitempty"`
			Points          float64      `json:"points"`
			Ratings         []ratingJSON `json:"ratings"`
		}
		crits := make([]critJSON, 0, len(r.Criteria))
		for _, c := range r.Criteria {
			cp := 0.0
			if c.Points != "" {
				if v, err := strconv.ParseFloat(c.Points, 64); err == nil {
					cp = v
				}
			}
			rat := make([]ratingJSON, 0, len(c.Ratings))
			for _, rr := range c.Ratings {
				rp := 0.0
				if rr.Points != "" {
					if v, err := strconv.ParseFloat(rr.Points, 64); err == nil {
						rp = v
					}
				}
				rat = append(rat, ratingJSON{ID: rr.ID, Description: rr.Description, Points: rp})
			}
			crits = append(crits, critJSON{ID: c.ID, Description: c.Description, LongDescription: c.LongDescription, Points: cp, Ratings: rat})
		}
		dataJSON, _ := json.Marshal(crits)
		rubric := &models.Rubric{
			ContextType:    "Course",
			ContextID:      courseID,
			Title:          strings.TrimSpace(r.Title),
			Description:    r.Description,
			Data:           string(dataJSON),
			PointsPossible: points,
			WorkflowState:  "active",
		}
		if rubric.Title == "" {
			rubric.Title = "Imported Rubric"
		}
		if err := p.rubricRepo.Create(ctx, rubric); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to create rubric %q: %v", rubric.Title, err))
			continue
		}
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "Rubric", ID: rubric.ID})
		if r.Identifier != "" {
			result.entityByMigID[r.Identifier] = EntityRef{Type: "Rubric", ID: rubric.ID}
		}

		// Wire associations to assignments by migration_id. The assignment
		// might not exist yet (rubrics.xml runs in pre-1, assignments in
		// pass-1); defer this until we have entityByMigID populated.
		for _, a := range r.Associations {
			if !strings.EqualFold(a.AssociationType, "assignment") || a.AssociationRef == "" {
				continue
			}
			result.pendingRubricAssocs = append(result.pendingRubricAssocs, pendingRubricAssoc{
				rubricID:        rubric.ID,
				assignmentMigID: a.AssociationRef,
				useForGrading:   parseBoolFlag(a.UseForGrading),
				purpose:         strings.TrimSpace(a.Purpose),
				courseID:        courseID,
			})
		}
	}
}

// pendingRubricAssoc holds an association that needs to be wired after
// pass-1 has created the target Assignment row.
type pendingRubricAssoc struct {
	rubricID        uint
	assignmentMigID string
	useForGrading   bool
	purpose         string
	courseID        uint
}

// applyPendingRubricAssocs runs after pass-1 to attach rubrics to the
// assignments they reference. Stored as a method on IMSCCParser so the
// caller can drive the flow without exposing the pending list.
func (p *IMSCCParser) applyPendingRubricAssocs(ctx context.Context, result *ImportResult) {
	if p.rubricAssocRepo == nil {
		return
	}
	for _, pa := range result.pendingRubricAssocs {
		ent, ok := result.entityByMigID[pa.assignmentMigID]
		if !ok || ent.Type != "Assignment" {
			continue
		}
		purpose := pa.purpose
		if purpose == "" {
			purpose = "grading"
		}
		assoc := &models.RubricAssociation{
			RubricID:        pa.rubricID,
			AssociationID:   ent.ID,
			AssociationType: "Assignment",
			ContextType:     "Course",
			ContextID:       pa.courseID,
			Purpose:         purpose,
			UseForGrading:   pa.useForGrading,
		}
		if err := p.rubricAssocRepo.Create(ctx, assoc); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("rubric association for assignment %s failed: %v", pa.assignmentMigID, err))
			continue
		}
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "RubricAssociation", ID: assoc.ID})
	}
}

// --- learning_outcomes.xml ---

type ccLearningOutcomes struct {
	XMLName xml.Name   `xml:"learningOutcomes"`
	Groups  []ccOutcomeGroup `xml:"learningOutcomeGroup"`
	Outcomes []ccOutcome     `xml:"learningOutcome"`
}

type ccOutcomeGroup struct {
	Identifier  string           `xml:"identifier,attr"`
	Title       string           `xml:"title"`
	Description string           `xml:"description"`
	Groups      []ccOutcomeGroup `xml:"learningOutcomeGroup"`
	Outcomes    []ccOutcome      `xml:"learningOutcome"`
}

type ccOutcome struct {
	Identifier        string `xml:"identifier,attr"`
	Title             string `xml:"title"`
	DisplayName       string `xml:"display_name"`
	Description       string `xml:"description"`
	CalculationMethod string `xml:"calculation_method"`
	CalculationInt    string `xml:"calculation_int"`
	MasteryPoints     string `xml:"mastery_points"`
	PointsPossible    string `xml:"points_possible"`
}

func (p *IMSCCParser) parseLearningOutcomesXML(
	ctx context.Context,
	courseID uint,
	zipFiles map[string]*zip.File,
	result *ImportResult,
) {
	if p.outcomeGroupRepo == nil || p.outcomeRepo == nil {
		return
	}
	data, err := readZipFile(zipFiles, "course_settings/learning_outcomes.xml")
	if err != nil || len(data) == 0 {
		return
	}
	var doc ccLearningOutcomes
	if err := xml.Unmarshal(data, &doc); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse learning_outcomes.xml: %v", err))
		return
	}
	root, err := p.outcomeGroupRepo.FindRootGroup(ctx, "Course", courseID)
	if err != nil || root == nil {
		// No root group yet — create one so the imported outcomes have a
		// home. Repos that auto-create may return nil here; tolerate both.
		root = &models.LearningOutcomeGroup{
			ContextType:   "Course",
			ContextID:     courseID,
			Title:         "Course Outcomes",
			WorkflowState: "active",
		}
		if err := p.outcomeGroupRepo.Create(ctx, root); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not create outcome root group: %v", err))
			return
		}
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "LearningOutcomeGroup", ID: root.ID})
	}
	for _, g := range doc.Groups {
		p.importOutcomeGroup(ctx, courseID, root.ID, g, result)
	}
	for _, o := range doc.Outcomes {
		p.importOutcome(ctx, courseID, root.ID, o, result)
	}
}

func (p *IMSCCParser) importOutcomeGroup(
	ctx context.Context,
	courseID uint,
	parentID uint,
	g ccOutcomeGroup,
	result *ImportResult,
) {
	parent := parentID
	group := &models.LearningOutcomeGroup{
		ContextType:   "Course",
		ContextID:     courseID,
		ParentGroupID: &parent,
		Title:         strings.TrimSpace(g.Title),
		Description:   g.Description,
		WorkflowState: "active",
	}
	if group.Title == "" {
		group.Title = "Imported Group"
	}
	if err := p.outcomeGroupRepo.Create(ctx, group); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("outcome group %q failed: %v", group.Title, err))
		return
	}
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "LearningOutcomeGroup", ID: group.ID})
	for _, sub := range g.Groups {
		p.importOutcomeGroup(ctx, courseID, group.ID, sub, result)
	}
	for _, o := range g.Outcomes {
		p.importOutcome(ctx, courseID, group.ID, o, result)
	}
}

func (p *IMSCCParser) importOutcome(
	ctx context.Context,
	courseID uint,
	groupID uint,
	o ccOutcome,
	result *ImportResult,
) {
	out := &models.LearningOutcome{
		ContextType:    "Course",
		ContextID:      courseID,
		OutcomeGroupID: groupID,
		Title:          strings.TrimSpace(o.Title),
		DisplayName:    o.DisplayName,
		Description:    o.Description,
		WorkflowState:  "active",
	}
	if out.Title == "" {
		out.Title = "Imported Outcome"
	}
	if o.CalculationMethod != "" {
		out.CalculationMethod = o.CalculationMethod
	}
	if v, err := strconv.Atoi(o.CalculationInt); err == nil {
		out.CalculationInt = v
	}
	if v, err := strconv.ParseFloat(o.MasteryPoints, 64); err == nil {
		out.MasteryPoints = v
	}
	if v, err := strconv.ParseFloat(o.PointsPossible, 64); err == nil {
		out.PointsPossible = v
	}
	if err := p.outcomeRepo.Create(ctx, out); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("outcome %q failed: %v", out.Title, err))
		return
	}
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "LearningOutcome", ID: out.ID})
}

// --- events.xml ---

type ccEvents struct {
	XMLName xml.Name  `xml:"events"`
	Events  []ccEvent `xml:"event"`
}

type ccEvent struct {
	Identifier      string `xml:"identifier,attr"`
	Title           string `xml:"title"`
	Description     string `xml:"description"`
	StartAt         string `xml:"start_at"`
	EndAt           string `xml:"end_at"`
	LocationName    string `xml:"location_name"`
	LocationAddress string `xml:"location_address"`
	AllDay          string `xml:"all_day"`
}

func (p *IMSCCParser) parseEventsXML(
	ctx context.Context,
	courseID uint,
	zipFiles map[string]*zip.File,
	result *ImportResult,
) {
	if p.calendarEventRepo == nil {
		return
	}
	data, err := readZipFile(zipFiles, "course_settings/events.xml")
	if err != nil || len(data) == 0 {
		return
	}
	var doc ccEvents
	if err := xml.Unmarshal(data, &doc); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse events.xml: %v", err))
		return
	}
	for _, e := range doc.Events {
		startPtr := parseCanvasTime(e.StartAt)
		if startPtr == nil {
			continue
		}
		uid := result.ownerUserID
		if uid == 0 {
			uid = 1
		}
		ev := &models.CalendarEvent{
			ContextType:     "Course",
			ContextID:       courseID,
			Title:           strings.TrimSpace(e.Title),
			Description:     e.Description,
			StartAt:         *startPtr,
			EndAt:           parseCanvasTime(e.EndAt),
			LocationName:    e.LocationName,
			LocationAddress: e.LocationAddress,
			AllDay:          parseBoolFlag(e.AllDay),
			CreatedByUserID: uid,
			WorkflowState:   "active",
		}
		if ev.Title == "" {
			ev.Title = "Imported Event"
		}
		if err := p.calendarEventRepo.Create(ctx, ev); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("calendar event %q failed: %v", ev.Title, err))
			continue
		}
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "CalendarEvent", ID: ev.ID})
	}
}

// --- BasicLTI tools ---

// warnSkippedLTI parses a BasicLTI resource just enough to surface what was
// in the cartridge so admins know what to wire up manually. We don't write
// to the database because ContextExternalTool requires a non-null
// DeveloperKeyID FK; the import flow has no clean way to mint or look up a
// key automatically.
func (p *IMSCCParser) warnSkippedLTI(
	zipFiles map[string]*zip.File,
	res ManifestResource,
	result *ImportResult,
) {
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}
	title := titleFromHref(href)
	launchURL := ""
	if href != "" {
		if data, err := readZipFile(zipFiles, href); err == nil {
			type ccLTI struct {
				XMLName     xml.Name `xml:"cartridge_basiclti_link"`
				Title       string   `xml:"title"`
				LaunchURL   string   `xml:"launch_url"`
				SecureURL   string   `xml:"secure_launch_url"`
				Description string   `xml:"description"`
			}
			var lti ccLTI
			if xerr := xml.Unmarshal(data, &lti); xerr == nil {
				if lti.Title != "" {
					title = lti.Title
				}
				launchURL = lti.LaunchURL
				if launchURL == "" {
					launchURL = lti.SecureURL
				}
			}
		}
	}
	if title == "" {
		title = res.Identifier
	}
	result.Warnings = append(result.Warnings, fmt.Sprintf("LTI tool %q (launch %s) skipped: developer key required, configure manually under Admin → Developer Keys", title, launchURL))
}
