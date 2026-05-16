package service

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	repoPostgres "github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// --- Manifest XML structures ---

// Manifest represents the top-level imsmanifest.xml element.
type Manifest struct {
	XMLName       xml.Name              `xml:"manifest"`
	Identifier    string                `xml:"identifier,attr"`
	Organizations ManifestOrganizations `xml:"organizations"`
	Resources     ManifestResources     `xml:"resources"`
}

// ManifestOrganizations wraps the list of organizations.
type ManifestOrganizations struct {
	Organizations []ManifestOrganization `xml:"organization"`
}

// ManifestOrganization represents a single organization (module structure) in the manifest.
type ManifestOrganization struct {
	Identifier string         `xml:"identifier,attr"`
	Structure  string         `xml:"structure,attr"`
	Items      []ManifestItem `xml:"item"`
}

// ManifestItem represents a single item within an organization. Items can be nested (modules contain items).
type ManifestItem struct {
	Identifier    string         `xml:"identifier,attr"`
	IdentifierRef string         `xml:"identifierref,attr"`
	Title         string         `xml:"title"`
	Items         []ManifestItem `xml:"item"`
}

// ManifestResources wraps the list of resources.
type ManifestResources struct {
	Resources []ManifestResource `xml:"resource"`
}

// ManifestResource represents a single resource entry in the manifest.
type ManifestResource struct {
	Identifier string              `xml:"identifier,attr"`
	Type       string              `xml:"type,attr"`
	Href       string              `xml:"href,attr"`
	Files      []ManifestFile      `xml:"file"`
	Dependencies []ManifestDependency `xml:"dependency"`
}

// ManifestFile represents a file reference within a resource.
type ManifestFile struct {
	Href string `xml:"href,attr"`
}

// ManifestDependency represents a dependency of one resource on another.
type ManifestDependency struct {
	IdentifierRef string `xml:"identifierref,attr"`
}

// --- Canvas-specific XML structures ---

// canvasDiscussionTopic represents a Canvas-exported discussion topic XML.
//
// Announcements in Canvas are a degenerate discussion: the same XML shape,
// distinguished by either a <type>announcement</type> child or a
// <discussion_type>announcement</discussion_type> on the topic. We capture
// both so the importer can route into the announcements table.
type canvasDiscussionTopic struct {
	XMLName        xml.Name `xml:"topic"`
	Title          string   `xml:"title"`
	Message        string   `xml:"text"`
	DiscussionType string   `xml:"discussion_type"`
	Type           string   `xml:"type"`
	Pinned         string   `xml:"pinned"`
	PostedAt       string   `xml:"posted_at"`
	DelayedPostAt  string   `xml:"delayed_post_at"`
	WorkflowState  string   `xml:"workflow_state"`
	AllowRating    string   `xml:"allow_rating"`
}

// canvasAssignment represents a Canvas-exported assignment XML.
type canvasAssignment struct {
	XMLName            xml.Name `xml:"assignment"`
	Identifier         string   `xml:"identifier,attr"`
	Title              string   `xml:"title"`
	Description        string   `xml:"text"`
	PointsPossible     string   `xml:"points_possible"`
	GradingType        string   `xml:"grading_type"`
	SubmissionTypes    string   `xml:"submission_types"`
	DueAt              string   `xml:"due_at"`
	UnlockAt           string   `xml:"unlock_at"`
	LockAt             string   `xml:"lock_at"`
	Position           string   `xml:"position"`
	WorkflowState      string   `xml:"workflow_state"`
	AssignmentGroupRef string   `xml:"assignment_group_identifierref"`
	AnonymousGrading   string   `xml:"anonymous_grading"`
	PeerReviews        string   `xml:"peer_reviews"`
	PeerReviewCount    string   `xml:"peer_review_count"`
}

// canvasAssignmentGroups represents course_settings/assignment_groups.xml.
type canvasAssignmentGroups struct {
	XMLName xml.Name                `xml:"assignmentGroups"`
	Groups  []canvasAssignmentGroup `xml:"assignmentGroup"`
}

type canvasAssignmentGroup struct {
	Identifier  string `xml:"identifier,attr"`
	Title       string `xml:"title"`
	Position    string `xml:"position"`
	GroupWeight string `xml:"group_weight"`
	Rules       string `xml:"rules"`
}

// canvasWebLink represents a web link (imswl) resource.
type canvasWebLink struct {
	XMLName xml.Name `xml:"webLink"`
	Title   string   `xml:"title"`
	URL     struct {
		Href string `xml:"href,attr"`
	} `xml:"url"`
}

// canvasQuizMeta represents the per-quiz assessment_meta.xml file Canvas
// emits alongside each QTI assessment. It carries the settings that don't
// fit into the standard QTI envelope (allowed attempts, scoring policy,
// shuffle, lock/unlock, quiz_type).
type canvasQuizMeta struct {
	XMLName            xml.Name `xml:"quiz"`
	Identifier         string   `xml:"identifier,attr"`
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
	LockAt             string   `xml:"lock_at"`
	UnlockAt           string   `xml:"unlock_at"`
	DueAt              string   `xml:"due_at"`
	Available          string   `xml:"available"`
	Assignment         struct {
		AssignmentGroupRef string `xml:"assignment_group_identifierref"`
		DueAt              string `xml:"due_at"`
		LockAt             string `xml:"lock_at"`
		UnlockAt           string `xml:"unlock_at"`
		PointsPossible     string `xml:"points_possible"`
	} `xml:"assignment"`
}

// --- Import result ---

// EntityRef identifies a single domain row created during an import. Used by
// CleanupFailedImport to soft-delete everything written by a partial run.
type EntityRef struct {
	Type string // "WikiPage" | "Assignment" | "Quiz" | "QuizQuestion" |
	//             "DiscussionTopic" | "ContextModule" | "ContentTag" | "Attachment"
	ID uint
}

// CCModuleSettings holds the per-module fields read from
// course_settings/module_meta.xml.
type CCModuleSettings struct {
	WorkflowState             string
	RequireSequentialProgress bool
	HasRequireSeq             bool // true if the source XML actually specified the field
}

// CCItemSettings holds the per-item fields read from module_meta.xml.
type CCItemSettings struct {
	Indent        int
	NewTab        bool
	WorkflowState string
}

// ImportResult summarizes what was imported from an IMSCC package.
//
// The lower-cased fields hold internal working state that flows between the
// pre-passes and pass-2 (token rewriting). They are intentionally not exported
// to JSON.
type ImportResult struct {
	ModulesCreated     int      `json:"modules_created"`
	PagesCreated       int      `json:"pages_created"`
	AssignmentsCreated int      `json:"assignments_created"`
	QuizzesCreated     int      `json:"quizzes_created"`
	QuestionsCreated   int      `json:"questions_created"`
	DiscussionsCreated int      `json:"discussions_created"`
	ModuleItemsCreated int      `json:"module_items_created"`
	FilesCreated       int      `json:"files_created"`
	Errors             []string `json:"errors,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`

	// Tracks every successful Create() call so a failed import can be rolled
	// back via CleanupFailedImport.
	CreatedEntities []EntityRef `json:"-"`

	// pre-1 / pre-2 working state, populated before pass-1.
	fileURLByPath map[string]string         `json:"-"` // zip path → public file URL
	moduleMeta    map[string]CCModuleSettings `json:"-"` // resource Identifier → module meta
	itemMeta      map[string]CCItemSettings   `json:"-"` // resource IdentifierRef → item meta

	// Populated during pass-1 so pass-2 can resolve $WIKI_REFERENCE$ /
	// $CANVAS_OBJECT_REFERENCE$ links.
	entityByMigID map[string]EntityRef `json:"-"`
	pageBySlug    map[string]uint      `json:"-"`

	// Lets importWebContent map a non-HTML href to the Attachment row created
	// in pre-2 so it can build a File-type module item instead of a page.
	attachmentByPath map[string]uint `json:"-"`

	// Resolves <assignment_group_identifierref> in assignment / quiz XML to
	// the AssignmentGroup row created during pre-1 from
	// course_settings/assignment_groups.xml.
	assignmentGroupByMigID map[string]uint `json:"-"`

	// Rubric → assignment associations queued during pre-1 (rubrics.xml is
	// read before assignments are created in pass-1). Drained after pass-1
	// in applyPendingRubricAssocs.
	pendingRubricAssocs []pendingRubricAssoc `json:"-"`

	// Authenticated user driving the import. Used to attribute Attachments
	// and CalendarEvents so they aren't always pinned to user 1.
	ownerUserID uint `json:"-"`
}

