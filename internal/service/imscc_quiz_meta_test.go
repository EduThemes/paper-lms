package service

import (
	"encoding/xml"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

const sampleAssessmentMeta = `<?xml version="1.0" encoding="UTF-8"?>
<quiz identifier="g1" xmlns="http://canvas.instructure.com/xsd/cccv1p0">
  <title>Mid-Unit Check</title>
  <description>Quick check.</description>
  <quiz_type>practice_quiz</quiz_type>
  <points_possible>10.0</points_possible>
  <time_limit>30</time_limit>
  <allowed_attempts>3</allowed_attempts>
  <shuffle_answers>true</shuffle_answers>
  <scoring_policy>keep_latest</scoring_policy>
  <show_correct_answers>false</show_correct_answers>
  <hide_results>until_after_last_attempt</hide_results>
  <one_question_at_a_time>true</one_question_at_a_time>
  <cant_go_back>true</cant_go_back>
  <available>true</available>
  <due_at>2026-06-01T23:59:00Z</due_at>
  <lock_at>2026-06-15T23:59:00Z</lock_at>
  <unlock_at>2026-05-15T00:00:00Z</unlock_at>
  <assignment identifier="ag1">
    <due_at/>
    <lock_at/>
    <unlock_at/>
  </assignment>
</quiz>`

func TestApplyQuizMeta(t *testing.T) {
	var meta canvasQuizMeta
	if err := xml.Unmarshal([]byte(sampleAssessmentMeta), &meta); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}

	q := &models.Quiz{ScoringPolicy: "keep_highest", ShowCorrectAnswers: true}
	applyQuizMeta(q, &meta)

	if q.Title != "Mid-Unit Check" {
		t.Errorf("title = %q", q.Title)
	}
	if q.QuizType != "practice_quiz" {
		t.Errorf("quiz_type = %q", q.QuizType)
	}
	if q.AllowedAttempts != 3 {
		t.Errorf("allowed_attempts = %d, want 3", q.AllowedAttempts)
	}
	if q.TimeLimit == nil || *q.TimeLimit != 30 {
		t.Errorf("time_limit = %v, want 30", q.TimeLimit)
	}
	if q.PointsPossible == nil || *q.PointsPossible != 10.0 {
		t.Errorf("points_possible = %v, want 10.0", q.PointsPossible)
	}
	if !q.ShuffleAnswers {
		t.Errorf("shuffle_answers = false, want true")
	}
	if q.ScoringPolicy != "keep_latest" {
		t.Errorf("scoring_policy = %q, want keep_latest", q.ScoringPolicy)
	}
	if q.ShowCorrectAnswers {
		t.Errorf("show_correct_answers = true, want false")
	}
	if q.HideResults != "until_after_last_attempt" {
		t.Errorf("hide_results = %q", q.HideResults)
	}
	if !q.OneQuestionAtATime || !q.CantGoBack {
		t.Errorf("OneQuestionAtATime/CantGoBack not set")
	}
	if !q.Published || q.WorkflowState != "published" {
		t.Errorf("available=true should publish, got published=%v state=%q", q.Published, q.WorkflowState)
	}
	if q.DueAt == nil || q.LockAt == nil || q.UnlockAt == nil {
		t.Errorf("dates not parsed: due=%v lock=%v unlock=%v", q.DueAt, q.LockAt, q.UnlockAt)
	}
}

func TestParseCanvasTime(t *testing.T) {
	cases := map[string]bool{
		"":                     false,
		"   ":                  false,
		"2026-05-09T12:00:00Z": true,
		"2026-05-09":           true,
		"not a date":           false,
	}
	for in, wantOK := range cases {
		got := parseCanvasTime(in)
		if (got != nil) != wantOK {
			t.Errorf("parseCanvasTime(%q) ok=%v, want %v", in, got != nil, wantOK)
		}
	}
}

const sampleAssignmentGroups = `<?xml version="1.0" encoding="UTF-8"?>
<assignmentGroups xmlns="http://canvas.instructure.com/xsd/cccv1p0">
  <assignmentGroup identifier="g_main">
    <title>Assignments</title>
    <position>1</position>
    <group_weight>0.0</group_weight>
  </assignmentGroup>
  <assignmentGroup identifier="g_imported">
    <title>Imported Assignments</title>
    <position>2</position>
    <group_weight>50.0</group_weight>
  </assignmentGroup>
</assignmentGroups>`

func TestUnmarshalAssignmentGroups(t *testing.T) {
	var groups canvasAssignmentGroups
	if err := xml.Unmarshal([]byte(sampleAssignmentGroups), &groups); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	if len(groups.Groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups.Groups))
	}
	if groups.Groups[1].Identifier != "g_imported" || groups.Groups[1].Title != "Imported Assignments" {
		t.Errorf("group[1] = %+v", groups.Groups[1])
	}
	if groups.Groups[1].GroupWeight != "50.0" {
		t.Errorf("group_weight = %q, want 50.0", groups.Groups[1].GroupWeight)
	}
}

const sampleCanvasAssignment = `<?xml version="1.0" encoding="UTF-8"?>
<assignment identifier="a1" xmlns="http://canvas.instructure.com/xsd/cccv1p0">
  <title>Homework 3</title>
  <due_at>2026-05-12T23:59:00Z</due_at>
  <unlock_at>2026-05-09T00:00:00Z</unlock_at>
  <lock_at>2026-05-19T23:59:00Z</lock_at>
  <points_possible>10.0</points_possible>
  <grading_type>points</grading_type>
  <submission_types>online_text_entry,online_upload</submission_types>
  <position>3</position>
  <workflow_state>published</workflow_state>
  <assignment_group_identifierref>g_imported</assignment_group_identifierref>
</assignment>`

