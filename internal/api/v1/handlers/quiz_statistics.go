package handlers

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizStatisticsHandler struct {
	quizService *service.QuizService
}

func NewQuizStatisticsHandler(quizService *service.QuizService) *QuizStatisticsHandler {
	return &QuizStatisticsHandler{quizService: quizService}
}

// answerOptionStat represents one answer choice for statistics purposes.
type answerOptionStat struct {
	ID     string  `json:"id"`
	Text   string  `json:"text"`
	Weight float64 `json:"weight"`
}

// questionStatistic holds per-question item analysis data.
type questionStatistic struct {
	QuestionID     uint                `json:"question_id"`
	QuestionText   string              `json:"question_text"`
	QuestionType   string              `json:"question_type"`
	PointsPossible float64             `json:"points_possible"`
	Responses      int                 `json:"responses"`
	Correct        int                 `json:"correct"`
	Incorrect      int                 `json:"incorrect"`
	Unanswered     int                 `json:"unanswered"`
	DifficultyIdx  float64             `json:"difficulty_index"`  // % correct
	AverageScore   float64             `json:"average_score"`
	Answers        []answerStatistic   `json:"answers,omitempty"` // per-option breakdown
}

// answerStatistic holds the count for each answer option.
type answerStatistic struct {
	ID      string  `json:"id"`
	Text    string  `json:"text"`
	Correct bool    `json:"correct"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// quizLevelStatistic holds summary statistics for the entire quiz.
type quizLevelStatistic struct {
	SubmissionCount int     `json:"submission_count"`
	AverageScore    float64 `json:"average_score"`
	HighScore       float64 `json:"high_score"`
	LowScore        float64 `json:"low_score"`
	MedianScore     float64 `json:"median_score"`
	StdDev          float64 `json:"standard_deviation"`
	PointsPossible  float64 `json:"points_possible"`
}

// GetQuizStatistics handles GET /courses/:course_id/quizzes/:quiz_id/statistics
func (h *QuizStatisticsHandler) GetQuizStatistics(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	ctx := c.Context()

	// 1. Fetch quiz to get points_possible
	quiz, err := h.quizService.GetQuiz(ctx, uint(quizID))
	if err != nil {
		return responses.NotFound(c, "quiz")
	}

	// 2. Fetch all questions for this quiz
	questions, err := h.quizService.ListAllQuestions(ctx, uint(quizID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch quiz questions")
	}

	// 3. Fetch all completed submissions (complete + pending_review)
	submissions, err := h.quizService.ListAllCompletedSubmissions(ctx, uint(quizID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch quiz submissions")
	}

	quizPointsPossible := 0.0
	if quiz.PointsPossible != nil {
		quizPointsPossible = *quiz.PointsPossible
	}

	if len(submissions) == 0 {
		return c.JSON(fiber.Map{
			"quiz_statistics": fiber.Map{
				"quiz_id":            quizID,
				"quiz_level":         quizLevelStatistic{PointsPossible: quizPointsPossible},
				"question_statistics": []questionStatistic{},
			},
		})
	}

	// 4. Fetch all answers for all completed submissions
	submissionIDs := make([]uint, len(submissions))
	for i, s := range submissions {
		submissionIDs[i] = s.ID
	}

	allAnswers, err := h.quizService.ListAnswersBySubmissionIDs(ctx, submissionIDs)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submission answers")
	}

	// Build lookup: questionID -> list of answers
	answersByQuestion := make(map[uint][]answerInfo)
	for _, a := range allAnswers {
		info := answerInfo{
			Answer:  a.Answer,
			Correct: a.Correct,
			Points:  a.Points,
		}
		answersByQuestion[a.QuestionID] = append(answersByQuestion[a.QuestionID], info)
	}

	// 5. Calculate per-question statistics
	totalSubmissions := len(submissions)
	questionStats := make([]questionStatistic, 0, len(questions))

	for _, q := range questions {
		pp := 1.0
		if q.PointsPossible != nil {
			pp = *q.PointsPossible
		}

		answers := answersByQuestion[q.ID]
		responded := len(answers)
		unanswered := totalSubmissions - responded

		correctCount := 0
		incorrectCount := 0
		totalPoints := 0.0

		for _, a := range answers {
			if a.Points != nil {
				totalPoints += *a.Points
			}
			if a.Correct != nil && *a.Correct {
				correctCount++
			} else if a.Correct != nil && !*a.Correct {
				incorrectCount++
			}
		}

		difficultyIdx := 0.0
		avgScore := 0.0
		if responded > 0 {
			difficultyIdx = math.Round(float64(correctCount)/float64(responded)*10000) / 100
			avgScore = math.Round(totalPoints/float64(responded)*100) / 100
		}

		qs := questionStatistic{
			QuestionID:     q.ID,
			QuestionText:   q.QuestionText,
			QuestionType:   q.QuestionType,
			PointsPossible: pp,
			Responses:      responded,
			Correct:        correctCount,
			Incorrect:      incorrectCount,
			Unanswered:     unanswered,
			DifficultyIdx:  difficultyIdx,
			AverageScore:   avgScore,
		}

		// For multiple_choice and true_false, build answer distribution
		if q.QuestionType == "multiple_choice" || q.QuestionType == "true_false" {
			var options []answerOptionStat
			if err := json.Unmarshal([]byte(q.Answers), &options); err == nil && len(options) > 0 {
				// Count how many students picked each option
				optionCounts := make(map[string]int)
				for _, a := range answers {
					optionCounts[a.Answer]++
				}

				answerStats := make([]answerStatistic, 0, len(options))
				for _, opt := range options {
					count := optionCounts[opt.ID]
					pct := 0.0
					if responded > 0 {
						pct = math.Round(float64(count)/float64(responded)*10000) / 100
					}
					answerStats = append(answerStats, answerStatistic{
						ID:      opt.ID,
						Text:    opt.Text,
						Correct: opt.Weight > 0,
						Count:   count,
						Percent: pct,
					})
				}
				qs.Answers = answerStats
			}
		}

		questionStats = append(questionStats, qs)
	}

	// 6. Calculate quiz-level statistics
	scores := make([]float64, 0, len(submissions))
	for _, s := range submissions {
		if s.Score != nil {
			scores = append(scores, *s.Score)
		}
	}

	quizLevel := quizLevelStatistic{
		SubmissionCount: len(submissions),
		PointsPossible:  quizPointsPossible,
	}

	if len(scores) > 0 {
		sort.Float64s(scores)

		sum := 0.0
		for _, s := range scores {
			sum += s
		}
		avg := sum / float64(len(scores))
		quizLevel.AverageScore = math.Round(avg*100) / 100
		quizLevel.HighScore = scores[len(scores)-1]
		quizLevel.LowScore = scores[0]

		// Median
		n := len(scores)
		if n%2 == 0 {
			quizLevel.MedianScore = math.Round((scores[n/2-1]+scores[n/2])/2*100) / 100
		} else {
			quizLevel.MedianScore = scores[n/2]
		}

		// Standard deviation (population)
		varianceSum := 0.0
		for _, s := range scores {
			diff := s - avg
			varianceSum += diff * diff
		}
		quizLevel.StdDev = math.Round(math.Sqrt(varianceSum/float64(len(scores)))*100) / 100
	}

	return c.JSON(fiber.Map{
		"quiz_statistics": fiber.Map{
			"quiz_id":             quizID,
			"quiz_level":          quizLevel,
			"question_statistics": questionStats,
		},
	})
}

// answerInfo is used internally for aggregation.
type answerInfo struct {
	Answer  string
	Correct *bool
	Points  *float64
}