// --- Parser ---

// IMSCCParser parses IMSCC/Common Cartridge zip files and imports content into a course.
type IMSCCParser struct {
	courseRepo            repository.CourseRepository
	moduleRepo            repository.ModuleRepository
	moduleItemRepo        repository.ModuleItemRepository
	pageRepo              repository.PageRepository
	assignmentRepo        repository.AssignmentRepository
	quizRepo              repository.QuizRepository
	quizQuestionRepo      repository.QuizQuestionRepository
	fileService           *FileService
	folderRepo            repository.FolderRepository
	discussionTopicRepo   repository.DiscussionTopicRepository
	questionBankRepo      repository.QuestionBankRepository
	questionBankEntryRepo repository.QuestionBankEntryRepository
	assignmentGroupRepo   repository.AssignmentGroupRepository
	announcementRepo      repoPostgres.AnnouncementRepository
	rubricRepo            repository.RubricRepository
	rubricAssocRepo       repository.RubricAssociationRepository
	outcomeGroupRepo      repository.LearningOutcomeGroupRepository
	outcomeRepo           repository.LearningOutcomeRepository
	calendarEventRepo     repository.CalendarEventRepository
}

// NewIMSCCParser creates a new IMSCC parser with all required dependencies.
func NewIMSCCParser(
	courseRepo repository.CourseRepository,
	moduleRepo repository.ModuleRepository,
	moduleItemRepo repository.ModuleItemRepository,
	pageRepo repository.PageRepository,
	assignmentRepo repository.AssignmentRepository,
	quizRepo repository.QuizRepository,
	quizQuestionRepo repository.QuizQuestionRepository,
	fileService *FileService,
	folderRepo repository.FolderRepository,
	discussionTopicRepo repository.DiscussionTopicRepository,
	questionBankRepo repository.QuestionBankRepository,
	questionBankEntryRepo repository.QuestionBankEntryRepository,
	assignmentGroupRepo repository.AssignmentGroupRepository,
	announcementRepo repoPostgres.AnnouncementRepository,
	rubricRepo repository.RubricRepository,
	rubricAssocRepo repository.RubricAssociationRepository,
	outcomeGroupRepo repository.LearningOutcomeGroupRepository,
	outcomeRepo repository.LearningOutcomeRepository,
	calendarEventRepo repository.CalendarEventRepository,
) *IMSCCParser {
	return &IMSCCParser{
		courseRepo:            courseRepo,
		moduleRepo:            moduleRepo,
		moduleItemRepo:        moduleItemRepo,
		pageRepo:              pageRepo,
		assignmentRepo:        assignmentRepo,
		quizRepo:              quizRepo,
		quizQuestionRepo:      quizQuestionRepo,
		fileService:           fileService,
		folderRepo:            folderRepo,
		discussionTopicRepo:   discussionTopicRepo,
		questionBankRepo:      questionBankRepo,
		questionBankEntryRepo: questionBankEntryRepo,
		assignmentGroupRepo:   assignmentGroupRepo,
		announcementRepo:      announcementRepo,
		rubricRepo:            rubricRepo,
		rubricAssocRepo:       rubricAssocRepo,
		outcomeGroupRepo:      outcomeGroupRepo,
		outcomeRepo:           outcomeRepo,
		calendarEventRepo:     calendarEventRepo,
	}
}

// ParsePackage is the main entry point. It opens the zip, reads the manifest, and imports all content.
//
// The flow is:
//   pre-1: open zip, parse manifest, parse course_settings/module_meta.xml.
//   pre-2: extract every binary/asset file (web_resources/**, etc.) into Attachment
//          rows via FileService.UploadFile and build a path→URL map.
//   pass-1: existing per-resource importers create modules, pages, assignments,
//           quizzes, discussions, questions; each successful create appends to
//           result.CreatedEntities and registers in entityByMigID/pageBySlug.
//   pass-2: walk created entities and rewrite Canvas placeholder tokens
//           ($IMS-CC-FILEBASE$, $WIKI_REFERENCE$, $CANVAS_OBJECT_REFERENCE$)
//           in their HTML bodies using the maps built in pre-2 and pass-1.
//
// Callers should run CleanupFailedImport(ctx, courseID, result.CreatedEntities)
// when a non-nil error is returned (or when result.Errors is non-empty and a
// rollback is desired).
//
// userID is the authenticated user driving the import. It's used to
// attribute every Attachment / CalendarEvent created during the run so we
// don't dump them onto the seed admin (UserID=1).
func (p *IMSCCParser) ParsePackage(ctx context.Context, courseID uint, userID uint, zipPath string) (*ImportResult, error) {
	// Verify the course exists
	_, err := p.courseRepo.FindByID(ctx, courseID, 0)
	if err != nil {
		return nil, fmt.Errorf("course %d not found: %w", courseID, err)
	}

	// Open zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Build a lookup map of zip file entries by name
	zipFiles := make(map[string]*zip.File)
	for _, f := range reader.File {
		zipFiles[f.Name] = f
	}

	// Read and parse imsmanifest.xml
	manifestFile, ok := zipFiles["imsmanifest.xml"]
	if !ok {
		return nil, fmt.Errorf("imsmanifest.xml not found in package")
	}

	manifest, err := parseManifest(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Build resource lookup by identifier
	resourceMap := make(map[string]ManifestResource)
	for _, res := range manifest.Resources.Resources {
		resourceMap[res.Identifier] = res
	}

	result := &ImportResult{
		fileURLByPath:           map[string]string{},
		entityByMigID:           map[string]EntityRef{},
		pageBySlug:              map[string]uint{},
		attachmentByPath:        map[string]uint{},
		assignmentGroupByMigID:  map[string]uint{},
		ownerUserID:             userID,
	}

	// pre-1: read course-wide metadata files. Each is independent and
	// optional — a missing file is normal for trimmed exports.
	result.moduleMeta, result.itemMeta = parseModuleMeta(zipFiles, result)
	p.parseAssignmentGroups(ctx, courseID, zipFiles, result)
	p.parseCourseSettingsXML(ctx, courseID, zipFiles, result)
	p.parseLearningOutcomesXML(ctx, courseID, zipFiles, result)
	p.parseRubricsXML(ctx, courseID, zipFiles, result)
	p.parseEventsXML(ctx, courseID, zipFiles, result)

	// pre-2: extract every binary/asset file in the zip into Attachment rows
	// regardless of whether the manifest references it — Canvas exports list
	// web_resources/foo.pdf as a webcontent resource, but the file is still
	// a binary asset, not a wiki page. importWebContent below detects the
	// non-HTML href and creates a File-type module item instead of a page.
	if err := p.extractFiles(ctx, courseID, zipFiles, nil, result); err != nil {
		return result, fmt.Errorf("file extraction failed: %w", err)
	}

	// pass-1: process organizations (modules) — existing logic, now with
	// entity tracking + module_meta application.
	for _, org := range manifest.Organizations.Organizations {
		p.processOrganization(ctx, courseID, org, resourceMap, zipFiles, result)
	}

	// pass-1 (cont'd): import orphan resources (not referenced by any module).
	referencedResources := collectReferencedResources(manifest)
	for _, res := range manifest.Resources.Resources {
		if _, referenced := referencedResources[res.Identifier]; referenced {
			continue
		}
		p.importResource(ctx, courseID, res, zipFiles, nil, 0, result)
	}

	// post pass-1: drain rubric→assignment associations now that the
	// referenced assignments exist in entityByMigID.
	p.applyPendingRubricAssocs(ctx, result)

	// pass-2: rewrite tokens in all created entities' HTML bodies. A hard
	// failure here means the import would land half-rewritten — surface it
	// to the caller so CleanupFailedImport can roll back.
	if err := p.rewriteAllBodies(ctx, courseID, result); err != nil {
		return result, fmt.Errorf("token rewrite failed: %w", err)
	}

	return result, nil
}

func parseManifest(f *zip.File) (*Manifest, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := xml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("XML unmarshal error: %w", err)
	}

	return &manifest, nil
}

