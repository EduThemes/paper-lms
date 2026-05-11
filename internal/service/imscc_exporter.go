package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ExportResult summarizes what was exported into an IMSCC package.
type ExportResult struct {
	ModulesExported     int      `json:"modules_exported"`
	PagesExported       int      `json:"pages_exported"`
	AssignmentsExported int      `json:"assignments_exported"`
	QuizzesExported     int      `json:"quizzes_exported"`
	QuestionsExported   int      `json:"questions_exported"`
	DiscussionsExported int      `json:"discussions_exported"`
	Errors              []string `json:"errors,omitempty"`
}

// IMSCCExporter generates IMSCC (IMS Common Cartridge) zip packages from course content.
type IMSCCExporter struct {
	courseRepo            repository.CourseRepository
	moduleRepo            repository.ModuleRepository
	moduleItemRepo        repository.ModuleItemRepository
	pageRepo              repository.PageRepository
	assignmentRepo        repository.AssignmentRepository
	quizRepo              repository.QuizRepository
	quizQuestionRepo      repository.QuizQuestionRepository
	discussionTopicRepo   repository.DiscussionTopicRepository
	assignmentGroupRepo   repository.AssignmentGroupRepository
	rubricRepo            repository.RubricRepository
	rubricAssocRepo       repository.RubricAssociationRepository
	outcomeGroupRepo      repository.LearningOutcomeGroupRepository
	outcomeRepo           repository.LearningOutcomeRepository
	calendarEventRepo     repository.CalendarEventRepository
	attachmentRepo        repository.AttachmentRepository
	fileService           *FileService
}

// NewIMSCCExporter creates a new IMSCC exporter with all required dependencies.
func NewIMSCCExporter(
	courseRepo repository.CourseRepository,
	moduleRepo repository.ModuleRepository,
	moduleItemRepo repository.ModuleItemRepository,
	pageRepo repository.PageRepository,
	assignmentRepo repository.AssignmentRepository,
	quizRepo repository.QuizRepository,
	quizQuestionRepo repository.QuizQuestionRepository,
	discussionTopicRepo repository.DiscussionTopicRepository,
	assignmentGroupRepo repository.AssignmentGroupRepository,
	rubricRepo repository.RubricRepository,
	rubricAssocRepo repository.RubricAssociationRepository,
	outcomeGroupRepo repository.LearningOutcomeGroupRepository,
	outcomeRepo repository.LearningOutcomeRepository,
	calendarEventRepo repository.CalendarEventRepository,
	attachmentRepo repository.AttachmentRepository,
	fileService *FileService,
) *IMSCCExporter {
	return &IMSCCExporter{
		courseRepo:          courseRepo,
		moduleRepo:          moduleRepo,
		moduleItemRepo:      moduleItemRepo,
		pageRepo:            pageRepo,
		assignmentRepo:      assignmentRepo,
		quizRepo:            quizRepo,
		quizQuestionRepo:    quizQuestionRepo,
		discussionTopicRepo: discussionTopicRepo,
		assignmentGroupRepo: assignmentGroupRepo,
		rubricRepo:          rubricRepo,
		rubricAssocRepo:     rubricAssocRepo,
		outcomeGroupRepo:    outcomeGroupRepo,
		outcomeRepo:         outcomeRepo,
		calendarEventRepo:   calendarEventRepo,
		attachmentRepo:      attachmentRepo,
		fileService:         fileService,
	}
}

// --- Export manifest XML structures (with namespace support) ---

// exportManifest is the top-level imsmanifest.xml element with proper IMS namespaces.
type exportManifest struct {
	XMLName       xml.Name                    `xml:"manifest"`
	Identifier    string                      `xml:"identifier,attr"`
	XMLNS         string                      `xml:"xmlns,attr"`
	XMLNSlom      string                      `xml:"xmlns:lom,attr"`
	XMLNSlomimscc string                      `xml:"xmlns:lomimscc,attr"`
	Metadata      exportManifestMetadata      `xml:"metadata"`
	Organizations exportManifestOrganizations `xml:"organizations"`
	Resources     exportManifestResources     `xml:"resources"`
}

