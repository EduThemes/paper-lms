package wiring_test

// DB-integration test for the rubric-assessment emit adapter. Mirrors the
// shape of submission_test.go: seed a real tenant+course+rubric+assessment
// chain, run the callback, assert exactly one gamification_events row with
// the expected verb/object/tenant + result/context JSONB.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
)

func TestRubricAssessmentCreatedEmitCallback_EmitsEvent(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	// Seed account → course → rubric (Course-scoped) → association → assessment.
	account := models.Account{Name: "Rubric Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	course := models.Course{
		AccountID:     account.ID,
		Name:          "Rubric Course",
		CourseCode:    "RC-1",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}
	rubric := models.Rubric{
		ContextType:    "Course",
		ContextID:      course.ID,
		Title:          "Essay Rubric",
		PointsPossible: 10,
		WorkflowState:  "active",
	}
	if err := g.Create(&rubric).Error; err != nil {
		t.Fatalf("create rubric: %v", err)
	}
	assoc := models.RubricAssociation{
		RubricID:        rubric.ID,
		AssociationID:   999, // synthetic assignment id; FK not enforced
		AssociationType: "Assignment",
		ContextType:     "Course",
		ContextID:       course.ID,
		Purpose:         "grading",
		UseForGrading:   true,
	}
	if err := g.Create(&assoc).Error; err != nil {
		t.Fatalf("create rubric_association: %v", err)
	}
	student := seedTestUser(t, g, account.ID, "rubric-student@example.test")
	teacher := seedTestUser(t, g, account.ID, "rubric-teacher@example.test")
	score := 8.5
	criterionData := `{"criterion_1": {"points": 5, "comments": "great"}}`
	assessment := models.RubricAssessment{
		RubricID:            rubric.ID,
		RubricAssociationID: assoc.ID,
		UserID:              student.ID,
		AssessorID:          teacher.ID,
		Score:               &score,
		Data:                criterionData,
		AssessmentType:      "grading",
		WorkflowState:       "active",
	}
	if err := g.Create(&assessment).Error; err != nil {
		t.Fatalf("create rubric_assessment: %v", err)
	}

	// Seed system currencies so the FERPA preflight + event persistence
	// path inside Emit has somewhere to land.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}

	emitter := buildEmitter(t, g)
	cb := wiring.RubricAssessmentCreatedEmitCallback(
		emitter,
		pgrepo.NewRubricAssessmentRepository(g),
		pgrepo.NewRubricRepository(g),
		pgrepo.NewCourseRepository(g),
	)

	cb(ctx, assessment.ID)

	var events []models.GamificationEvent
	if err := g.Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 gamification_events row, got %d", len(events))
	}
	ev := events[0]
	if ev.Verb != gamification.VerbAssessed {
		t.Errorf("Verb = %q, want %q", ev.Verb, gamification.VerbAssessed)
	}
	if ev.ObjectType != gamification.ObjectRubric {
		t.Errorf("ObjectType = %q, want %q", ev.ObjectType, gamification.ObjectRubric)
	}
	if ev.TenantID != account.ID {
		t.Errorf("TenantID = %d, want %d", ev.TenantID, account.ID)
	}
	if ev.ActorID != assessment.UserID {
		t.Errorf("ActorID = %d, want %d (student UserID)", ev.ActorID, assessment.UserID)
	}
	if ev.ObjectID == nil || *ev.ObjectID != rubric.ID {
		t.Errorf("ObjectID = %v, want %d (rubric.ID)", ev.ObjectID, rubric.ID)
	}
	if ev.Source != gamification.EmitterSource {
		t.Errorf("Source = %q, want %q", ev.Source, gamification.EmitterSource)
	}

	// Result JSONB carries assessment_id and an inlined `data` object with
	// per-criterion ratings (criterion_1.points == 5).
	var resultDecoded map[string]any
	if err := json.Unmarshal(ev.Result, &resultDecoded); err != nil {
		t.Fatalf("decode result: %v (raw=%s)", err, string(ev.Result))
	}
	gotAssessmentID, ok := resultDecoded["assessment_id"].(float64)
	if !ok {
		t.Fatalf("result.assessment_id not a number: %v (raw=%s)", resultDecoded["assessment_id"], string(ev.Result))
	}
	if uint(gotAssessmentID) != assessment.ID {
		t.Errorf("result.assessment_id = %v, want %d", gotAssessmentID, assessment.ID)
	}
	dataField, ok := resultDecoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("result.data not an object: %v (raw=%s)", resultDecoded["data"], string(ev.Result))
	}
	criterion1, ok := dataField["criterion_1"].(map[string]any)
	if !ok {
		t.Fatalf("result.data.criterion_1 not an object: %v (raw=%s)", dataField["criterion_1"], string(ev.Result))
	}
	gotPoints, ok := criterion1["points"].(float64)
	if !ok {
		t.Fatalf("result.data.criterion_1.points not a number: %v (raw=%s)", criterion1["points"], string(ev.Result))
	}
	if gotPoints != 5 {
		t.Errorf("result.data.criterion_1.points = %v, want 5", gotPoints)
	}

	// Context JSONB carries rubric_id + context_type=Course.
	var contextDecoded map[string]any
	if err := json.Unmarshal(ev.Context, &contextDecoded); err != nil {
		t.Fatalf("decode context: %v (raw=%s)", err, string(ev.Context))
	}
	gotRubricID, ok := contextDecoded["rubric_id"].(float64)
	if !ok {
		t.Fatalf("context.rubric_id not a number: %v (raw=%s)", contextDecoded["rubric_id"], string(ev.Context))
	}
	if uint(gotRubricID) != rubric.ID {
		t.Errorf("context.rubric_id = %v, want %d", gotRubricID, rubric.ID)
	}
	if got := contextDecoded["context_type"]; got != "Course" {
		t.Errorf("context.context_type = %v, want %q", got, "Course")
	}
}
