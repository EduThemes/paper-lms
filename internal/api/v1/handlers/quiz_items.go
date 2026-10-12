package handlers

// Quiz-items lane handlers: delivery settings on the quiz (shuffle answers,
// one question at a time, can't go back, access code, IP filter, anonymous
// survey submissions), their enforcement while taking a quiz, per-attempt
// question rendering (shuffled choices, formula variables, locked
// questions), and the formula authoring preview.

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ---------- Quiz JSON ----------

// addQuizDeliveryJSON adds the delivery settings every viewer may see.
// The access code itself and the IP filter are staff-only
// (addQuizStaffDeliveryJSON); students only learn that one is required.
func addQuizDeliveryJSON(m fiber.Map, q *models.Quiz) fiber.Map {
	m["shuffle_answers"] = q.ShuffleAnswers
	m["one_question_at_a_time"] = q.OneQuestionAtATime
	m["cant_go_back"] = service.CantGoBackActive(q)
	m["anonymous_submissions"] = q.AnonymousSubmissions
	m["requires_access_code"] = len(q.AccessCodeEncrypted) > 0
	m["has_ip_filter"] = strings.TrimSpace(q.IPFilter) != ""
	return m
}

// addQuizStaffDeliveryJSON adds the access code and IP filter for staff.
func addQuizStaffDeliveryJSON(m fiber.Map, q *models.Quiz) fiber.Map {
	code, err := quizAccessCode(q)
	if err != nil {
		code = ""
		m["access_code_unavailable"] = true
	}
	m["access_code"] = code
	m["ip_filter"] = q.IPFilter
	return m
}

// Staff see the access code and IP filter through
// (*QuizHandler).quizToViewerJSON in quizzes.go.

// ---------- Settings input ----------

// quizDeliveryInput is the delivery-settings slice of a quiz create/update
// body ({"quiz": {...}}). Pointers distinguish "absent" from false/"".
type quizDeliveryInput struct {
	Quiz struct {
		ShuffleAnswers       *bool   `json:"shuffle_answers"`
		OneQuestionAtATime   *bool   `json:"one_question_at_a_time"`
		CantGoBack           *bool   `json:"cant_go_back"`
		AccessCode           *string `json:"access_code"`
		IPFilter             *string `json:"ip_filter"`
		AnonymousSubmissions *bool   `json:"anonymous_submissions"`
	} `json:"quiz"`
}

// applyQuizDeliveryInput reads the delivery settings from the request body
// onto quiz. Returns wrote=true after writing a 400/500; the caller must
// then return nil.
func applyQuizDeliveryInput(c *fiber.Ctx, quiz *models.Quiz) bool {
	var in quizDeliveryInput
	if body := c.Body(); len(body) > 0 {
		if err := json.Unmarshal(body, &in); err != nil {
			// The main body parser already accepted the request (it may be
			// form-encoded); there are simply no delivery settings to read.
			return false
		}
	}
	s := in.Quiz
	if s.ShuffleAnswers != nil {
		quiz.ShuffleAnswers = *s.ShuffleAnswers
	}
	if s.OneQuestionAtATime != nil {
		quiz.OneQuestionAtATime = *s.OneQuestionAtATime
	}
	if s.CantGoBack != nil {
		quiz.CantGoBack = *s.CantGoBack
	}
	// Canvas: can't-go-back only exists on one-question-at-a-time quizzes.
	if !quiz.OneQuestionAtATime {
		quiz.CantGoBack = false
	}
	if s.AnonymousSubmissions != nil {
		quiz.AnonymousSubmissions = *s.AnonymousSubmissions
	}
	if s.IPFilter != nil {
		norm, err := service.NormalizeIPFilter(*s.IPFilter)
		if err != nil {
			_ = responses.BadRequest(c, err.Error())
			return true
		}
		quiz.IPFilter = norm
	}
	if s.AccessCode != nil {
		if err := setQuizAccessCode(quiz, *s.AccessCode); err != nil {
			if errors.Is(err, errAccessCodeUnavailable) {
				_ = responses.InternalError(c, err.Error())
			} else {
				_ = responses.BadRequest(c, err.Error())
			}
			return true
		}
	}
	return false
}