type exportManifestMetadata struct {
	Schema        string `xml:"schema"`
	SchemaVersion string `xml:"schemaversion"`
}

type exportManifestOrganizations struct {
	Organizations []exportManifestOrganization `xml:"organization"`
}

type exportManifestOrganization struct {
	Identifier string               `xml:"identifier,attr"`
	Structure  string               `xml:"structure,attr"`
	Items      []exportManifestItem `xml:"item"`
}

type exportManifestItem struct {
	Identifier    string               `xml:"identifier,attr"`
	IdentifierRef string               `xml:"identifierref,attr,omitempty"`
	Title         string               `xml:"title"`
	Items         []exportManifestItem `xml:"item,omitempty"`
}

type exportManifestResources struct {
	Resources []exportManifestResource `xml:"resource"`
}

type exportManifestResource struct {
	Identifier   string                     `xml:"identifier,attr"`
	Type         string                     `xml:"type,attr"`
	Href         string                     `xml:"href,attr,omitempty"`
	Files        []exportManifestFile       `xml:"file,omitempty"`
	Dependencies []exportManifestDependency `xml:"dependency,omitempty"`
}

type exportManifestFile struct {
	Href string `xml:"href,attr"`
}

type exportManifestDependency struct {
	IdentifierRef string `xml:"identifierref,attr"`
}

// --- Canvas-specific export XML structures ---

// exportAssignment matches the canvasAssignment format from the importer.
type exportAssignment struct {
	XMLName         xml.Name `xml:"assignment"`
	Title           string   `xml:"title"`
	Description     string   `xml:"text"`
	PointsPossible  string   `xml:"points_possible"`
	GradingType     string   `xml:"grading_type"`
	SubmissionTypes string   `xml:"submission_types"`
}

// exportDiscussionTopic matches the canvasDiscussionTopic format from the importer.
type exportDiscussionTopic struct {
	XMLName        xml.Name `xml:"topic"`
	Title          string   `xml:"title"`
	Message        string   `xml:"text"`
	DiscussionType string   `xml:"discussion_type"`
}

// --- QTI 1.2 export XML structures ---

type exportQTIQuestestinterop struct {
	XMLName     xml.Name              `xml:"questestinterop"`
	Assessments []exportQTIAssessment `xml:"assessment"`
}

type exportQTIAssessment struct {
	XMLName  xml.Name           `xml:"assessment"`
	Ident    string             `xml:"ident,attr"`
	Title    string             `xml:"title,attr"`
	MetaData exportQTIMetaData  `xml:"qtimetadata"`
	Sections []exportQTISection `xml:"section"`
}

type exportQTIMetaData struct {
	Fields []exportQTIMetaDataField `xml:"qtimetadatafield"`
}

type exportQTIMetaDataField struct {
	Label string `xml:"fieldlabel"`
	Entry string `xml:"fieldentry"`
}

type exportQTISection struct {
	XMLName xml.Name        `xml:"section"`
	Ident   string          `xml:"ident,attr"`
	Title   string          `xml:"title,attr"`
	Items   []exportQTIItem `xml:"item"`
}

type exportQTIItem struct {
	XMLName            xml.Name                `xml:"item"`
	Ident              string                  `xml:"ident,attr"`
	Title              string                  `xml:"title,attr"`
	MetaData           exportQTIItemMetaData   `xml:"itemmetadata"`
	Presentation       exportQTIPresentation   `xml:"presentation"`
	ResponseProcessing exportQTIResProcessing  `xml:"resprocessing"`
	Feedbacks          []exportQTIItemFeedback `xml:"itemfeedback,omitempty"`
}

type exportQTIItemMetaData struct {
	Inner exportQTIItemMetaDataInner `xml:"qtimetadata"`
}

type exportQTIItemMetaDataInner struct {
	Fields []exportQTIMetaDataField `xml:"qtimetadatafield"`
}

