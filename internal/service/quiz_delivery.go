package service

// Quiz delivery rules (quiz-items lane): shuffled answer order, the
// one-question-at-a-time "can't go back" lock, access codes, IP filters
// and survey grading semantics. Canvas reference: Quizzes::Quiz
// (shuffle_answers, cant_go_back, access_code, ip_filter,
// anonymous_submissions), Quizzes::QuizQuestionBuilder#shuffle_answers,
// Quizzes::QuizSubmission#sanitize_params and Quizzes::SubmissionGrader.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ---------- Shuffle answers ----------

// shuffleableAnswerTypes are the item types whose choice order is
// randomized per attempt when a quiz has shuffle_answers on. True/false
// keeps its natural order; matching (whose right-hand pool is always
// shuffled) and fill-in-multiple-blanks have no choice list to shuffle.
var shuffleableAnswerTypes = map[string]bool{
	"multiple_choice":   true,
	"multiple_answer":   true,
	"multiple_dropdown": true,
}

// ShuffleQuizAnswers returns answersJSON with its options in a per-attempt
// order when the item type is shuffleable. The order is stable for a seed,
// so the same attempt sees the same order on every reload (and on review).
func ShuffleQuizAnswers(questionType, answersJSON string, seed int64) string {
	if !shuffleableAnswerTypes[NormalizeQuizQuestionType(questionType)] {
		return answersJSON
	}
	var opts []json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(answersJSON)), &opts); err != nil || len(opts) < 2 {
		return answersJSON
	}
	b, err := json.Marshal(seededShuffle(opts, seed^0x5f3759df))
	if err != nil {
		return answersJSON
	}
	return string(b)
}

// ---------- Can't go back ----------

// ErrQuestionLocked is returned when a student answers a question they
// have already moved past on a "can't go back" quiz.
var ErrQuestionLocked = errors.New("this question is locked: you cannot go back to it")

// CantGoBackActive reports whether the quiz locks questions once a student
// moves past them (Canvas only honors cant_go_back together with
// one_question_at_a_time).
func CantGoBackActive(q *models.Quiz) bool {
	return q != nil && q.OneQuestionAtATime && q.CantGoBack
}

