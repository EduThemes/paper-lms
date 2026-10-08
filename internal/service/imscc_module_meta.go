package service

import (
	"context"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// G068: module settings carried by course_settings/module_meta.xml beyond
// publish state / sequential progress — unlock dates, prerequisites,
// "complete one" requirement counts and item completion requirements —
// plus the cartridge's folder structure for files.

// CCCompletionRequirement is one <completionRequirement> of a module.
type CCCompletionRequirement struct {
	Type string
	// MinScore is the min_score threshold in points.
	MinScore *float64
	// MinPercentage is Canvas's min_percentage; it is converted to
	// points against the item's assignment when applied.
	MinPercentage *float64
	// ItemRef is the module_meta item identifier the requirement is on.
	ItemRef string
}

type ccPrerequisitesXML struct {
	Items []ccPrerequisiteXML `xml:"prerequisite"`
}

type ccPrerequisiteXML struct {
	Type          string `xml:"type,attr"`
	Title         string `xml:"title"`
	IdentifierRef string `xml:"identifierref"`
}

type ccCompletionRequirementsXML struct {
	Items []ccCompletionRequirementXML `xml:"completionRequirement"`
}

type ccCompletionRequirementXML struct {
	Type          string `xml:"type,attr"`
	MinScore      string `xml:"min_score"`
	MinPercentage string `xml:"min_percentage"`
	IdentifierRef string `xml:"identifierref"`
}

// SetPrerequisiteRepo lets the importer store module prerequisites.
func (p *IMSCCParser) SetPrerequisiteRepo(repo repository.ModulePrerequisiteRepository) {
	p.prereqRepo = repo
}

func parseOptionalFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return nil
	}
	return &v
}

// applyModuleMetaXML copies the G068 fields of one <module> into ms.
func applyModuleMetaXML(ms *CCModuleSettings, m ccModuleXML) {
	ms.UnlockAt = parseCanvasTime(strings.TrimSpace(m.UnlockAt))
	if n, err := strconv.Atoi(strings.TrimSpace(m.RequirementCount)); err == nil && n == 1 {
		ms.RequirementCount = 1
	}
	for _, pr := range m.Prerequisites.Items {
		t := strings.TrimSpace(pr.Type)
		ref := strings.TrimSpace(pr.IdentifierRef)
		if ref != "" && (t == "" || t == "context_module") {
			ms.Prerequisites = append(ms.Prerequisites, ref)
		}
	}
	for _, cr := range m.CompletionRequirements.Items {
		req := CCCompletionRequirement{
			Type:          strings.TrimSpace(cr.Type),
			MinScore:      parseOptionalFloat(cr.MinScore),
			MinPercentage: parseOptionalFloat(cr.MinPercentage),
			ItemRef:       strings.TrimSpace(cr.IdentifierRef),
		}
		if req.Type != "" && req.ItemRef != "" {
			ms.Requirements = append(ms.Requirements, req)
		}
	}
}

// recordItemTag maps a manifest item identifier to the ContentTag the
// importer created for it (the last ContentTag appended to
// CreatedEntities since index before).
func (r *ImportResult) recordItemTag(identifier string, before int) {
	if identifier == "" || r.itemTagByMigID == nil {
		return
	}
	for i := len(r.CreatedEntities) - 1; i >= before && i >= 0; i-- {
		if r.CreatedEntities[i].Type == "ContentTag" {
			r.itemTagByMigID[identifier] = r.CreatedEntities[i].ID
			return
		}
	}
}