type exportQTIPresentation struct {
	Material    exportQTIMaterial      `xml:"material"`
	Responses   []exportQTIResponse    `xml:"response_lid,omitempty"`
	ResponseStr []exportQTIResponseStr `xml:"response_str,omitempty"`
}

type exportQTIMaterial struct {
	MatText exportQTIMatText `xml:"mattext"`
}

type exportQTIMatText struct {
	TextType string `xml:"texttype,attr"`
	Text     string `xml:",chardata"`
}

type exportQTIResponse struct {
	Ident        string                `xml:"ident,attr"`
	RCardinality string                `xml:"rcardinality,attr"`
	RenderChoice exportQTIRenderChoice `xml:"render_choice"`
}

type exportQTIResponseStr struct {
	Ident        string             `xml:"ident,attr"`
	RCardinality string             `xml:"rcardinality,attr"`
	RenderFib    exportQTIRenderFib `xml:"render_fib"`
}

type exportQTIRenderFib struct {
	Rows    int `xml:"rows,attr"`
	Columns int `xml:"columns,attr"`
}

type exportQTIRenderChoice struct {
	Labels []exportQTIResponseLabel `xml:"response_label"`
}

type exportQTIResponseLabel struct {
	Ident    string            `xml:"ident,attr"`
	Material exportQTIMaterial `xml:"material"`
}

type exportQTIResProcessing struct {
	Outcomes   exportQTIOutcomes       `xml:"outcomes"`
	Conditions []exportQTIResCondition `xml:"respcondition"`
}

type exportQTIOutcomes struct {
	DecVars []exportQTIDecVar `xml:"decvar"`
}

type exportQTIDecVar struct {
	MaxValue string `xml:"maxvalue,attr"`
	MinValue string `xml:"minvalue,attr"`
	VarName  string `xml:"varname,attr"`
	VarType  string `xml:"vartype,attr"`
}

type exportQTIResCondition struct {
	Continue        string                     `xml:"continue,attr"`
	ConditionVar    exportQTIConditionVar      `xml:"conditionvar"`
	SetVars         []exportQTISetVar          `xml:"setvar"`
	DisplayFeedback []exportQTIDisplayFeedback `xml:"displayfeedback,omitempty"`
}

type exportQTIConditionVar struct {
	VarEqual []exportQTIVarEqual `xml:"varequal,omitempty"`
	Other    *exportQTIOther     `xml:"other,omitempty"`
}

type exportQTIOther struct {
	XMLName xml.Name `xml:"other"`
}

type exportQTIVarEqual struct {
	RespIdent string `xml:"respident,attr"`
	Value     string `xml:",chardata"`
}

type exportQTISetVar struct {
	VarName string `xml:"varname,attr"`
	Action  string `xml:"action,attr"`
	Value   string `xml:",chardata"`
}

type exportQTIDisplayFeedback struct {
	FeedbackType string `xml:"feedbacktype,attr"`
	LinkRefID    string `xml:"linkrefid,attr"`
}

type exportQTIItemFeedback struct {
	Ident   string                    `xml:"ident,attr"`
	FlowMat exportQTIItemFeedbackFlow `xml:"flow_mat"`
}

type exportQTIItemFeedbackFlow struct {
	Material exportQTIMaterial `xml:"material"`
}

