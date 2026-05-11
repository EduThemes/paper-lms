package qti

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

// goXMLUnmarshal is a one-line alias for encoding/xml.Unmarshal. Kept
// behind a name so xmlUnmarshal() can be stubbed in tests if needed.
func goXMLUnmarshal(data []byte, v interface{}) error {
	return xml.Unmarshal(data, v)
}

// Importer is the public entry point for the QTI package. It accepts
// either an .imscc bundle or a single QTI XML file.
//
// Persistence is the caller's job. ImportIMSCC returns an ImportResult
// with zero-ID models; the service layer copies fields onto persisted
// rows and resolves stimulus / bank references.
type Importer interface {
	ImportIMSCC(ctx context.Context, zipPath string, courseID uint) (*ImportResult, error)
	ImportQTIFile(ctx context.Context, qtiPath string, courseID uint) (*ImportResult, error)
}

// NewImporter returns the default importer implementation.
func NewImporter() Importer {
	return &defaultImporter{}
}

type defaultImporter struct{}

// ImportIMSCC reads a .imscc zip and produces an in-memory ImportResult.
// The CourseID is informational here (carried for the service layer to
// stamp on persisted rows).
func (d *defaultImporter) ImportIMSCC(ctx context.Context, zipPath string, courseID uint) (*ImportResult, error) {
	bundle, err := openIMSCC(zipPath)
	if err != nil {
		return nil, err
	}
	return importBundle(bundle)
}

// ImportQTIFile reads a standalone QTI XML file (no manifest). Detects
// dialect from the root element and routes appropriately.
func (d *defaultImporter) ImportQTIFile(ctx context.Context, qtiPath string, courseID uint) (*ImportResult, error) {
	data, err := os.ReadFile(qtiPath)
	if err != nil {
		return nil, fmt.Errorf("read qti file: %w", err)
	}
	// Detect dialect.
	head := peekHead(data)
	if strings.Contains(head, "<assessmentItem") || strings.Contains(head, "<assessmentTest") {
		return importStandaloneNewQuizzes(qtiPath, data)
	}
	if strings.Contains(head, "<questestinterop") || strings.Contains(head, "<objectbank") {
		return importStandaloneClassic(qtiPath, data)
	}
	return nil, fmt.Errorf("could not detect QTI dialect (root element not <assessmentItem>/<assessmentTest>/<questestinterop>/<objectbank>)")
}

