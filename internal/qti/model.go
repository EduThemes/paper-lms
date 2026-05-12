// Package qti implements a Canvas-compatible QTI (Question and Test
// Interoperability) importer and exporter for Paper LMS.
//
// The package accepts two flavors of Canvas exports:
//
//  1. Canvas Classic QTI 2.1 — the .imscc bundle that the legacy Canvas
//     Quizzes engine has produced for ~15 years. Uses `<assessment>` /
//     `<section>` / `<item>` elements (the QTI 1.2-ish flavor that Canvas
//     ships under the QTI 2.1 banner — see parser_classic.go for the
//     reconciliation).
//
//  2. Canvas New Quizzes QTI 2.2 — the modern engine. Uses true IMS-QTI
//     2.2 elements: `<assessmentItem>` with `<choiceInteraction>`,
//     `<matchInteraction>`, `<orderInteraction>`, etc.
//
// Both dialects flow through the same intermediate representation defined
// in this file, then materialize as Paper LMS QuizQuestion + QuizItemBank
// + QuizStimulus rows. Exporter writes only Canvas Classic format (the
// most portable target — every Canvas instance accepts it).
//
// No new go.mod dependencies — only stdlib (encoding/xml, archive/zip).
package qti

// Dialect identifies which Canvas QTI flavor a bundle came from. Detected
// by parser.go from the top-level XML element of the first referenced
// assessment file.
type Dialect string

const (
	DialectClassic     Dialect = "classic"     // Canvas Classic QTI 2.1
	DialectNewQuizzes  Dialect = "newquizzes"  // Canvas New Quizzes QTI 2.2
	DialectUnknown     Dialect = "unknown"
)

// ImportWarning is a non-fatal issue raised during parsing. The import
// still proceeds; the caller can decide whether to surface these to the
// user (we always do in the HTTP response).
type ImportWarning struct {
	Source  string `json:"source"`  // file name or item identifier
	Code    string `json:"code"`    // short machine-readable tag, e.g. "unknown_item_type"
	Message string `json:"message"` // human-readable detail
}

// ImportError is a fatal per-item failure. The bundle still imports —
// other items succeed — but this specific item could not be parsed.
type ImportError struct {
	Source  string `json:"source"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ImportResult is returned by Importer.ImportIMSCC / ImportQTIFile.
// Crucially it is in-memory only; no persistence happens here. The
// service layer (internal/service/qti_import_service.go) consumes this
// and writes rows via existing QuizService / QuizItemBankService /
// QuizStimulusService — that way QTI never touches the DB directly and
// stays unit-testable without a Postgres instance.
type ImportResult struct {
	// Quizzes holds top-level quiz definitions parsed from the bundle.
	// Each entry's CourseID is set by the importer from the caller's
	// ImportIMSCC argument. IDs are zero until the service layer
	// persists them.
	Quizzes []QuizImport `json:"quizzes"`

	// ItemBanks holds the Canvas-Classic <objectbank> rows
	// (assessment_question_banks/<id>.xml). NQ does not ship banks as
	// separate files, so this is always empty for NQ inputs.
	ItemBanks []ItemBankImport `json:"item_banks"`

	// Stimuli holds shared passages (NQ stimulus blocks). Classic does
	// not have stimulus passages so this is empty for classic inputs.
	Stimuli []StimulusImport `json:"stimuli"`

	// Dialect records which parser handled the bundle. Useful for the
	// HTTP response and debugging.
	Dialect Dialect `json:"dialect"`

	Warnings []ImportWarning `json:"warnings,omitempty"`
	Errors   []ImportError   `json:"errors,omitempty"`
}

// QuizImport mirrors models.Quiz plus its children. We keep the model
// fields separate from the DB row so parsers can populate this without
// worrying about ID assignment. The service layer copies fields across.
type QuizImport struct {
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	QuizType       string    `json:"quiz_type"`     // "assignment" by default
	TimeLimit      *int      `json:"time_limit"`    // minutes; nil if not specified
	PointsPossible *float64  `json:"points_possible"`
	ShuffleAnswers bool      `json:"shuffle_answers"`
	Published      bool      `json:"published"`
	// Identifier is the source-system identifier (Canvas IdentifierRef).
	// Preserved so the exporter can round-trip it without churning IDs.
	Identifier string         `json:"identifier"`
	Questions  []QuestionImport `json:"questions"`
}

// QuestionImport mirrors models.QuizQuestion. The Answers field is the
// already-shaped JSON string that quiz_service.go's graders consume.
// This is the load-bearing translation: every dialect parser MUST emit
// answers in the shape documented in quiz_service.go (see answerOption
// struct, lines 28-49 of quiz_service.go).
type QuestionImport struct {
	Position          int      `json:"position"`
	QuestionType      string   `json:"question_type"` // Paper LMS unified type
	QuestionText      string   `json:"question_text"`
	PointsPossible    *float64 `json:"points_possible"`
	Answers           string   `json:"answers"` // JSON string in Paper LMS grader format
	CorrectComments   string   `json:"correct_comments"`
	IncorrectComments string   `json:"incorrect_comments"`
	NeutralComments   string   `json:"neutral_comments"`

	// SourceIdentifier preserves the Canvas item identifier so the
	// exporter can round-trip it. Not persisted on QuizQuestion (no
	// column for it) — only used in-memory.
	SourceIdentifier string `json:"source_identifier,omitempty"`

	// BankItemIdentifier is set when this question is a `<assessmentRef>`
	// pointing into an item bank. The service layer resolves this to a
	// concrete BankItemID after the banks are persisted.
	BankItemIdentifier string `json:"bank_item_identifier,omitempty"`

	// StimulusIdentifier is the source-system identifier of the stimulus
	// this question reads from. Service layer resolves to StimulusID.
	StimulusIdentifier string `json:"stimulus_identifier,omitempty"`
}

// ItemBankImport mirrors models.QuizItemBank plus its items.
type ItemBankImport struct {
	Identifier  string             `json:"identifier"` // Canvas bank id
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Items       []BankItemImport   `json:"items"`
}

// BankItemImport mirrors models.QuizItemBankItem. Same shape as
// QuestionImport minus the references — bank items can't reference
// other banks or stimuli.
type BankItemImport struct {
	Identifier        string   `json:"identifier"`
	Position          int      `json:"position"`
	QuestionType      string   `json:"question_type"`
	QuestionText      string   `json:"question_text"`
	PointsPossible    *float64 `json:"points_possible"`
	Answers           string   `json:"answers"`
	CorrectComments   string   `json:"correct_comments"`
	IncorrectComments string   `json:"incorrect_comments"`
	NeutralComments   string   `json:"neutral_comments"`
}

// StimulusImport mirrors models.QuizStimulus. NQ stimuli are TipTap-ready
// HTML; we wrap it in a minimal TipTap doc shell so the existing TipTap
// renderer can display it.
type StimulusImport struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Content    string `json:"content"` // TipTap document JSON
}