// ExportCourse generates an IMSCC package for the given course and returns the zip file path.
func (e *IMSCCExporter) ExportCourse(ctx context.Context, courseID uint, outputDir string) (string, *ExportResult, error) {
	// Verify the course exists
	course, err := e.courseRepo.FindByID(ctx, courseID)
	if err != nil {
		return "", nil, fmt.Errorf("course %d not found: %w", courseID, err)
	}

	result := &ExportResult{}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create zip file
	courseName := strings.ReplaceAll(course.Name, " ", "_")
	zipFileName := fmt.Sprintf("course_%d_%s.imscc", courseID, courseName)
	zipPath := filepath.Join(outputDir, zipFileName)

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Collect all resources and organization items
	var resources []exportManifestResource
	var orgItems []exportManifestItem

	// Use large pagination to get all content
	largePage := repository.PaginationParams{Page: 1, PerPage: 10000}

	// Export attachments first so the token emitter can build a complete
	// file-id → web_resources/<path> map before any HTML body is written.
	filePathByID, _ := e.writeAttachments(ctx, zipWriter, courseID, result)

	// Pre-list pages to populate the slug set the emitter validates against.
	// (Listing twice is cheap; the actual write happens below.)
	pageList, _ := e.pageRepo.ListByCourseID(ctx, courseID, largePage)
	pageSlugs := make(map[string]bool)
	if pageList != nil {
		for _, p := range pageList.Items {
			pageSlugs[p.URL] = true
		}
	}

	emitterCtx := &tokenEmitterCtx{
		courseID:            courseID,
		filePathByID:        filePathByID,
		pageSlugs:           pageSlugs,
		migrationIDByEntity: map[string]string{},
	}

	// --- Export Modules (organization structure) ---
	modules, err := e.moduleRepo.ListByCourseID(ctx, courseID, largePage)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list modules: %v", err))
	} else {
		for _, mod := range modules.Items {
			moduleItem := exportManifestItem{
				Identifier: fmt.Sprintf("module_%d", mod.ID),
				Title:      mod.Name,
			}

			// Get module items
			items, itemErr := e.moduleItemRepo.ListByModuleID(ctx, mod.ID, largePage)
			if itemErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to list items for module %d: %v", mod.ID, itemErr))
				continue
			}

			for _, item := range items.Items {
				if item.ContentType == "ContextModuleSubHeader" {
					// Sub-headers are items with no resource reference
					subItem := exportManifestItem{
						Identifier: fmt.Sprintf("item_subheader_%d", item.ID),
						Title:      item.Title,
					}
					moduleItem.Items = append(moduleItem.Items, subItem)
					continue
				}

				resID := exportResourceID(item.ContentType, item.ContentID)
				subItem := exportManifestItem{
					Identifier:    fmt.Sprintf("item_%d", item.ID),
					IdentifierRef: resID,
					Title:         item.Title,
				}
				moduleItem.Items = append(moduleItem.Items, subItem)
			}

			orgItems = append(orgItems, moduleItem)
			result.ModulesExported++
		}
	}

	// --- Export Wiki Pages (we already pre-listed them above for the emitter ctx) ---
	if pageList != nil {
		for _, page := range pageList.Items {
			resID := fmt.Sprintf("res_page_%d", page.ID)
			href := fmt.Sprintf("wiki_content/%s.html", page.URL)

			// Wrap the body in a minimal HTML document so the importer's
			// extractBodyHTML pulls the body cleanly back out, and rewrite
			// internal URLs back to Canvas placeholder tokens.
			body := emitTokens(page.Body, emitterCtx)
			doc := buildWikiHTMLDocument(page.Title, body)

			w, writeErr := zipWriter.Create(href)
			if writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create zip entry for page %q: %v", page.Title, writeErr))
				continue
			}
			if _, writeErr = w.Write([]byte(doc)); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to write page %q content: %v", page.Title, writeErr))
				continue
			}

			resources = append(resources, exportManifestResource{
				Identifier: resID,
				Type:       "webcontent",
				Href:       href,
				Files: []exportManifestFile{
					{Href: href},
				},
			})
			result.PagesExported++
		}
	}

	// --- Export Assignments ---
	assignments, err := e.assignmentRepo.ListByCourseID(ctx, courseID, largePage)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list assignments: %v", err))
	} else {
		for _, assignment := range assignments.Items {
			resID := fmt.Sprintf("res_assignment_%d", assignment.ID)
			href := fmt.Sprintf("assignments/%d.xml", assignment.ID)

			pointsStr := ""
			if assignment.PointsPossible != nil {
				pointsStr = strconv.FormatFloat(*assignment.PointsPossible, 'f', -1, 64)
			}

			assignXML := exportAssignment{
				Title:           assignment.Name,
				Description:     emitTokens(assignment.Description, emitterCtx),
				PointsPossible:  pointsStr,
				GradingType:     assignment.GradingType,
				SubmissionTypes: assignment.SubmissionTypes,
			}

			xmlData, marshalErr := xml.MarshalIndent(assignXML, "", "  ")
			if marshalErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to marshal assignment %q: %v", assignment.Name, marshalErr))
				continue
			}

			w, writeErr := zipWriter.Create(href)
			if writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create zip entry for assignment %q: %v", assignment.Name, writeErr))
				continue
			}
			xmlContent := append([]byte(xml.Header), xmlData...)
			if _, writeErr = w.Write(xmlContent); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to write assignment %q: %v", assignment.Name, writeErr))
				continue
			}

			resources = append(resources, exportManifestResource{
				Identifier: resID,
				Type:       "assignment_xmlv1p0",
				Href:       href,
				Files: []exportManifestFile{
					{Href: href},
				},
			})
			result.AssignmentsExported++
		}
	}

	// --- Export Quizzes (each in its own folder with QTI + assessment_meta) ---
	quizzes, err := e.quizRepo.ListByCourseID(ctx, courseID, largePage)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list quizzes: %v", err))
	} else {
		for _, quiz := range quizzes.Items {
			resID := fmt.Sprintf("res_quiz_%d", quiz.ID)
			folder := fmt.Sprintf("quizzes/%d", quiz.ID)
			qtiHref := fmt.Sprintf("%s/assessment_qti.xml", folder)
			metaHref := fmt.Sprintf("%s/assessment_meta.xml", folder)

			qtiData, qtiErr := e.buildQTIForQuiz(ctx, quiz.ID, quiz.Title, emitTokens(quiz.Description, emitterCtx), quiz.QuizType, quiz.TimeLimit, result)
			if qtiErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to build QTI for quiz %q: %v", quiz.Title, qtiErr))
				continue
			}
			if err := writeRawFile(zipWriter, qtiHref, qtiData); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to write quiz %q: %v", quiz.Title, err))
				continue
			}
			if metaXML, err := quizAssessmentMetaXML(quiz); err == nil {
				_ = writeRawFile(zipWriter, metaHref, append([]byte(xml.Header), metaXML...))
			}

			resources = append(resources, exportManifestResource{
				Identifier: resID,
				Type:       "imsqti_xmlv1p2/imscc_xmlv1p3/assessment",
				Href:       qtiHref,
				Files: []exportManifestFile{
					{Href: qtiHref},
					{Href: metaHref},
				},
			})
			result.QuizzesExported++
		}
	}

	// --- Export Discussion Topics ---
	discussions, err := e.discussionTopicRepo.ListByCourseID(ctx, courseID, largePage)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list discussions: %v", err))
	} else {
		for _, disc := range discussions.Items {
			resID := fmt.Sprintf("res_discussion_%d", disc.ID)
			href := fmt.Sprintf("discussions/%d.xml", disc.ID)

			discXML := exportDiscussionTopic{
				Title:          disc.Title,
				Message:        emitTokens(disc.Message, emitterCtx),
				DiscussionType: disc.DiscussionType,
			}

			xmlData, marshalErr := xml.MarshalIndent(discXML, "", "  ")
			if marshalErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to marshal discussion %q: %v", disc.Title, marshalErr))
				continue
			}

			w, writeErr := zipWriter.Create(href)
			if writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create zip entry for discussion %q: %v", disc.Title, writeErr))
				continue
			}
			xmlContent := append([]byte(xml.Header), xmlData...)
			if _, writeErr = w.Write(xmlContent); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to write discussion %q: %v", disc.Title, writeErr))
				continue
			}

			resources = append(resources, exportManifestResource{
				Identifier: resID,
				Type:       "imsdt_xmlv1p3",
				Href:       href,
				Files: []exportManifestFile{
					{Href: href},
				},
			})
			result.DiscussionsExported++
		}
	}

	// --- Export course-level metadata (mirrors what the importer reads). ---
	// writeAttachments already ran above so the token emitter could see the
	// file map; the rest of the metadata is order-independent.
	_ = e.writeCourseSettingsXML(zipWriter, course)
	_ = e.writeSyllabusHTML(zipWriter, course)
	_ = e.writeCanvasExportTxt(zipWriter)
	_ = e.writeModuleMetaXML(ctx, zipWriter, courseID, result)
	_ = e.writeAssignmentGroupsXML(ctx, zipWriter, courseID, result)
	_ = e.writeRubricsXML(ctx, zipWriter, courseID, result)
	_ = e.writeLearningOutcomesXML(ctx, zipWriter, courseID, result)
	_ = e.writeEventsXML(ctx, zipWriter, courseID, result)

	// --- Build and write imsmanifest.xml ---
	manifest := exportManifest{
		Identifier:    fmt.Sprintf("course_export_%d", courseID),
		XMLNS:         "http://www.imsglobal.org/xsd/imsccv1p3/imscp_v1p1",
		XMLNSlom:      "http://ltsc.ieee.org/xsd/imsccv1p3/LOM/resource",
		XMLNSlomimscc: "http://ltsc.ieee.org/xsd/imsccv1p3/LOM/manifest",
		Metadata: exportManifestMetadata{
			Schema:        "IMS Common Cartridge",
			SchemaVersion: "1.3.0",
		},
		Organizations: exportManifestOrganizations{
			Organizations: []exportManifestOrganization{
				{
					Identifier: "org_1",
					Structure:  "rooted-hierarchy",
					Items:      orgItems,
				},
			},
		},
		Resources: exportManifestResources{
			Resources: resources,
		},
	}

	manifestXML, err := xml.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", result, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestWriter, err := zipWriter.Create("imsmanifest.xml")
	if err != nil {
		return "", result, fmt.Errorf("failed to create manifest entry: %w", err)
	}
	manifestContent := append([]byte(xml.Header), manifestXML...)
	if _, err = manifestWriter.Write(manifestContent); err != nil {
		return "", result, fmt.Errorf("failed to write manifest: %w", err)
	}

	return zipPath, result, nil
}