func TestUnmarshalCanvasAssignment(t *testing.T) {
	var ca canvasAssignment
	if err := xml.Unmarshal([]byte(sampleCanvasAssignment), &ca); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	if ca.Title != "Homework 3" {
		t.Errorf("title = %q", ca.Title)
	}
	if ca.AssignmentGroupRef != "g_imported" {
		t.Errorf("group ref = %q", ca.AssignmentGroupRef)
	}
	if ca.WorkflowState != "published" {
		t.Errorf("state = %q", ca.WorkflowState)
	}
	if parseCanvasTime(ca.DueAt) == nil {
		t.Errorf("due_at didn't parse")
	}
}

const sampleCourseSettings = `<?xml version="1.0" encoding="UTF-8"?>
<course identifier="c1" xmlns="http://canvas.instructure.com/xsd/cccv1p0">
  <title>Algebra 1</title>
  <course_code>ALG-1</course_code>
  <is_public>false</is_public>
  <license>cc_by_nc</license>
  <default_view>wiki</default_view>
  <start_at>2026-08-15T00:00:00Z</start_at>
  <conclude_at>2026-12-20T00:00:00Z</conclude_at>
  <tab_configuration>[{"id":0},{"id":14}]</tab_configuration>
</course>`

func TestUnmarshalCourseSettings(t *testing.T) {
	var cs ccCourseSettings
	if err := xml.Unmarshal([]byte(sampleCourseSettings), &cs); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	if cs.Title != "Algebra 1" {
		t.Errorf("title = %q", cs.Title)
	}
	if cs.License != "cc_by_nc" {
		t.Errorf("license = %q", cs.License)
	}
	if cs.DefaultView != "wiki" {
		t.Errorf("default_view = %q", cs.DefaultView)
	}
	if cs.IsPublic != "false" {
		t.Errorf("is_public = %q", cs.IsPublic)
	}
	if parseCanvasTime(cs.StartAt) == nil {
		t.Errorf("start_at didn't parse")
	}
}

const sampleRubrics = `<?xml version="1.0" encoding="UTF-8"?>
<rubrics>
  <rubric identifier="r1">
    <title>Essay Rubric</title>
    <points_possible>20.0</points_possible>
    <criteria>
      <criterion id="c1">
        <description>Clarity</description>
        <points>10</points>
        <ratings>
          <rating id="r_a"><description>Excellent</description><points>10</points></rating>
          <rating id="r_b"><description>Adequate</description><points>5</points></rating>
        </ratings>
      </criterion>
    </criteria>
    <associations>
      <association>
        <association_type>assignment</association_type>
        <association_identifierref>asg1</association_identifierref>
        <use_for_grading>true</use_for_grading>
      </association>
    </associations>
  </rubric>
</rubrics>`

func TestUnmarshalRubrics(t *testing.T) {
	var doc ccRubrics
	if err := xml.Unmarshal([]byte(sampleRubrics), &doc); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}
	if len(doc.Rubrics) != 1 {
		t.Fatalf("got %d rubrics, want 1", len(doc.Rubrics))
	}
	r := doc.Rubrics[0]
	if len(r.Criteria) != 1 || r.Criteria[0].Description != "Clarity" {
		t.Errorf("criteria = %+v", r.Criteria)
	}
	if len(r.Criteria[0].Ratings) != 2 {
		t.Errorf("ratings = %+v", r.Criteria[0].Ratings)
	}
	if len(r.Associations) != 1 || r.Associations[0].AssociationRef != "asg1" {
		t.Errorf("associations = %+v", r.Associations)
	}
}

func TestIsAnnouncementTopic(t *testing.T) {
	cases := []struct {
		name string
		t    canvasDiscussionTopic
		want bool
	}{
		{"empty", canvasDiscussionTopic{}, false},
		{"plain discussion", canvasDiscussionTopic{DiscussionType: "side_comment"}, false},
		{"type element", canvasDiscussionTopic{Type: "announcement"}, true},
		{"discussion_type element", canvasDiscussionTopic{DiscussionType: "announcement"}, true},
		{"mixed case", canvasDiscussionTopic{Type: "Announcement"}, true},
		{"whitespace", canvasDiscussionTopic{Type: " announcement "}, true},
	}
	for _, c := range cases {
		got := isAnnouncementTopic(c.t, CCItemSettings{})
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNormalizeResourceTypeWithHref(t *testing.T) {
	cases := []struct {
		t, h, want string
	}{
		{"associatedcontent/imscc_xmlv1p1/learning-application-resource", "g123/assessment_meta.xml", "assessment_sidecar"},
		{"associatedcontent/imscc_xmlv1p1/learning-application-resource", "g123/something_else.xml", "learning_application"},
		{"imsqti_xmlv1p2/imscc_xmlv1p1/assessment", "g456/assessment_qti.xml", "quiz"},
		{"webcontent", "wiki_content/page.html", "webcontent"},
	}
	for _, c := range cases {
		got := normalizeResourceTypeWithHref(c.t, c.h)
		if got != c.want {
			t.Errorf("normalizeResourceTypeWithHref(%q,%q) = %q, want %q", c.t, c.h, got, c.want)
		}
	}
}