// importBundle walks a parsed imsccBundle and returns the consolidated
// ImportResult. The bundle may contain a mix of Classic and NQ files —
// we honor whichever is referenced and tag the result with the
// majority dialect.
func importBundle(b *imsccBundle) (*ImportResult, error) {
	resetStimuli()
	result := &ImportResult{
		Quizzes:   []QuizImport{},
		ItemBanks: []ItemBankImport{},
		Stimuli:   []StimulusImport{},
	}

	classicFiles, nqFiles, bankFiles := b.findAssessmentFiles()

	// 1. Item banks first — Classic only — so we can resolve
	// <sourcebank_ref> in step 2 to the right bank identifier.
	for _, bf := range bankFiles {
		data, _, ok := b.resolvePath(bf)
		if !ok {
			result.Warnings = append(result.Warnings, ImportWarning{
				Source: bf, Code: "missing_file",
				Message: "manifest referenced bank file not found in bundle",
			})
			continue
		}
		bank, warnings, err := parseClassicBank(bf, data)
		result.Warnings = append(result.Warnings, warnings...)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Source: bf, Code: "bank_parse_error",
				Message: err.Error(),
			})
			continue
		}
		result.ItemBanks = append(result.ItemBanks, *bank)
	}

	// 2. Classic assessments.
	for _, cf := range classicFiles {
		data, _, ok := b.resolvePath(cf)
		if !ok {
			result.Warnings = append(result.Warnings, ImportWarning{
				Source: cf, Code: "missing_file",
				Message: "manifest referenced assessment file not found",
			})
			continue
		}
		quizzes, warnings, errs := parseClassicAssessmentFile(cf, data)
		result.Warnings = append(result.Warnings, warnings...)
		result.Errors = append(result.Errors, errs...)
		result.Quizzes = append(result.Quizzes, quizzes...)
		if len(quizzes) > 0 {
			result.Dialect = DialectClassic
		}
	}

	// 3. New Quizzes assessmentTest + assessmentItem files. We need to
	// find the test file (root element <assessmentTest>) and follow
	// its <assessmentItemRef> hrefs to the individual item files.
	testFiles := []string{}
	itemFiles := []string{}
	for _, f := range nqFiles {
		data, _, ok := b.resolvePath(f)
		if !ok {
			continue
		}
		if strings.Contains(peekHead(data), "<assessmentTest") {
			testFiles = append(testFiles, f)
		} else {
			itemFiles = append(itemFiles, f)
		}
	}

	// Index item files by their declared identifier so we can resolve
	// itemRef hrefs.
	itemByHref := map[string][]byte{}
	for _, f := range itemFiles {
		if data, _, ok := b.resolvePath(f); ok {
			itemByHref[f] = data
		}
	}

	for _, tf := range testFiles {
		data, _, _ := b.resolvePath(tf)
		quiz, warnings, errs := parseNewQuizzesAssessmentTest(tf, data, itemByHref, b)
		result.Warnings = append(result.Warnings, warnings...)
		result.Errors = append(result.Errors, errs...)
		if quiz != nil {
			result.Quizzes = append(result.Quizzes, *quiz)
			result.Dialect = DialectNewQuizzes
		}
	}

	// 4. Orphan NQ items (no enclosing test). Treat each as a one-
	// question quiz. (Hand-built fixtures sometimes ship this way.)
	if len(testFiles) == 0 && len(itemFiles) > 0 {
		orphanQuiz := QuizImport{
			Title:    "Imported Questions",
			QuizType: "assignment",
		}
		for i, f := range itemFiles {
			data := itemByHref[f]
			q, warnings, perr := parseNewQuizzesItem(f, data, i)
			result.Warnings = append(result.Warnings, warnings...)
			if perr != nil {
				result.Errors = append(result.Errors, *perr)
				continue
			}
			orphanQuiz.Questions = append(orphanQuiz.Questions, q)
		}
		if len(orphanQuiz.Questions) > 0 {
			result.Quizzes = append(result.Quizzes, orphanQuiz)
			result.Dialect = DialectNewQuizzes
		}
	}

	if result.Dialect == "" {
		result.Dialect = DialectUnknown
	}

	// Promote any stimuli collected during NQ section walks.
	result.Stimuli = append(result.Stimuli, collectedStimuli()...)

	return result, nil
}

// parseClassicAssessmentFile parses one Canvas Classic assessment file
// (which may contain multiple <assessment> elements). Returns the list
// of quizzes plus warnings/errors.
func parseClassicAssessmentFile(filename string, data []byte) ([]QuizImport, []ImportWarning, []ImportError) {
	root, err := parseClassicXML(filename, data)
	if err != nil {
		return nil, nil, []ImportError{{
			Source: filename, Code: "xml_parse_error", Message: err.Error(),
		}}
	}

	warnings := []ImportWarning{}
	errs := []ImportError{}
	quizzes := []QuizImport{}

	for _, a := range root.Assessments {
		q := QuizImport{
			Title:      a.Title,
			QuizType:   "assignment",
			Identifier: a.Ident,
			Published:  false,
		}
		// Some Canvas exports carry quiz_type / time_limit in
		// the assessment's qtimetadata.
		for _, f := range a.Metadata.Fields {
			switch f.Label {
			case "quiz_type":
				if f.Entry != "" {
					q.QuizType = f.Entry
				}
			case "time_limit":
				if v, err := atoiSafe(f.Entry); err == nil && v > 0 {
					q.TimeLimit = &v
				}
			case "shuffle_answers":
				q.ShuffleAnswers = strings.EqualFold(f.Entry, "true")
			case "points_possible":
				if v, err := atofSafe(f.Entry); err == nil {
					q.PointsPossible = &v
				}
			}
		}

		// Flatten sections → items, in order.
		position := 0
		for _, sec := range a.Sections {
			// Bank references: section has a <sourcebank_ref> pointing
			// to a bank id. We emit a placeholder question with
			// BankItemIdentifier set; the service layer resolves it
			// after banks are persisted. NB: Canvas Classic's
			// "pick N from bank" semantics need question groups —
			// we punt on quantity for now (the service can decide).
			if sec.SourceBank != nil && strings.TrimSpace(sec.SourceBank.Ident) != "" {
				q.Questions = append(q.Questions, QuestionImport{
					Position:           position,
					QuestionType:       UnifiedMultipleChoice, // placeholder; service resolves the real type
					QuestionText:       fmt.Sprintf("[Bank reference: %s]", strings.TrimSpace(sec.SourceBank.Ident)),
					BankItemIdentifier: strings.TrimSpace(sec.SourceBank.Ident),
				})
				position++
			}
			for _, item := range sec.Items {
				qi, w, perr := classicItemToQuestion(item, position, filename)
				warnings = append(warnings, w...)
				if perr != nil {
					errs = append(errs, *perr)
					continue
				}
				q.Questions = append(q.Questions, qi)
				position++
			}
		}

		quizzes = append(quizzes, q)
	}

	return quizzes, warnings, errs
}