// exportResourceID returns the manifest resource identifier for a content tag's content type and ID.
func exportResourceID(contentType string, contentID *uint) string {
	if contentID == nil {
		return ""
	}
	id := *contentID
	switch contentType {
	case "WikiPage":
		return fmt.Sprintf("res_page_%d", id)
	case "Assignment":
		return fmt.Sprintf("res_assignment_%d", id)
	case "Quiz":
		return fmt.Sprintf("res_quiz_%d", id)
	case "DiscussionTopic":
		return fmt.Sprintf("res_discussion_%d", id)
	default:
		return fmt.Sprintf("res_%s_%d", strings.ToLower(contentType), id)
	}
}

// buildQTIForQuiz generates QTI 1.2 XML for a quiz and its questions.
func (e *IMSCCExporter) buildQTIForQuiz(
	ctx context.Context,
	quizID uint,
	title string,
	description string,
	quizType string,
	timeLimit *int,
	result *ExportResult,
) ([]byte, error) {
	largePage := repository.PaginationParams{Page: 1, PerPage: 10000}
	questions, err := e.quizQuestionRepo.ListByQuizID(ctx, quizID, largePage)
	if err != nil {
		return nil, fmt.Errorf("failed to list questions: %w", err)
	}

	// Build metadata fields
	metaFields := []exportQTIMetaDataField{
		{Label: "quiz_type", Entry: quizType},
	}
	if timeLimit != nil {
		metaFields = append(metaFields, exportQTIMetaDataField{
			Label: "qmd_timelimit",
			Entry: strconv.Itoa(*timeLimit),
		})
	}

	// Build QTI items from questions
	var qtiItems []exportQTIItem
	for _, q := range questions.Items {
		qtiItem := buildQTIItemFromQuestion(q)
		qtiItems = append(qtiItems, qtiItem)
		result.QuestionsExported++
	}

	assessment := exportQTIAssessment{
		Ident:    fmt.Sprintf("quiz_%d", quizID),
		Title:    title,
		MetaData: exportQTIMetaData{Fields: metaFields},
		Sections: []exportQTISection{
			{
				Ident: fmt.Sprintf("quiz_%d_section", quizID),
				Title: "Default Section",
				Items: qtiItems,
			},
		},
	}

	interop := exportQTIQuestestinterop{
		Assessments: []exportQTIAssessment{assessment},
	}

	xmlData, err := xml.MarshalIndent(interop, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal QTI: %w", err)
	}

	return append([]byte(xml.Header), xmlData...), nil
}