// collectReferencedResources returns a set of resource identifiers that are referenced by organization items.
func collectReferencedResources(manifest *Manifest) map[string]bool {
	refs := make(map[string]bool)
	for _, org := range manifest.Organizations.Organizations {
		collectItemRefs(org.Items, refs)
	}
	return refs
}

func collectItemRefs(items []ManifestItem, refs map[string]bool) {
	for _, item := range items {
		if item.IdentifierRef != "" {
			refs[item.IdentifierRef] = true
		}
		collectItemRefs(item.Items, refs)
	}
}

// hasLeafChild returns true if the item has at least one direct child with
// an identifierref (a leaf resource link). Used to decide whether the item
// is a module or a wrapper.
func hasLeafChild(item ManifestItem) bool {
	for _, c := range item.Items {
		if c.IdentifierRef != "" {
			return true
		}
	}
	return false
}

// hasAnyLeafDescendant returns true if the item or any of its descendants
// have an identifierref. Used to skip empty wrappers entirely.
func hasAnyLeafDescendant(item ManifestItem) bool {
	if item.IdentifierRef != "" {
		return true
	}
	for _, c := range item.Items {
		if hasAnyLeafDescendant(c) {
			return true
		}
	}
	return false
}

// flattenWrappers descends into "wrapper" items (no identifierref + only
// container children) and returns the level at which actual modules live.
//
// Quantitown's manifest is `org → wrapper(d0) → 13 unit-sections(d1) → leaves(d2)`,
// so flattenWrappers([d0]) returns the 13 d1 items, and each becomes a module.
// A simpler `org → module(d0) → leaves` cartridge returns [d0] unchanged.
func flattenWrappers(items []ManifestItem) []ManifestItem {
	var out []ManifestItem
	for _, item := range items {
		if !hasAnyLeafDescendant(item) {
			continue // empty wrapper
		}
		// If this item has any direct leaf child, it's a module.
		if item.IdentifierRef != "" || hasLeafChild(item) {
			out = append(out, item)
			continue
		}
		// Otherwise it's a wrapper — descend.
		out = append(out, flattenWrappers(item.Items)...)
	}
	return out
}

func (p *IMSCCParser) processOrganization(
	ctx context.Context,
	courseID uint,
	org ManifestOrganization,
	resourceMap map[string]ManifestResource,
	zipFiles map[string]*zip.File,
	result *ImportResult,
) {
	// Walk past wrapper items so that for nested cartridges (e.g. Canvas
	// exports with an outer "course root" item) each unit becomes its own
	// top-level module instead of a single mega-module containing everything.
	moduleItems := flattenWrappers(org.Items)
	for modulePos, topItem := range moduleItems {
		moduleName := topItem.Title
		if moduleName == "" {
			moduleName = fmt.Sprintf("Module %d", modulePos+1)
		}

		module := &models.ContextModule{
			CourseID:      courseID,
			Name:          moduleName,
			Position:      modulePos + 1,
			WorkflowState: "active",
		}
		// Apply module_meta.xml settings if present.
		if ms, ok := result.moduleMeta[topItem.Identifier]; ok {
			if ms.HasRequireSeq {
				module.RequireSequentialProgress = ms.RequireSequentialProgress
			}
			if ms.WorkflowState == "unpublished" {
				module.WorkflowState = "unpublished"
			}
		}

		if err := p.moduleRepo.Create(ctx, module); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to create module %q: %v", moduleName, err))
			continue
		}
		result.ModulesCreated++
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "ContextModule", ID: module.ID})
		if topItem.Identifier != "" {
			result.entityByMigID[topItem.Identifier] = EntityRef{Type: "ContextModule", ID: module.ID}
		}

		// If the top-level item itself references a resource (no sub-items), treat it as both module and item
		if topItem.IdentifierRef != "" && len(topItem.Items) == 0 {
			res, ok := resourceMap[topItem.IdentifierRef]
			if ok {
				p.importResource(ctx, courseID, res, zipFiles, module, 1, result)
			}
			continue
		}

		// Process sub-items as module items
		for itemPos, subItem := range topItem.Items {
			if subItem.IdentifierRef == "" {
				// Sub-header (no resource reference)
				tag := &models.ContentTag{
					ContextModuleID: module.ID,
					ContentType:     "ContextModuleSubHeader",
					Title:           subItem.Title,
					Position:        itemPos + 1,
					WorkflowState:   "active",
				}
				// module_meta keys items by IdentifierRef preferred, then Identifier.
				if is, ok := result.itemMeta[subItem.Identifier]; ok {
					tag.Indent = is.Indent
					tag.NewTab = is.NewTab
					if is.WorkflowState == "unpublished" {
						tag.WorkflowState = "unpublished"
					}
				}
				if err := p.moduleItemRepo.Create(ctx, tag); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to create sub-header %q: %v", subItem.Title, err))
				} else {
					result.ModuleItemsCreated++
					result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "ContentTag", ID: tag.ID})
				}
				continue
			}

			res, ok := resourceMap[subItem.IdentifierRef]
			if !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("resource %q referenced by item %q not found", subItem.IdentifierRef, subItem.Title))
				continue
			}

			p.importResource(ctx, courseID, res, zipFiles, module, itemPos+1, result)
		}
	}
}

// importResource imports a single manifest resource and optionally creates a module item for it.
func (p *IMSCCParser) importResource(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	zipFiles map[string]*zip.File,
	module *models.ContextModule,
	position int,
	result *ImportResult,
) {
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}
	resType := normalizeResourceTypeWithHref(res.Type, href)

	// Dedup: if the same resource was already imported (e.g. referenced from
	// a different module's items list), don't create a second copy. We still
	// surface the resource in this module by emitting a fresh ContentTag
	// pointing at the existing entity.
	if res.Identifier != "" {
		if existing, ok := result.entityByMigID[res.Identifier]; ok {
			if module != nil && resType != "assessment_sidecar" {
				title := titleFromHref(href)
				if title == "" {
					title = res.Identifier
				}
				eid := existing.ID
				p.createModuleItem(ctx, module.ID, existing.Type, &eid, title, position, res.Identifier, result)
			}
			return
		}
	}

	switch resType {
	case "webcontent":
		p.importWebContent(ctx, courseID, res, zipFiles, module, position, result)
	case "discussion":
		p.importDiscussion(ctx, courseID, res, zipFiles, module, position, result)
	case "assignment":
		p.importAssignment(ctx, courseID, res, zipFiles, module, position, result)
	case "quiz":
		p.importQuiz(ctx, courseID, res, zipFiles, module, position, result)
	case "question_bank":
		p.importQuestionBank(ctx, courseID, res, zipFiles, result)
	case "weblink":
		p.importWebLink(ctx, courseID, res, zipFiles, module, position, result)
	case "basic_lti":
		// LTI tools require a developer key for the not-null FK on
		// ContextExternalTool — the import flow doesn't have a clean way to
		// mint one, so we surface a warning and skip the DB write. The
		// resource is parsed enough to log title + launch URL so admins
		// know what to wire up manually.
		p.warnSkippedLTI(zipFiles, res, result)
	case "assessment_sidecar":
		// Standalone learning-application-resource pointing at assessment_meta.xml.
		// The quiz importer reads the same file from the quiz folder, so the
		// sidecar entry is redundant and would otherwise fail an assignment parse.
		return
	case "learning_application":
		p.importAssignment(ctx, courseID, res, zipFiles, module, position, result)
	default:
		result.Warnings = append(result.Warnings, fmt.Sprintf("unsupported resource type %q for resource %q", res.Type, res.Identifier))
	}
}