// parseClassicBank parses one assessment_question_banks/<bank>.xml file.
func parseClassicBank(filename string, data []byte) (*ItemBankImport, []ImportWarning, error) {
	root, err := parseClassicXML(filename, data)
	if err != nil {
		return nil, nil, err
	}
	if root.ObjectBank == nil {
		return nil, []ImportWarning{{
			Source: filename, Code: "no_objectbank",
			Message: "file does not contain an <objectbank> element",
		}}, nil
	}
	ob := root.ObjectBank
	bank := &ItemBankImport{
		Identifier: ob.Ident,
	}
	// Title from qtimetadata if present.
	for _, f := range ob.Metadata.Fields {
		switch f.Label {
		case "bank_title", "title":
			bank.Title = f.Entry
		case "description":
			bank.Description = f.Entry
		}
	}
	if bank.Title == "" {
		bank.Title = "Question Bank " + ob.Ident
	}

	warnings := []ImportWarning{}
	for i, item := range ob.Items {
		// Reuse the item converter; downgrade QuestionImport to BankItemImport.
		qi, w, perr := classicItemToQuestion(item, i, filename)
		warnings = append(warnings, w...)
		if perr != nil {
			warnings = append(warnings, ImportWarning{
				Source: perr.Source, Code: perr.Code, Message: perr.Message,
			})
			continue
		}
		bank.Items = append(bank.Items, BankItemImport{
			Identifier:        item.Ident,
			Position:          i,
			QuestionType:      qi.QuestionType,
			QuestionText:      qi.QuestionText,
			PointsPossible:    qi.PointsPossible,
			Answers:           qi.Answers,
			CorrectComments:   qi.CorrectComments,
			IncorrectComments: qi.IncorrectComments,
			NeutralComments:   qi.NeutralComments,
		})
	}
	return bank, warnings, nil
}

// parseNewQuizzesAssessmentTest parses an <assessmentTest> file and its
// referenced items. Returns one QuizImport.
func parseNewQuizzesAssessmentTest(filename string, data []byte, itemByHref map[string][]byte, b *imsccBundle) (*QuizImport, []ImportWarning, []ImportError) {
	var test nqAssessmentTest
	if err := xmlUnmarshal(data, &test); err != nil {
		return nil, nil, []ImportError{{
			Source: filename, Code: "xml_parse_error", Message: err.Error(),
		}}
	}
	q := &QuizImport{
		Title:      test.Title,
		QuizType:   "assignment",
		Identifier: test.Identifier,
	}
	warnings := []ImportWarning{}
	errs := []ImportError{}
	position := 0

	// Walk sections (incl. nested for stimulus groups).
	var walkSection func(sec nqAssessmentSection, stimulusID string)
	walkSection = func(sec nqAssessmentSection, stimulusID string) {
		// rubricBlock = stimulus content for the wrapping section.
		// Canvas NQ puts the passage in a rubricBlock view="candidate".
		thisStimulus := stimulusID
		if sec.RubricBlock != nil && strings.TrimSpace(sec.RubricBlock.Content) != "" {
			// Create a synthetic identifier so questions can refer
			// to it. Use the section ident.
			thisStimulus = sec.Identifier
		}

		for _, ref := range sec.AssessmentItemRefs {
			// Resolve href — Canvas typically writes "items/<ident>.xml".
			data, _, ok := b.resolvePath(ref.Href)
			if !ok {
				// Try a couple of common variations.
				if d2, ok2 := itemByHref[ref.Href]; ok2 {
					data = d2
					ok = true
				}
			}
			if !ok {
				warnings = append(warnings, ImportWarning{
					Source: ref.Href, Code: "missing_item",
					Message: "assessmentItemRef points to file not in bundle",
				})
				continue
			}
			qi, w, perr := parseNewQuizzesItem(ref.Href, data, position)
			warnings = append(warnings, w...)
			if perr != nil {
				errs = append(errs, *perr)
				continue
			}
			if thisStimulus != "" {
				qi.StimulusIdentifier = thisStimulus
			}
			q.Questions = append(q.Questions, qi)
			position++
		}
		for _, nested := range sec.Sections {
			walkSection(nested, thisStimulus)
		}
	}

	// Collect stimuli during the walk.
	var collectStimuli func(sec nqAssessmentSection)
	collectStimuli = func(sec nqAssessmentSection) {
		if sec.RubricBlock != nil && strings.TrimSpace(sec.RubricBlock.Content) != "" {
			// Emit as a stimulus on the result. We have to bubble it
			// up to the bundle-level result though — defer to the
			// caller via a side channel. The simplest approach: stash
			// in the QuizImport and let importBundle promote them.
			// For now, we don't have a slot on QuizImport, so we use
			// a package-level helper.
			_ = appendStimulus(StimulusImport{
				Identifier: sec.Identifier,
				Title:      sec.Title,
				Content:    wrapTipTap(sec.RubricBlock.Content),
			})
		}
		for _, nested := range sec.Sections {
			collectStimuli(nested)
		}
	}

	for _, tp := range test.TestParts {
		for _, sec := range tp.AssessmentSections {
			collectStimuli(sec)
			walkSection(sec, "")
		}
	}

	return q, warnings, errs
}