// ---------- Access code (secretbox-sealed) ----------

var errAccessCodeUnavailable = errors.New("access codes are unavailable: the encryption key is not configured")

// setQuizAccessCode seals code onto the quiz; an empty code clears it.
func setQuizAccessCode(q *models.Quiz, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		q.AccessCodeEncrypted = nil
		return nil
	}
	if len(code) > 255 {
		return errors.New("access code must be 255 characters or fewer")
	}
	sealed, err := auth.Encrypt([]byte(code))
	if err != nil {
		return errAccessCodeUnavailable
	}
	q.AccessCodeEncrypted = sealed
	return nil
}

// quizAccessCode opens the quiz's access code ("" when none).
func quizAccessCode(q *models.Quiz) (string, error) {
	if q == nil || len(q.AccessCodeEncrypted) == 0 {
		return "", nil
	}
	plain, err := auth.Decrypt(q.AccessCodeEncrypted)
	if err != nil {
		return "", errAccessCodeUnavailable
	}
	return string(plain), nil
}

// checkQuizAccessCode compares the submitted code in constant time. A
// quiz without a code accepts anything; a code that cannot be opened
// accepts nothing.
func checkQuizAccessCode(q *models.Quiz, submitted string) bool {
	if len(q.AccessCodeEncrypted) == 0 {
		return true
	}
	want, err := quizAccessCode(q)
	if err != nil || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(strings.TrimSpace(submitted))) == 1
}

// ---------- Enforcement while taking ----------