// normalizeResourceType maps IMSCC resource type strings to simplified categories.
//
// Some categories also depend on the resource's href: a
// "learning-application-resource" whose href ends in "assessment_meta.xml"
// is a sidecar for the quiz next door, not its own importable item, so we
// classify it specially so importResource can drop it cleanly.
func normalizeResourceType(t string) string {
	return normalizeResourceTypeWithHref(t, "")
}

func normalizeResourceTypeWithHref(t, href string) string {
	t = strings.ToLower(t)
	href = strings.ToLower(href)

	switch {
	case strings.Contains(t, "imsdt_xmlv1p") || strings.Contains(t, "imsdt_v1p"):
		return "discussion"
	case strings.Contains(t, "imsqti_xmlv") || strings.Contains(t, "imsqti_item_xmlv"):
		return "quiz"
	case strings.Contains(t, "questionbank") || strings.Contains(t, "question_bank") || strings.Contains(t, "question-bank"):
		return "question_bank"
	case t == "webcontent" || strings.Contains(t, "webcontent"):
		return "webcontent"
	case strings.Contains(t, "assignment_xmlv1p") || strings.Contains(t, "canvas_assignment"):
		return "assignment"
	case strings.Contains(t, "imswl_xmlv1p"):
		return "weblink"
	case strings.Contains(t, "imsbasiclti_xmlv") || strings.Contains(t, "imsbasiclti"):
		return "basic_lti"
	case strings.Contains(t, "learning-application-resource"):
		if strings.HasSuffix(href, "assessment_meta.xml") {
			return "assessment_sidecar"
		}
		return "learning_application"
	default:
		return t
	}
}

// --- Resource importers ---

func (p *IMSCCParser) importWebContent(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	zipFiles map[string]*zip.File,
	module *models.ContextModule,
	position int,
	result *ImportResult,
) {
	// Read the HTML file from the zip
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}
	if href == "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("webcontent resource %q has no file reference", res.Identifier))
		return
	}

	// Canvas exports tag binary files (PDFs, images, etc.) as webcontent
	// resources too. Those were already uploaded as Attachments by extractFiles
	// — link a File-type module item to the existing attachment instead of
	// shoving binary bytes into a wiki page body.
	if !isHTMLFile(href) {
		if attID, ok := result.attachmentByPath[strings.TrimPrefix(href, "/")]; ok {
			if res.Identifier != "" {
				result.entityByMigID[res.Identifier] = EntityRef{Type: "Attachment", ID: attID}
			}
			if module != nil {
				title := titleFromHref(href)
				if title == "" {
					title = res.Identifier
				}
				p.createModuleItem(ctx, module.ID, "Attachment", &attID, title, position, res.Identifier, result)
			}
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("file resource %q (%s) has no extracted attachment", res.Identifier, href))
		}
		return
	}

	body, err := readZipFile(zipFiles, href)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read webcontent file %q: %v", href, err))
		return
	}

	// Strip the outer <html><head>...</head><body>...</body></html> wrapper
	// that Canvas exports include, so meta/identifier/workflow_state tags
	// don't render as visible noise at the top of every page.
	pageHTML := extractBodyHTML(string(body))

	// Prefer the document's <title> over the slugified filename — Canvas
	// preserves the human-authored title with proper punctuation, capitals,
	// emoji, etc.
	title := extractDocumentTitle(string(body))
	if title == "" {
		title = titleFromHref(href)
	}

	// Create wiki page
	pageURL := slugify(title)
	page := &models.WikiPage{
		CourseID:      courseID,
		Title:         title,
		URL:           pageURL,
		Body:          pageHTML,
		WorkflowState: "active",
	}

	if err := p.pageRepo.Create(ctx, page); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create page %q: %v", title, err))
		return
	}
	result.PagesCreated++
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "WikiPage", ID: page.ID})
	if res.Identifier != "" {
		result.entityByMigID[res.Identifier] = EntityRef{Type: "WikiPage", ID: page.ID}
	}
	if pageURL != "" {
		result.pageBySlug[pageURL] = page.ID
	}

	// Create module item if in a module
	if module != nil {
		p.createModuleItem(ctx, module.ID, "WikiPage", &page.ID, title, position, res.Identifier, result)
	}
}

func (p *IMSCCParser) importDiscussion(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	zipFiles map[string]*zip.File,
	module *models.ContextModule,
	position int,
	result *ImportResult,
) {
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}

	title := titleFromHref(href)
	message := ""
	var topic canvasDiscussionTopic

	if href != "" {
		data, err := readZipFile(zipFiles, href)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read discussion file %q: %v", href, err))
		} else {
			if xmlErr := xml.Unmarshal(data, &topic); xmlErr == nil {
				if topic.Title != "" {
					title = topic.Title
				}
				message = topic.Message
			} else {
				// Treat as plain HTML content
				message = string(data)
			}
		}
	}

	if title == "" {
		title = res.Identifier
	}

	if isAnnouncementTopic(topic, result.itemMeta[res.Identifier]) {
		p.importAnnouncementFromTopic(ctx, courseID, res, topic, title, message, module, position, result)
		return
	}

	disc := &models.DiscussionTopic{
		CourseID:       courseID,
		UserID:         0, // Will be set by caller or default
		Title:          title,
		Message:        message,
		DiscussionType: "side_comment",
		WorkflowState:  "active",
	}
	if topic.DiscussionType != "" && topic.DiscussionType != "announcement" {
		disc.DiscussionType = topic.DiscussionType
	}
	if topic.WorkflowState == "unpublished" || topic.WorkflowState == "deleted" {
		disc.WorkflowState = topic.WorkflowState
	}

	if err := p.discussionTopicRepo.Create(ctx, disc); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create discussion %q: %v", title, err))
		return
	}
	result.DiscussionsCreated++
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "DiscussionTopic", ID: disc.ID})
	if res.Identifier != "" {
		result.entityByMigID[res.Identifier] = EntityRef{Type: "DiscussionTopic", ID: disc.ID}
	}

	if module != nil {
		p.createModuleItem(ctx, module.ID, "DiscussionTopic", &disc.ID, title, position, res.Identifier, result)
	}
}

// isAnnouncementTopic returns true when the discussion XML or its module-meta
// item flags the topic as an announcement. Canvas writes either
// <type>announcement</type> or <discussion_type>announcement</discussion_type>;
// some exports also tag the module item itself.
func isAnnouncementTopic(t canvasDiscussionTopic, item CCItemSettings) bool {
	if strings.EqualFold(strings.TrimSpace(t.Type), "announcement") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(t.DiscussionType), "announcement") {
		return true
	}
	_ = item
	return false
}