// buildQTIItemFromQuestion converts a models.QuizQuestion into a QTI 1.2 item struct.
func buildQTIItemFromQuestion(q models.QuizQuestion) exportQTIItem {
	ident := fmt.Sprintf("question_%d", q.ID)

	// Item-level metadata
	itemMetaFields := []exportQTIMetaDataField{
		{Label: "question_type", Entry: q.QuestionType},
	}
	if q.PointsPossible != nil {
		itemMetaFields = append(itemMetaFields, exportQTIMetaDataField{
			Label: "points_possible",
			Entry: strconv.FormatFloat(*q.PointsPossible, 'f', -1, 64),
		})
	}

	// Determine max value for outcomes
	maxValue := "1"
	if q.PointsPossible != nil {
		maxValue = strconv.FormatFloat(*q.PointsPossible, 'f', -1, 64)
	}

	item := exportQTIItem{
		Ident: ident,
		Title: fmt.Sprintf("Question %d", q.Position),
		MetaData: exportQTIItemMetaData{
			Inner: exportQTIItemMetaDataInner{
				Fields: itemMetaFields,
			},
		},
		Presentation: exportQTIPresentation{
			Material: exportQTIMaterial{
				MatText: exportQTIMatText{
					TextType: "text/html",
					Text:     q.QuestionText,
				},
			},
		},
		ResponseProcessing: exportQTIResProcessing{
			Outcomes: exportQTIOutcomes{
				DecVars: []exportQTIDecVar{
					{
						MaxValue: maxValue,
						MinValue: "0",
						VarName:  "SCORE",
						VarType:  "Decimal",
					},
				},
			},
		},
	}

	// Parse answers JSON
	var answers []answerChoice
	if q.Answers != "" && q.Answers != "null" {
		_ = json.Unmarshal([]byte(q.Answers), &answers)
	}

	// Build presentation and response processing based on question type
	switch q.QuestionType {
	case "multiple_choice", "true_false":
		item.Presentation.Responses = buildExportChoiceResponse(ident, answers)
		item.ResponseProcessing.Conditions = buildExportChoiceConditions(ident, answers, maxValue)
	case "short_answer", "fill_in_multiple_blanks", "numerical_question":
		item.Presentation.ResponseStr = []exportQTIResponseStr{
			{
				Ident:        "response_" + ident,
				RCardinality: "Single",
				RenderFib: exportQTIRenderFib{
					Rows:    1,
					Columns: 40,
				},
			},
		}
		item.ResponseProcessing.Conditions = buildExportShortAnswerConditions(ident, answers, maxValue)
	case "essay":
		item.Presentation.ResponseStr = []exportQTIResponseStr{
			{
				Ident:        "response_" + ident,
				RCardinality: "Single",
				RenderFib: exportQTIRenderFib{
					Rows:    10,
					Columns: 60,
				},
			},
		}
		// Essays have no auto-grading conditions
	}

	// Add feedback if present
	if q.CorrectComments != "" || q.IncorrectComments != "" {
		item.Feedbacks = buildExportFeedback(q.CorrectComments, q.IncorrectComments)
	}

	return item
}