// quizClientIP is the address the IP filter checks. When the direct peer is
// a loopback/private address (the app's reverse proxy), the right-most
// X-Forwarded-For entry — the one that proxy appended — is the client;
// otherwise the peer address itself is used, so a client talking to the
// app directly cannot spoof its address with the header.
func quizClientIP(c *fiber.Ctx) string {
	peer := c.IP()
	ip := net.ParseIP(peer)
	if ip == nil || (!ip.IsLoopback() && !ip.IsPrivate()) {
		return peer
	}
	xff := strings.TrimSpace(c.Get(fiber.HeaderXForwardedFor))
	if xff == "" {
		return peer
	}
	parts := strings.Split(xff, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	if net.ParseIP(last) == nil {
		return peer
	}
	return last
}

// quizHasTakeRestrictions reports whether the quiz is gated by an access
// code or an IP filter.
func quizHasTakeRestrictions(q *models.Quiz) bool {
	return len(q.AccessCodeEncrypted) > 0 || strings.TrimSpace(q.IPFilter) != ""
}

// enforceQuizTakeRestrictions applies the IP filter (and, when
// checkAccessCode, the access code from the request body) to a non-staff
// caller. Returns wrote=true after writing a 403; the caller must return
// nil. Staff (teachers previewing their quiz) are never gated (Canvas).
func enforceQuizTakeRestrictions(c *fiber.Ctx, quiz *models.Quiz, checkAccessCode bool) bool {
	if quizCallerIsStaff(c) {
		return false
	}
	if !service.QuizIPAllowed(quiz.IPFilter, quizClientIP(c)) {
		_ = responses.Forbidden(c, "This quiz can only be taken from an approved network (IP address denied)")
		return true
	}
	if checkAccessCode && len(quiz.AccessCodeEncrypted) > 0 {
		var body struct {
			AccessCode string `json:"access_code"`
		}
		if raw := c.Body(); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		if body.AccessCode == "" {
			body.AccessCode = c.Query("access_code")
		}
		if !checkQuizAccessCode(quiz, body.AccessCode) {
			c.Set("X-Quiz-Access-Code", "required")
			_ = responses.Forbidden(c, "invalid access code")
			return true
		}
	}
	return false
}

// anonymousSurveyHidden reports whether the caller must not see this
// attempt's individual responses: on an anonymous survey only the
// respondent reads their own answers; staff see the aggregate reports.
func anonymousSurveyHidden(c *fiber.Ctx, quiz *models.Quiz, sub *models.QuizSubmission) bool {
	if quiz == nil || sub == nil || !quiz.AnonymousSubmissions || !service.QuizIsSurvey(quiz) {
		return false
	}
	userID, _ := c.Locals("user_id").(uint)
	return userID == 0 || userID != sub.UserID
}

// ---------- Attempt question rendering ----------

// attemptQuestionJSON renders one question of an attempt: the viewer JSON
// (key redacted unless showKey), the attempt's formula variables
// substituted into the text, the per-attempt shuffled choice order for
// students on a shuffle_answers quiz, and the can't-go-back lock flag.
func attemptQuestionJSON(quiz *models.Quiz, submission *models.QuizSubmission, q *models.QuizQuestion, showKey, staff bool, stimuli map[uint]*models.QuizStimulus, read map[uint]bool) fiber.Map {
	personal, inst := service.PersonalizeFormulaQuestion(*q, submission.ID)
	m := quizQuestionToViewerJSON(&personal, showKey, service.QuizShuffleSeed(submission.ID, q.ID), stimuli)
	if inst != nil {
		m["formula_variables"] = inst.Values
		if showKey {
			m["formula_answer"] = inst.Expected
			m["formula_margin"] = inst.Margin
		}
	}
	if quiz.ShuffleAnswers && !staff {
		if answers, ok := m["answers"].(string); ok {
			m["answers"] = service.ShuffleQuizAnswers(q.QuestionType, answers, service.QuizShuffleSeed(submission.ID, q.ID))
		}
	}
	if service.CantGoBackActive(quiz) && read[q.ID] {
		m["locked"] = true
	}
	return m
}

// MarkQuestionRead handles
// POST /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id/read
//
// The take page calls it when a student moves past a question on a
// one-question-at-a-time + can't-go-back quiz; from then on the server
// refuses answers to that question (Canvas "_question_N_read" marker).
func (h *QuizSubmissionHandler) MarkQuestionRead(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	quiz, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID))
	if wrote {
		return nil
	}
	if enforceQuizTakeRestrictions(c, quiz, false) {
		return nil
	}
	submission, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 || submission.UserID != userID {
		return responses.NotFound(c, "quiz submission")
	}
	read, err := h.quizService.MarkQuestionRead(c.Context(), submission, uint(questionID))
	if err != nil {
		if errors.Is(err, service.ErrQuestionNotInSubmission) {
			return responses.NotFound(c, "quiz question")
		}
		return responses.BadRequest(c, err.Error())
	}
	ids := make([]uint, 0, len(read))
	for id := range read {
		ids = append(ids, id)
	}
	return c.JSON(fiber.Map{"read_question_ids": ids, "cant_go_back": service.CantGoBackActive(quiz)})
}

// ---------- Formula preview ----------

// FormulaPreview handles POST /courses/:course_id/quizzes/:quiz_id/formula_preview.
// Body: {"answers": [ <formula config> ], "count": 5}. Returns sample
// variable sets with their computed answers so the author can check the
// formula before students see it.
func (h *QuizQuestionHandler) FormulaPreview(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if _, wrote := loadQuizForCaller(c, h.quizService, uint(quizID), uint(courseID)); wrote {
		return nil
	}
	var in struct {
		Answers json.RawMessage `json:"answers"`
		Count   int             `json:"count"`
	}
	if err := json.Unmarshal(c.Body(), &in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	raw := strings.TrimSpace(string(in.Answers))
	// The editor may send the answers array as a JSON string.
	if strings.HasPrefix(raw, `"`) {
		var s string
		if err := json.Unmarshal(in.Answers, &s); err == nil {
			raw = s
		}
	}
	cfg, ok := service.ParseFormulaConfig(raw)
	if !ok {
		return responses.BadRequest(c, "A formula is required")
	}
	samples, err := service.FormulaPreview(cfg, in.Count)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"samples": samples})
}