// importAnnouncementFromTopic creates an Announcement row and a corresponding
// ContentTag (announcements ride in modules with ContentType="Announcement"
// in Canvas; we keep the tag pointing at the announcement so the module
// listing stays intact).
func (p *IMSCCParser) importAnnouncementFromTopic(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	topic canvasDiscussionTopic,
	title, message string,
	module *models.ContextModule,
	position int,
	result *ImportResult,
) {
	if p.announcementRepo == nil {
		// Fall back to a plain discussion if announcements aren't wired.
		result.Warnings = append(result.Warnings, fmt.Sprintf("announcement %q imported as discussion (announcement repo unavailable)", title))
		return
	}
	cid := courseID
	a := &models.Announcement{
		CourseID:      &cid,
		Title:         title,
		Message:       message,
		Priority:      "normal",
		WorkflowState: "active",
		PostedAt:      parseCanvasTime(topic.PostedAt),
		DelayedPostAt: parseCanvasTime(topic.DelayedPostAt),
	}
	if topic.WorkflowState == "unpublished" {
		a.WorkflowState = "draft"
	}
	if err := p.announcementRepo.Create(ctx, a); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create announcement %q: %v", title, err))
		return
	}
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "Announcement", ID: a.ID})
	if res.Identifier != "" {
		result.entityByMigID[res.Identifier] = EntityRef{Type: "Announcement", ID: a.ID}
	}
	if module != nil {
		p.createModuleItem(ctx, module.ID, "Announcement", &a.ID, title, position, res.Identifier, result)
	}
}

func (p *IMSCCParser) importAssignment(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	zipFiles map[string]*zip.File,
	module *models.ContextModule,
	position int,
	result *ImportResult,
) {
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}

	title := titleFromHref(href)
	description := ""
	gradingType := "points"
	submissionTypes := "online_text_entry"
	var pointsPossible *float64
	var ca canvasAssignment

	if href != "" {
		data, err := readZipFile(zipFiles, href)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read assignment file %q: %v", href, err))
		} else {
			if xmlErr := xml.Unmarshal(data, &ca); xmlErr == nil {
				if ca.Title != "" {
					title = ca.Title
				}
				description = ca.Description
				if ca.GradingType != "" {
					gradingType = ca.GradingType
				}
				if ca.SubmissionTypes != "" {
					submissionTypes = ca.SubmissionTypes
				}
				if ca.PointsPossible != "" {
					if pts, parseErr := parseFloatStr(ca.PointsPossible); parseErr == nil {
						pointsPossible = &pts
					}
				}
			} else {
				// Treat as HTML description
				description = string(data)
			}
		}
	}

	if title == "" {
		title = res.Identifier
	}

	assignment := &models.Assignment{
		CourseID:        courseID,
		Name:            title,
		Description:     description,
		PointsPossible:  pointsPossible,
		GradingType:     gradingType,
		SubmissionTypes: submissionTypes,
		WorkflowState:   "unpublished",
		DueAt:           parseCanvasTime(ca.DueAt),
		UnlockAt:        parseCanvasTime(ca.UnlockAt),
		LockAt:          parseCanvasTime(ca.LockAt),
	}
	if ca.WorkflowState == "published" {
		assignment.WorkflowState = "published"
		assignment.Published = true
	}
	if ca.Position != "" {
		if pos, err := strconv.Atoi(ca.Position); err == nil {
			assignment.Position = pos
		}
	}
	if ca.AssignmentGroupRef != "" {
		if gid, ok := result.assignmentGroupByMigID[ca.AssignmentGroupRef]; ok {
			assignment.AssignmentGroupID = &gid
		}
	}

	if err := p.assignmentRepo.Create(ctx, assignment); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create assignment %q: %v", title, err))
		return
	}
	result.AssignmentsCreated++
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "Assignment", ID: assignment.ID})
	if res.Identifier != "" {
		result.entityByMigID[res.Identifier] = EntityRef{Type: "Assignment", ID: assignment.ID}
	}

	if module != nil {
		p.createModuleItem(ctx, module.ID, "Assignment", &assignment.ID, title, position, res.Identifier, result)
	}
}

func (p *IMSCCParser) importQuiz(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	zipFiles map[string]*zip.File,
	module *models.ContextModule,
	position int,
	result *ImportResult,
) {
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}
	if href == "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("quiz resource %q has no file reference", res.Identifier))
		return
	}

	data, err := readZipFile(zipFiles, href)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read quiz file %q: %v", href, err))
		return
	}

	qtiResult, err := ParseQTIAssessment(data)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to parse QTI for resource %q: %v", res.Identifier, err))
		return
	}

	// Canvas reuses the imsqti_xml resource type for question banks. The QTI
	// envelope marks them with cc_profile=cc.itembank.v0p1 (or quiz_type=
	// "assessment_question_bank"). Route those to the bank importer instead
	// of materializing a phantom Quiz.
	if qtiResult.IsQuestionBank {
		p.importQuestionBank(ctx, courseID, res, zipFiles, result)
		return
	}

	title := qtiResult.Title
	if title == "" {
		title = titleFromHref(href)
	}
	if title == "" {
		title = res.Identifier
	}

	pointsPossible := qtiResult.PointsPossible

	quiz := &models.Quiz{
		CourseID:        courseID,
		Title:           title,
		Description:     qtiResult.Description,
		QuizType:        qtiResult.QuizType,
		TimeLimit:       qtiResult.TimeLimit,
		AllowedAttempts: 1,
		PointsPossible:  &pointsPossible,
		ScoringPolicy:   "keep_highest",
		ShowCorrectAnswers: true,
		WorkflowState:   "unpublished",
	}

	// assessment_meta.xml lives next to the QTI file (same resource folder)
	// and overrides everything we can derive from the QTI body itself.
	if meta := readQuizMeta(zipFiles, href); meta != nil {
		applyQuizMeta(quiz, meta)
	}

	if err := p.quizRepo.Create(ctx, quiz); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create quiz %q: %v", title, err))
		return
	}
	result.QuizzesCreated++
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "Quiz", ID: quiz.ID})
	if res.Identifier != "" {
		result.entityByMigID[res.Identifier] = EntityRef{Type: "Quiz", ID: quiz.ID}
	}

	// Create quiz questions
	for i := range qtiResult.Questions {
		q := &qtiResult.Questions[i]
		q.QuizID = quiz.ID
		if err := p.quizQuestionRepo.Create(ctx, q); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to create question %d for quiz %q: %v", q.Position, title, err))
		} else {
			result.QuestionsCreated++
			result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "QuizQuestion", ID: q.ID})
		}
	}

	if module != nil {
		p.createModuleItem(ctx, module.ID, "Quiz", &quiz.ID, title, position, res.Identifier, result)
	}
}

func (p *IMSCCParser) importWebLink(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	zipFiles map[string]*zip.File,
	module *models.ContextModule,
	position int,
	result *ImportResult,
) {
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}

	title := titleFromHref(href)
	linkURL := ""

	if href != "" {
		data, err := readZipFile(zipFiles, href)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read weblink file %q: %v", href, err))
		} else {
			var wl canvasWebLink
			if xmlErr := xml.Unmarshal(data, &wl); xmlErr == nil {
				if wl.Title != "" {
					title = wl.Title
				}
				linkURL = wl.URL.Href
			}
		}
	}

	if title == "" {
		title = res.Identifier
	}

	// External weblinks are first-class module items (ContentTag with
	// ContentType="ExternalUrl") in Canvas, not wiki pages. Materializing
	// them as pages dropped new-tab + url metadata and inflated the page
	// count. We only emit a ContentTag if the resource lives inside a
	// module — orphan weblinks (rare) get a warning.
	if module == nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("web link %q has no parent module; skipping", title))
		return
	}

	tag := &models.ContentTag{
		ContextModuleID: module.ID,
		ContentType:     "ExternalUrl",
		Title:           title,
		Position:        position,
		URL:             linkURL,
		WorkflowState:   "active",
	}
	if res.Identifier != "" {
		if is, ok := result.itemMeta[res.Identifier]; ok {
			tag.Indent = is.Indent
			tag.NewTab = is.NewTab
			if is.WorkflowState == "unpublished" {
				tag.WorkflowState = "unpublished"
			}
		}
	}
	if err := p.moduleItemRepo.Create(ctx, tag); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create external-url item %q: %v", title, err))
		return
	}
	result.ModuleItemsCreated++
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "ContentTag", ID: tag.ID})
	if res.Identifier != "" {
		result.entityByMigID[res.Identifier] = EntityRef{Type: "ContentTag", ID: tag.ID}
	}
}