// --- stimulus collection via package-level slot ---
//
// We avoid plumbing a *ImportResult through walkSection by using a
// goroutine-unsafe package variable; the importer is sequential so this
// is fine. The variable is cleared at the start of importBundle.
var pendingStimuli []StimulusImport

func appendStimulus(s StimulusImport) error {
	pendingStimuli = append(pendingStimuli, s)
	return nil
}

func resetStimuli() {
	pendingStimuli = nil
}

func collectedStimuli() []StimulusImport {
	out := pendingStimuli
	pendingStimuli = nil
	return out
}

// --- standalone-file paths (used when no .imscc) ---

func importStandaloneClassic(filename string, data []byte) (*ImportResult, error) {
	resetStimuli()
	quizzes, warnings, errs := parseClassicAssessmentFile(filename, data)
	return &ImportResult{
		Quizzes:  quizzes,
		Warnings: warnings,
		Errors:   errs,
		Dialect:  DialectClassic,
	}, nil
}

func importStandaloneNewQuizzes(filename string, data []byte) (*ImportResult, error) {
	resetStimuli()
	q, warnings, perr := parseNewQuizzesItem(filename, data, 0)
	result := &ImportResult{
		Dialect:  DialectNewQuizzes,
		Warnings: warnings,
	}
	if perr != nil {
		result.Errors = append(result.Errors, *perr)
		return result, nil
	}
	quiz := QuizImport{
		Title:     "Imported Question",
		QuizType:  "assignment",
		Questions: []QuestionImport{q},
	}
	result.Quizzes = []QuizImport{quiz}
	return result, nil
}

// wrapTipTap embeds raw HTML in a minimal TipTap document JSON so the
// existing renderer can display it. Real TipTap docs are far richer;
// the renderer treats unknown content types as raw HTML so this is OK
// as a starting point and the user can re-author later.
func wrapTipTap(html string) string {
	// {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"…"}]}]}
	// Simpler: an HTML node, which TipTap supports as raw via the
	// "html" extension we ship in the editor.
	wrapped := map[string]interface{}{
		"type": "doc",
		"content": []map[string]interface{}{
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": strings.TrimSpace(stripHTMLForLabel(html))},
				},
			},
		},
	}
	b, err := json.Marshal(wrapped)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// xmlUnmarshal dispatches to encoding/xml.Unmarshal. Kept as a thin
// wrapper for symmetry with future format switches.
func xmlUnmarshal(data []byte, v interface{}) error {
	return goXMLUnmarshal(data, v)
}

// atoiSafe / atofSafe are noisy-free conversion helpers.
func atoiSafe(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func atofSafe(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}