// ReadQuestionSet decodes a submission's read (locked) question ids.
func ReadQuestionSet(sub *models.QuizSubmission) map[uint]bool {
	out := map[uint]bool{}
	if sub == nil || strings.TrimSpace(sub.ReadQuestions) == "" {
		return out
	}
	var ids []uint
	if err := json.Unmarshal([]byte(sub.ReadQuestions), &ids); err != nil {
		return out
	}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// MarkQuestionRead records that the student moved past questionID. Only
// meaningful while the attempt is in progress on a can't-go-back quiz;
// for any other quiz it is a no-op. Returns the updated read set.
func (s *QuizService) MarkQuestionRead(ctx context.Context, submission *models.QuizSubmission, questionID uint) (map[uint]bool, error) {
	if submission.WorkflowState != "untaken" {
		return nil, errors.New("quiz submission is not in progress")
	}
	question, err := s.questionRepo.FindByID(ctx, questionID)
	if err != nil || question == nil {
		return nil, ErrQuestionNotInSubmission
	}
	selected, hasSelected := selectedQuestionSet(submission)
	if !questionBelongsToSubmission(submission, question, selected, hasSelected) {
		return nil, ErrQuestionNotInSubmission
	}
	quiz, err := s.quizRepo.FindByID(ctx, submission.QuizID, 0)
	if err != nil || quiz == nil {
		return nil, errors.New("quiz not found")
	}
	read := ReadQuestionSet(submission)
	if !CantGoBackActive(quiz) || read[questionID] {
		return read, nil
	}
	// The Postgres repository appends atomically and only while the attempt
	// is in progress, so a lock racing the attempt's completion can never
	// reset its workflow_state.
	if marker, ok := s.submissionRepo.(quizReadMarker); ok {
		stored, inProgress, err := marker.AppendReadQuestionIfInProgress(ctx, submission.ID, questionID)
		if err != nil {
			return nil, err
		}
		if !inProgress {
			return nil, errors.New("quiz submission is not in progress")
		}
		submission.ReadQuestions = stored
		return ReadQuestionSet(submission), nil
	}
	read[questionID] = true
	var ids []uint
	if strings.TrimSpace(submission.ReadQuestions) != "" {
		_ = json.Unmarshal([]byte(submission.ReadQuestions), &ids)
	}
	ids = append(ids, questionID)
	b, _ := json.Marshal(ids)
	submission.ReadQuestions = string(b)
	if err := s.submissionRepo.Update(ctx, submission); err != nil {
		return nil, err
	}
	return read, nil
}

// quizReadMarker is implemented by the Postgres quiz-submission repository.
type quizReadMarker interface {
	AppendReadQuestionIfInProgress(ctx context.Context, submissionID, questionID uint) (string, bool, error)
}

// ensureQuestionNotLocked refuses an answer to a question the student has
// already moved past on a can't-go-back quiz.
func (s *QuizService) ensureQuestionNotLocked(ctx context.Context, submission *models.QuizSubmission, questionID uint) error {
	read := ReadQuestionSet(submission)
	if !read[questionID] {
		return nil
	}
	quiz, err := s.quizRepo.FindByID(ctx, submission.QuizID, 0)
	if err != nil || quiz == nil {
		return nil
	}
	if CantGoBackActive(quiz) {
		return ErrQuestionLocked
	}
	return nil
}

// ---------- IP filter ----------

// maxIPFilterEntries bounds the filter so a request never walks an
// arbitrarily long list.
const maxIPFilterEntries = 100

// NormalizeIPFilter validates a comma/whitespace-separated list of IP
// addresses and CIDR ranges (Canvas also accepts "ip/netmask") and returns
// it in canonical comma-separated form. An empty filter means "anywhere".
func NormalizeIPFilter(filter string) (string, error) {
	entries := splitIPFilter(filter)
	if len(entries) == 0 {
		return "", nil
	}
	if len(entries) > maxIPFilterEntries {
		return "", fmt.Errorf("IP filter may list at most %d entries", maxIPFilterEntries)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, err := parseIPFilterEntry(e); err != nil {
			return "", fmt.Errorf("invalid IP filter entry %q", e)
		}
		out = append(out, e)
	}
	return strings.Join(out, ","), nil
}

func splitIPFilter(filter string) []string {
	fields := strings.FieldsFunc(filter, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// parseIPFilterEntry turns one entry into a network: a bare address is a
// single-host network, "a.b.c.d/n" a CIDR, "a.b.c.d/255.255.0.0" a
// dotted netmask.
func parseIPFilterEntry(entry string) (*net.IPNet, error) {
	if !strings.Contains(entry, "/") {
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, errors.New("bad address")
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		} else {
			ip = ip.To4()
		}
		return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}, nil
	}
	if _, n, err := net.ParseCIDR(entry); err == nil {
		return n, nil
	}
	addr, mask, _ := strings.Cut(entry, "/")
	ip := net.ParseIP(addr).To4()
	m := net.ParseIP(mask).To4()
	if ip == nil || m == nil {
		return nil, errors.New("bad network")
	}
	ipMask := net.IPv4Mask(m[0], m[1], m[2], m[3])
	if ones, bits := ipMask.Size(); ones == 0 && bits == 0 {
		return nil, errors.New("non-canonical netmask")
	}
	return &net.IPNet{IP: ip.Mask(ipMask), Mask: ipMask}, nil
}

// QuizIPAllowed reports whether remoteIP satisfies the quiz's IP filter.
// An empty filter allows everyone; an unparseable remote address is
// refused when a filter is set.
func QuizIPAllowed(filter, remoteIP string) bool {
	entries := splitIPFilter(filter)
	if len(entries) == 0 {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(remoteIP))
	if ip == nil {
		return false
	}
	for _, e := range entries {
		n, err := parseIPFilterEntry(e)
		if err != nil {
			continue
		}
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ---------- Import ----------

// quizSecretSealer seals an imported access code (secretbox lives in the
// auth package, which imports this one, so main wires it in).
var quizSecretSealer func([]byte) ([]byte, error)

// SetQuizSecretSealer registers the encryption used for imported quiz
// access codes. Without it, imported access codes are dropped.
func SetQuizSecretSealer(seal func([]byte) ([]byte, error)) {
	quizSecretSealer = seal
}

// applyQuizDeliveryMeta copies a Canvas assessment_meta.xml's access code,
// IP filter and anonymous-submissions flag onto an imported quiz. An IP
// filter Paper cannot parse is dropped rather than locking everyone out.
func applyQuizDeliveryMeta(q *models.Quiz, accessCode, ipFilter, anonymous string) {
	if norm, err := NormalizeIPFilter(ipFilter); err == nil {
		q.IPFilter = norm
	}
	q.AnonymousSubmissions = strings.EqualFold(strings.TrimSpace(anonymous), "true")
	code := strings.TrimSpace(accessCode)
	if code != "" && len(code) <= 255 && quizSecretSealer != nil {
		if sealed, err := quizSecretSealer([]byte(code)); err == nil {
			q.AccessCodeEncrypted = sealed
		}
	}
}

// ---------- Survey semantics ----------

// QuizIsSurvey reports whether the quiz is a graded or ungraded survey.
func QuizIsSurvey(q *models.Quiz) bool {
	return q != nil && (q.QuizType == "survey" || q.QuizType == "graded_survey")
}

// applySurveySemantics adjusts a finished attempt for survey quizzes
// (Canvas SubmissionGrader): there are no right answers, so nothing is
// held for manual review; a graded survey awards full points for taking
// it and an ungraded survey awards none.
func applySurveySemantics(quiz *models.Quiz, answers []models.QuizSubmissionAnswer, totalScore *float64, needsReview *bool) {
	if !QuizIsSurvey(quiz) {
		return
	}
	*needsReview = false
	via := gradedViaAuto
	for i := range answers {
		zero := 0.0
		answers[i].Correct = nil
		answers[i].Points = &zero
		answers[i].GradedVia = &via
	}
	if quiz.QuizType == "graded_survey" && quiz.PointsPossible != nil {
		*totalScore = *quiz.PointsPossible
	} else {
		*totalScore = 0
	}
}