// --- Helpers ---

func (p *IMSCCParser) createModuleItem(
	ctx context.Context,
	moduleID uint,
	contentType string,
	contentID *uint,
	title string,
	position int,
	migrationID string,
	result *ImportResult,
) {
	tag := &models.ContentTag{
		ContextModuleID: moduleID,
		ContentType:     contentType,
		ContentID:       contentID,
		Title:           title,
		Position:        position,
		WorkflowState:   "active",
	}
	if migrationID != "" {
		if is, ok := result.itemMeta[migrationID]; ok {
			tag.Indent = is.Indent
			tag.NewTab = is.NewTab
			if is.WorkflowState == "unpublished" {
				tag.WorkflowState = "unpublished"
			}
		}
	}

	if err := p.moduleItemRepo.Create(ctx, tag); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create module item %q: %v", title, err))
		return
	}
	result.ModuleItemsCreated++
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "ContentTag", ID: tag.ID})
}

// readZipFile reads the contents of a file inside the zip by its path.
func readZipFile(zipFiles map[string]*zip.File, filePath string) ([]byte, error) {
	f, ok := zipFiles[filePath]
	if !ok {
		// Try with/without leading slash
		alt := strings.TrimPrefix(filePath, "/")
		f, ok = zipFiles[alt]
		if !ok {
			return nil, fmt.Errorf("file %q not found in zip", filePath)
		}
	}

	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("could not open %q: %w", filePath, err)
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// titleFromHref extracts a human-readable title from a file path.
func titleFromHref(href string) string {
	if href == "" {
		return ""
	}
	base := path.Base(href)
	// Remove extension
	ext := path.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	// Replace underscores and hyphens with spaces
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	return strings.TrimSpace(base)
}

// parseFloatStr is a helper to parse a float string, returning an error on failure.
func parseFloatStr(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// --- module_meta.xml ---

type ccModuleMetaXML struct {
	XMLName xml.Name        `xml:"modules"`
	Modules []ccModuleXML   `xml:"module"`
}

type ccModuleXML struct {
	Identifier                string             `xml:"identifier,attr"`
	Title                     string             `xml:"title"`
	WorkflowState             string             `xml:"workflow_state"`
	RequireSequentialProgress string             `xml:"require_sequential_progress"`
	Items                     ccModuleItemsXML   `xml:"items"`
}

type ccModuleItemsXML struct {
	Items []ccModuleItemXML `xml:"item"`
}

type ccModuleItemXML struct {
	Identifier    string `xml:"identifier,attr"`
	IdentifierRef string `xml:"identifierref"`
	Indent        string `xml:"indent"`
	NewTab        string `xml:"new_tab"`
	WorkflowState string `xml:"workflow_state"`
}

// parseModuleMeta reads course_settings/module_meta.xml if present and returns
// per-module / per-item settings keyed by their resource identifiers.
//
// Modules are keyed by their organization-item identifier (matches what
// processOrganization sees as topItem.Identifier). Items are keyed by their
// IdentifierRef where set, falling back to Identifier — so importResource can
// look the meta up using the resource ref it already has in hand.
func parseModuleMeta(zipFiles map[string]*zip.File, result *ImportResult) (
	map[string]CCModuleSettings, map[string]CCItemSettings,
) {
	moduleMeta := map[string]CCModuleSettings{}
	itemMeta := map[string]CCItemSettings{}

	f, ok := zipFiles["course_settings/module_meta.xml"]
	if !ok {
		return moduleMeta, itemMeta
	}

	rc, err := f.Open()
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not open module_meta.xml: %v", err))
		return moduleMeta, itemMeta
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not read module_meta.xml: %v", err))
		return moduleMeta, itemMeta
	}

	var meta ccModuleMetaXML
	if err := xml.Unmarshal(data, &meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse module_meta.xml: %v", err))
		return moduleMeta, itemMeta
	}

	for _, m := range meta.Modules {
		ms := CCModuleSettings{WorkflowState: strings.TrimSpace(m.WorkflowState)}
		if v := strings.TrimSpace(m.RequireSequentialProgress); v != "" {
			ms.HasRequireSeq = true
			ms.RequireSequentialProgress = parseBoolFlag(v)
		}
		if m.Identifier != "" {
			moduleMeta[m.Identifier] = ms
		}

		for _, it := range m.Items.Items {
			is := CCItemSettings{
				WorkflowState: strings.TrimSpace(it.WorkflowState),
				NewTab:        parseBoolFlag(it.NewTab),
			}
			if v := strings.TrimSpace(it.Indent); v != "" {
				if n, perr := strconv.Atoi(v); perr == nil {
					is.Indent = n
				}
			}
			key := it.IdentifierRef
			if key == "" {
				key = it.Identifier
			}
			if key != "" {
				itemMeta[key] = is
			}
		}
	}
	return moduleMeta, itemMeta
}

func parseBoolFlag(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

// --- file extraction ---

// collectReferencedHrefs returns the set of hrefs the manifest's resources
// already point at. extractFiles uses this to skip XML/HTML payloads that
// other importers will consume.
func collectReferencedHrefs(manifest *Manifest) map[string]bool {
	hrefs := map[string]bool{}
	for _, res := range manifest.Resources.Resources {
		if res.Href != "" {
			hrefs[res.Href] = true
		}
		for _, fr := range res.Files {
			if fr.Href != "" {
				hrefs[fr.Href] = true
			}
		}
	}
	return hrefs
}

// extractFiles walks the zip and uploads every binary/asset entry into
// Attachment rows, populating result.fileURLByPath. HTML/XML files are
// skipped because they're consumed by the resource importers (pages,
// quizzes, discussions, etc.) — uploading them as duplicate attachments
// would clutter the course's file list.
//
// Failures on a single file are recorded as warnings, not errors — one bad
// asset shouldn't sink the whole import.
//
// `referencedHrefs` is currently unused but kept on the signature in case a
// future caller wants to pass an explicit allow/deny list.
func (p *IMSCCParser) extractFiles(
	ctx context.Context,
	courseID uint,
	zipFiles map[string]*zip.File,
	_ map[string]bool,
	result *ImportResult,
) error {
	folder, err := p.fileService.GetOrCreateRootFolder(ctx, "Course", courseID)
	if err != nil {
		return fmt.Errorf("could not get root folder: %w", err)
	}

	for name, f := range zipFiles {
		if strings.HasSuffix(name, "/") || f.UncompressedSize64 == 0 {
			continue // directory entry
		}
		if name == "imsmanifest.xml" || strings.HasPrefix(name, "course_settings/") {
			continue
		}
		// HTML, XML, and QTI files are consumed by other importers (pages,
		// quizzes, discussions, etc.); don't re-upload them as plain files
		// — they'd land in the course Files list as opaque
		// application/octet-stream rows.
		lower := strings.ToLower(name)
		if isHTMLFile(name) ||
			strings.HasSuffix(lower, ".xml") ||
			strings.HasSuffix(lower, ".qti") ||
			strings.HasSuffix(lower, ".qti.xml") {
			continue
		}

		base := path.Base(name)
		size := int64(f.UncompressedSize64)
		folderID := folder.ID

		uid := result.ownerUserID
		if uid == 0 {
			uid = 1 // legacy fallback when no caller plumbed an owner
		}
		att := &models.Attachment{
			ContextType: "Course",
			ContextID:   courseID,
			FolderID:    &folderID,
			UserID:      uid,
			DisplayName: base,
			Filename:    base,
			ContentType: detectMIMEByExt(base),
			Size:        size,
		}

		rc, openErr := f.Open()
		if openErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not open %q: %v", name, openErr))
			continue
		}

		uploadErr := p.fileService.UploadFileTrusted(ctx, att, rc)
		_ = rc.Close()
		if uploadErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not upload %q: %v", name, uploadErr))
			continue
		}

		// Public URL matches the existing files-download route.
		url := fmt.Sprintf("/api/v1/files/%d/download", att.ID)
		result.fileURLByPath[name] = url
		result.fileURLByPath[strings.TrimPrefix(name, "/")] = url
		result.attachmentByPath[name] = att.ID
		result.attachmentByPath[strings.TrimPrefix(name, "/")] = att.ID
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "Attachment", ID: att.ID})
		result.FilesCreated++
	}
	return nil
}