// buildExportChoiceResponse builds response_lid elements for multiple choice / true-false questions.
func buildExportChoiceResponse(itemIdent string, answers []answerChoice) []exportQTIResponse {
	var labels []exportQTIResponseLabel
	for _, ans := range answers {
		labels = append(labels, exportQTIResponseLabel{
			Ident: ans.ID,
			Material: exportQTIMaterial{
				MatText: exportQTIMatText{
					TextType: "text/html",
					Text:     ans.Text,
				},
			},
		})
	}

	return []exportQTIResponse{
		{
			Ident:        "response_" + itemIdent,
			RCardinality: "Single",
			RenderChoice: exportQTIRenderChoice{
				Labels: labels,
			},
		},
	}
}

// buildExportChoiceConditions builds respconditions for choice-based questions.
func buildExportChoiceConditions(itemIdent string, answers []answerChoice, maxValue string) []exportQTIResCondition {
	var conditions []exportQTIResCondition

	for _, ans := range answers {
		if ans.Weight > 0 {
			conditions = append(conditions, exportQTIResCondition{
				Continue: "No",
				ConditionVar: exportQTIConditionVar{
					VarEqual: []exportQTIVarEqual{
						{
							RespIdent: "response_" + itemIdent,
							Value:     ans.ID,
						},
					},
				},
				SetVars: []exportQTISetVar{
					{
						VarName: "SCORE",
						Action:  "Set",
						Value:   maxValue,
					},
				},
			})
		}
	}

	// Add default "other" condition (incorrect)
	conditions = append(conditions, exportQTIResCondition{
		Continue: "No",
		ConditionVar: exportQTIConditionVar{
			Other: &exportQTIOther{},
		},
		SetVars: []exportQTISetVar{
			{
				VarName: "SCORE",
				Action:  "Set",
				Value:   "0",
			},
		},
	})

	return conditions
}