// applyModuleMetaLinks stores the imported prerequisites and completion
// requirements once every module and item exists. Anything that cannot
// be resolved or is not valid for the item is skipped with a warning.
func (p *IMSCCParser) applyModuleMetaLinks(ctx context.Context, result *ImportResult) {
	for modRef, ms := range result.moduleMeta {
		ent, ok := result.entityByMigID[modRef]
		if !ok || ent.Type != "ContextModule" {
			continue
		}
		moduleID := ent.ID

		if len(ms.Prerequisites) > 0 {
			var ids []uint
			for _, ref := range ms.Prerequisites {
				pre, ok := result.entityByMigID[ref]
				if !ok || pre.Type != "ContextModule" || pre.ID == moduleID {
					result.Warnings = append(result.Warnings, fmt.Sprintf("module prerequisite %q not found; skipped", ref))
					continue
				}
				ids = append(ids, pre.ID)
			}
			if len(ids) > 0 {
				if p.prereqRepo == nil {
					result.Warnings = append(result.Warnings, "module prerequisites were not imported (no prerequisite store)")
				} else if err := p.prereqRepo.SetPrerequisites(ctx, moduleID, ids); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("could not set module prerequisites: %v", err))
				}
			}
		}

		for _, req := range ms.Requirements {
			tagID, ok := result.itemTagByMigID[req.ItemRef]
			if !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("completion requirement on unknown item %q skipped", req.ItemRef))
				continue
			}
			tag, err := p.moduleItemRepo.FindByID(ctx, tagID)
			if err != nil || tag == nil || tag.ContextModuleID != moduleID {
				continue
			}
			typ := req.Type
			minScore := req.MinScore
			if typ == "min_percentage" {
				minScore = p.percentageToPoints(ctx, tag, req.MinPercentage)
				typ = models.ModuleCompletionMinScore
				if minScore == nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("min_percentage requirement on %q has no points to convert; skipped", tag.Title))
					continue
				}
			}
			allowed := false
			for _, t := range models.ValidCompletionTypesFor(tag.ContentType) {
				if t == typ {
					allowed = true
					break
				}
			}
			if !allowed || (typ == models.ModuleCompletionMinScore && minScore == nil) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("completion requirement %q is not valid for %q; skipped", req.Type, tag.Title))
				continue
			}
			tag.CompletionType = typ
			tag.MinScore = nil
			if typ == models.ModuleCompletionMinScore {
				tag.MinScore = minScore
			}
			if err := p.moduleItemRepo.Update(ctx, tag); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not set completion requirement on %q: %v", tag.Title, err))
			}
		}
	}
}

// percentageToPoints converts a min_percentage requirement to points
// using the linked assignment's points possible.
func (p *IMSCCParser) percentageToPoints(ctx context.Context, tag *models.ContentTag, pct *float64) *float64 {
	if pct == nil || tag.ContentID == nil || tag.ContentType != "Assignment" || p.assignmentRepo == nil {
		return nil
	}
	a, err := p.assignmentRepo.FindByID(ctx, *tag.ContentID, 0)
	if err != nil || a == nil || a.PointsPossible == nil || *a.PointsPossible <= 0 {
		return nil
	}
	v := math.Round(*pct/100**a.PointsPossible*100) / 100
	return &v
}

// importFolderFor returns the course folder a cartridge file belongs in,
// creating the folders of its path under the course files root as
// needed. web_resources/ is the root itself (Canvas exports course files
// there). Returns 0 for files at the root.
func (p *IMSCCParser) importFolderFor(ctx context.Context, courseID uint, root *models.Folder, name string, cache map[string]uint) (uint, error) {
	dir := path.Dir(strings.TrimPrefix(name, "/"))
	if dir == "." || dir == "" {
		return 0, nil
	}
	if dir == "web_resources" {
		return 0, nil
	}
	dir = strings.TrimPrefix(dir, "web_resources/")

	parentID := root.ID
	parentFull := root.FullName
	key := ""
	for _, seg := range strings.Split(dir, "/") {
		seg = strings.TrimSpace(seg)
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		key += "/" + seg
		full := parentFull + "/" + seg
		if id, ok := cache[key]; ok {
			parentID, parentFull = id, full
			continue
		}
		id, err := p.findOrCreateFolder(ctx, courseID, parentID, seg, full)
		if err != nil {
			return 0, err
		}
		cache[key] = id
		parentID, parentFull = id, full
	}
	if parentID == root.ID {
		return 0, nil
	}
	return parentID, nil
}

func (p *IMSCCParser) findOrCreateFolder(ctx context.Context, courseID, parentID uint, name, fullName string) (uint, error) {
	parent := parentID
	for page := 1; page < 50; page++ {
		res, err := p.fileService.ListFolders(ctx, "Course", courseID, &parent, repository.PaginationParams{Page: page, PerPage: 100})
		if err != nil || res == nil {
			break
		}
		for _, f := range res.Items {
			if f.Name == name && f.WorkflowState != "deleted" {
				return f.ID, nil
			}
		}
		if int64(page*100) >= res.TotalCount || len(res.Items) == 0 {
			break
		}
	}
	folder := &models.Folder{
		ContextType:    "Course",
		ContextID:      courseID,
		ParentFolderID: &parent,
		Name:           name,
		FullName:       fullName,
		WorkflowState:  "visible",
	}
	if err := p.fileService.CreateFolder(ctx, folder); err != nil {
		return 0, err
	}
	return folder.ID, nil
}