// isHTMLFile returns true for paths ending in .html or .htm. Used to keep
// HTML wiki pages out of the file-extraction pre-pass and to keep binary
// assets out of importWebContent.
func isHTMLFile(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".html") || strings.HasSuffix(n, ".htm")
}

// reHTMLBody captures the innerHTML of <body>...</body> from a Canvas-exported
// wiki page. The cartridge's HTML files are full documents with <html>, <head>,
// and several <meta name="..."/> tags that we don't want to render. The match
// is permissive about attributes on the <body> tag and case.
var reHTMLBody = regexp.MustCompile(`(?is)<body[^>]*>(.*?)</body>`)

// reHTMLTitle captures the text of the document's <title> element. Canvas
// exports preserve the human-authored page title there (e.g. "1.1 | The
// Power of Precision – N.Q.1") whereas the wiki_content/* filename is a
// slugified, lossy version.
var reHTMLTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// extractBodyHTML returns the innerHTML of <body> if the document has one,
// otherwise the original input (already a fragment).
func extractBodyHTML(html string) string {
	m := reHTMLBody.FindStringSubmatch(html)
	if len(m) < 2 {
		return html
	}
	return strings.TrimSpace(m[1])
}

// extractDocumentTitle returns the text of the first <title> element with
// HTML entities decoded. Returns "" if no title is found or it's empty.
func extractDocumentTitle(html string) string {
	m := reHTMLTitle.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	t := strings.TrimSpace(m[1])
	// Decode common HTML entities the cartridge emits in titles.
	t = strings.ReplaceAll(t, "&amp;", "&")
	t = strings.ReplaceAll(t, "&lt;", "<")
	t = strings.ReplaceAll(t, "&gt;", ">")
	t = strings.ReplaceAll(t, "&quot;", "\"")
	t = strings.ReplaceAll(t, "&#39;", "'")
	t = strings.ReplaceAll(t, "&nbsp;", " ")
	return t
}

// detectMIMEByExt is a small fallback content-type guesser for common course
// asset extensions. The full validator lives in FileService.
func detectMIMEByExt(name string) string {
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".zip":
		return "application/zip"
	}
	return "application/octet-stream"
}

// --- pass-2: rewrite tokens in created entities ---

// rewriteAllBodies walks every created entity, resolves Canvas placeholder
// tokens in its HTML body, and writes the result back. Returns a non-nil
// error when at least one update failed for reasons other than context
// cancellation — the caller (ParsePackage) treats that as a hard failure
// so the migration rolls back rather than landing half-rewritten.
func (p *IMSCCParser) rewriteAllBodies(ctx context.Context, courseID uint, result *ImportResult) error {
	rc := &tokenRewriteCtx{
		courseID:      courseID,
		fileURLByPath: result.fileURLByPath,
		pageBySlug:    result.pageBySlug,
		entityByMigID: result.entityByMigID,
	}

	var hardErr error
	record := func(kind string, id uint, err error) {
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			// Caller cancelled — bail without flagging it as a hard failure.
			return
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not update %s %d: %v", kind, id, err))
		if hardErr == nil {
			hardErr = fmt.Errorf("%s %d update failed during pass-2: %w", kind, id, err)
		}
	}

	for _, e := range result.CreatedEntities {
		if ctx.Err() != nil {
			break
		}
		switch e.Type {
		case "WikiPage":
			page, err := p.pageRepo.FindByID(ctx, e.ID, 0)
			if err != nil {
				continue
			}
			newBody := rewriteTokens(page.Body, rc)
			if newBody != page.Body {
				page.Body = newBody
				record("page", page.ID, p.pageRepo.Update(ctx, page))
			}
		case "Assignment":
			a, err := p.assignmentRepo.FindByID(ctx, e.ID, 0)
			if err != nil {
				continue
			}
			newDesc := rewriteTokens(a.Description, rc)
			if newDesc != a.Description {
				a.Description = newDesc
				record("assignment", a.ID, p.assignmentRepo.Update(ctx, a))
			}
		case "Quiz":
			q, err := p.quizRepo.FindByID(ctx, e.ID, 0)
			if err != nil {
				continue
			}
			newDesc := rewriteTokens(q.Description, rc)
			if newDesc != q.Description {
				q.Description = newDesc
				record("quiz", q.ID, p.quizRepo.Update(ctx, q))
			}
		case "QuizQuestion":
			qq, err := p.quizQuestionRepo.FindByID(ctx, e.ID)
			if err != nil {
				continue
			}
			newText := rewriteTokens(qq.QuestionText, rc)
			if newText != qq.QuestionText {
				qq.QuestionText = newText
				record("quiz_question", qq.ID, p.quizQuestionRepo.Update(ctx, qq))
			}
		case "DiscussionTopic":
			d, err := p.discussionTopicRepo.FindByID(ctx, e.ID)
			if err != nil {
				continue
			}
			newMsg := rewriteTokens(d.Message, rc)
			if newMsg != d.Message {
				d.Message = newMsg
				record("discussion", d.ID, p.discussionTopicRepo.Update(ctx, d))
			}
		}
	}
	return hardErr
}

// CleanupFailedImport soft-deletes every entity that was created during a
// failed import. Iterates in reverse order (children before parents) and
// keeps going past individual delete failures so a single bad row can't
// abort the rollback.
func (p *IMSCCParser) CleanupFailedImport(ctx context.Context, courseID uint, refs []EntityRef) {
	_ = courseID // currently unused; kept for future per-course scoping
	for i := len(refs) - 1; i >= 0; i-- {
		e := refs[i]
		func() {
			defer func() {
				_ = recover()
			}()
			switch e.Type {
			case "ContentTag":
				_ = p.moduleItemRepo.Delete(ctx, e.ID)
			case "ContextModule":
				_ = p.moduleRepo.Delete(ctx, e.ID)
			case "WikiPage":
				_ = p.pageRepo.Delete(ctx, e.ID)
			case "Assignment":
				_ = p.assignmentRepo.Delete(ctx, e.ID)
			case "Quiz":
				_ = p.quizRepo.Delete(ctx, e.ID)
			case "QuizQuestion":
				_ = p.quizQuestionRepo.Delete(ctx, e.ID)
			case "DiscussionTopic":
				_ = p.discussionTopicRepo.Delete(ctx, e.ID)
			case "Attachment":
				_ = p.fileService.DeleteAttachment(ctx, e.ID)
			case "QuestionBank":
				_ = p.questionBankRepo.Delete(ctx, e.ID)
			case "QuestionBankEntry":
				_ = p.questionBankEntryRepo.Delete(ctx, e.ID)
			case "AssignmentGroup":
				_ = p.assignmentGroupRepo.Delete(ctx, e.ID)
			case "Announcement":
				if p.announcementRepo != nil {
					_ = p.announcementRepo.Delete(ctx, e.ID)
				}
			case "Rubric":
				if p.rubricRepo != nil {
					_ = p.rubricRepo.Delete(ctx, e.ID)
				}
			case "RubricAssociation":
				if p.rubricAssocRepo != nil {
					_ = p.rubricAssocRepo.Delete(ctx, e.ID)
				}
			case "LearningOutcomeGroup":
				if p.outcomeGroupRepo != nil {
					_ = p.outcomeGroupRepo.Delete(ctx, e.ID)
				}
			case "LearningOutcome":
				if p.outcomeRepo != nil {
					_ = p.outcomeRepo.Delete(ctx, e.ID)
				}
			case "CalendarEvent":
				if p.calendarEventRepo != nil {
					_ = p.calendarEventRepo.Delete(ctx, e.ID)
				}
			}
		}()
	}
}