// buildExportShortAnswerConditions builds respconditions for short-answer / numerical questions.
func buildExportShortAnswerConditions(itemIdent string, answers []answerChoice, maxValue string) []exportQTIResCondition {
	var conditions []exportQTIResCondition

	for _, ans := range answers {
		if ans.Weight > 0 {
			conditions = append(conditions, exportQTIResCondition{
				Continue: "No",
				ConditionVar: exportQTIConditionVar{
					VarEqual: []exportQTIVarEqual{
						{
							RespIdent: "response_" + itemIdent,
							Value:     ans.Text,
						},
					},
				},
				SetVars: []exportQTISetVar{
					{
						VarName: "SCORE",
						Action:  "Set",
						Value:   maxValue,
					},
				},
			})
		}
	}

	// Default incorrect condition
	conditions = append(conditions, exportQTIResCondition{
		Continue: "No",
		ConditionVar: exportQTIConditionVar{
			Other: &exportQTIOther{},
		},
		SetVars: []exportQTISetVar{
			{
				VarName: "SCORE",
				Action:  "Set",
				Value:   "0",
			},
		},
	})

	return conditions
}

// buildExportFeedback builds itemfeedback elements for correct and incorrect feedback.
func buildExportFeedback(correctComments, incorrectComments string) []exportQTIItemFeedback {
	var feedbacks []exportQTIItemFeedback

	if correctComments != "" {
		feedbacks = append(feedbacks, exportQTIItemFeedback{
			Ident: "correct_fb",
			FlowMat: exportQTIItemFeedbackFlow{
				Material: exportQTIMaterial{
					MatText: exportQTIMatText{
						TextType: "text/html",
						Text:     correctComments,
					},
				},
			},
		})
	}

	if incorrectComments != "" {
		feedbacks = append(feedbacks, exportQTIItemFeedback{
			Ident: "incorrect_fb",
			FlowMat: exportQTIItemFeedbackFlow{
				Material: exportQTIMaterial{
					MatText: exportQTIMatText{
						TextType: "text/html",
						Text:     incorrectComments,
					},
				},
			},
		})
	}

	return feedbacks
}