// parseAssignmentGroups reads course_settings/assignment_groups.xml and
// creates one AssignmentGroup row per <assignmentGroup> element. Populates
// result.assignmentGroupByMigID so importAssignment can wire the FK.
//
// Missing or empty file is fine (not all cartridges include grouping).
func (p *IMSCCParser) parseAssignmentGroups(
	ctx context.Context,
	courseID uint,
	zipFiles map[string]*zip.File,
	result *ImportResult,
) {
	if p.assignmentGroupRepo == nil {
		return
	}
	data, err := readZipFile(zipFiles, "course_settings/assignment_groups.xml")
	if err != nil || len(data) == 0 {
		return
	}
	var groups canvasAssignmentGroups
	if err := xml.Unmarshal(data, &groups); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse assignment_groups.xml: %v", err))
		return
	}
	for _, g := range groups.Groups {
		ag := &models.AssignmentGroup{
			CourseID:      courseID,
			Name:          strings.TrimSpace(g.Title),
			WorkflowState: "available",
		}
		if ag.Name == "" {
			ag.Name = "Imported Group"
		}
		if g.Position != "" {
			if pos, err := strconv.Atoi(g.Position); err == nil {
				ag.Position = pos
			}
		}
		if g.GroupWeight != "" {
			if w, err := strconv.ParseFloat(g.GroupWeight, 64); err == nil {
				ag.GroupWeight = w
			}
		}
		if g.Rules != "" {
			ag.Rules = g.Rules
		}
		if err := p.assignmentGroupRepo.Create(ctx, ag); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to create assignment group %q: %v", ag.Name, err))
			continue
		}
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "AssignmentGroup", ID: ag.ID})
		if g.Identifier != "" {
			result.assignmentGroupByMigID[g.Identifier] = ag.ID
		}
	}
}

// readQuizMeta opens the assessment_meta.xml file that lives next to the
// given QTI href in the cartridge, parses it, and returns the populated
// canvasQuizMeta or nil if the file is absent / unparseable. We treat a
// parse failure as a soft miss so the QTI-only path still completes.
func readQuizMeta(zipFiles map[string]*zip.File, qtiHref string) *canvasQuizMeta {
	dir := path.Dir(qtiHref)
	candidate := path.Join(dir, "assessment_meta.xml")
	data, err := readZipFile(zipFiles, candidate)
	if err != nil {
		return nil
	}
	var meta canvasQuizMeta
	if err := xml.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

// applyQuizMeta layers the assessment_meta.xml fields onto a Quiz that's
// already been seeded from the QTI envelope. assessment_meta wins because
// QTI doesn't have a place to record allowed_attempts, scoring policy, etc.
func applyQuizMeta(q *models.Quiz, m *canvasQuizMeta) {
	if m.Title != "" {
		q.Title = m.Title
	}
	if m.Description != "" {
		q.Description = m.Description
	}
	if m.QuizType != "" {
		q.QuizType = m.QuizType
	}
	if m.PointsPossible != "" {
		if v, err := strconv.ParseFloat(m.PointsPossible, 64); err == nil {
			q.PointsPossible = &v
		}
	}
	if m.TimeLimit != "" {
		if v, err := strconv.Atoi(m.TimeLimit); err == nil {
			q.TimeLimit = &v
		}
	}
	if m.AllowedAttempts != "" {
		if v, err := strconv.Atoi(m.AllowedAttempts); err == nil {
			q.AllowedAttempts = v
		}
	}
	q.ShuffleAnswers = parseBoolFlag(m.ShuffleAnswers)
	if m.ScoringPolicy != "" {
		q.ScoringPolicy = m.ScoringPolicy
	}
	q.ShowCorrectAnswers = parseBoolFlag(m.ShowCorrectAnswers)
	q.HideResults = strings.TrimSpace(m.HideResults)
	q.OneQuestionAtATime = parseBoolFlag(m.OneQuestionAtATime)
	q.CantGoBack = parseBoolFlag(m.CantGoBack)
	if t := parseCanvasTime(m.LockAt); t != nil {
		q.LockAt = t
	}
	if t := parseCanvasTime(m.UnlockAt); t != nil {
		q.UnlockAt = t
	}
	if t := parseCanvasTime(m.DueAt); t != nil {
		q.DueAt = t
	}
	// The <assignment> sub-block sometimes carries the only date values.
	if t := parseCanvasTime(m.Assignment.DueAt); t != nil && q.DueAt == nil {
		q.DueAt = t
	}
	if t := parseCanvasTime(m.Assignment.LockAt); t != nil && q.LockAt == nil {
		q.LockAt = t
	}
	if t := parseCanvasTime(m.Assignment.UnlockAt); t != nil && q.UnlockAt == nil {
		q.UnlockAt = t
	}
	if parseBoolFlag(m.Available) {
		q.Published = true
		q.WorkflowState = "published"
	}
}

// parseCanvasTime accepts the ISO-8601 date strings Canvas writes to the
// cartridge (RFC3339 or yyyy-mm-dd), returning nil for empty / unparseable
// input. Canvas leaves these fields as <due_at/> when unset.
func parseCanvasTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

// importQuestionBank handles a Canvas question-bank resource. The bank is
// stored as a QTI assessment but we don't materialize it as a Quiz — every
// item becomes a QuestionBankEntry on a fresh QuestionBank row.
func (p *IMSCCParser) importQuestionBank(
	ctx context.Context,
	courseID uint,
	res ManifestResource,
	zipFiles map[string]*zip.File,
	result *ImportResult,
) {
	href := res.Href
	if href == "" && len(res.Files) > 0 {
		href = res.Files[0].Href
	}
	if href == "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("question-bank resource %q has no file reference", res.Identifier))
		return
	}
	data, err := readZipFile(zipFiles, href)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read question-bank file %q: %v", href, err))
		return
	}

	qtiResult, parseErr := ParseQTIAssessment(data)
	if parseErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to parse question-bank QTI %q: %v", res.Identifier, parseErr))
		return
	}

	title := qtiResult.Title
	if title == "" {
		title = titleFromHref(href)
	}
	if title == "" {
		title = res.Identifier
	}

	bank := &models.QuestionBank{
		CourseID:      courseID,
		Title:         title,
		WorkflowState: "active",
	}
	if err := p.questionBankRepo.Create(ctx, bank); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create question bank %q: %v", title, err))
		return
	}
	result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "QuestionBank", ID: bank.ID})

	for i := range qtiResult.Questions {
		q := &qtiResult.Questions[i]
		points := 1.0
		if q.PointsPossible != nil {
			points = *q.PointsPossible
		}
		entry := &models.QuestionBankEntry{
			QuestionBankID: bank.ID,
			QuestionType:   q.QuestionType,
			QuestionText:   q.QuestionText,
			QuestionName:   fmt.Sprintf("Question %d", q.Position),
			Answers:        q.Answers,
			PointsPossible: points,
			Position:       q.Position,
		}
		if err := p.questionBankEntryRepo.Create(ctx, entry); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to create question-bank entry %d for %q: %v", q.Position, title, err))
			continue
		}
		result.CreatedEntities = append(result.CreatedEntities, EntityRef{Type: "QuestionBankEntry", ID: entry.ID})
	}

	if res.Identifier != "" {
		result.entityByMigID[res.Identifier] = EntityRef{Type: "QuestionBank", ID: bank.ID}
	}
}
