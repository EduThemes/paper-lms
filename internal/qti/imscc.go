package qti

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// imsccBundle is the in-memory view of a Common Cartridge .imscc zip.
// All file contents are loaded into memory — Canvas exports are typically
// well under 10 MB even for multi-thousand-question courses, so this is
// pragmatic. If a future fixture violates that we add streaming.
type imsccBundle struct {
	// files maps the cartridge-relative path (forward-slash separated,
	// as written by every Canvas export ever) to raw file bytes.
	files map[string][]byte
	// manifest is the parsed imsmanifest.xml. May be nil if the bundle
	// is malformed; the parser falls back to scanning the zip for
	// likely QTI files.
	manifest *imsManifest
}

// imsManifest is a minimal parse of imsmanifest.xml. Canvas writes far
// more metadata than this — LOM, organizations, dependencies, etc. — but
// we only need the resource list to find the QTI files and the
// assessment_question_banks directories.
type imsManifest struct {
	XMLName   xml.Name      `xml:"manifest"`
	Resources []imsResource `xml:"resources>resource"`
}

type imsResource struct {
	Identifier string    `xml:"identifier,attr"`
	Type       string    `xml:"type,attr"`
	Href       string    `xml:"href,attr"`
	Files      []imsFile `xml:"file"`
}

type imsFile struct {
	Href string `xml:"href,attr"`
}

// openIMSCC reads a .imscc zip file into memory. It returns an error if
// the file is not a valid zip; it does NOT error on a missing manifest
// (some hand-rolled exports skip the manifest). The caller should check
// bundle.manifest == nil and fall back to file-extension heuristics.
func openIMSCC(zipPath string) (*imsccBundle, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open imscc zip: %w", err)
	}
	defer zr.Close()

	b := &imsccBundle{files: make(map[string][]byte)}

	for _, f := range zr.File {
		// Skip directories — Canvas exports them with trailing slashes.
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %q in zip: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q from zip: %w", f.Name, err)
		}
		// Normalize separators — defensive; Canvas writes "/" but
		// some tools (Windows zips) write "\".
		key := strings.ReplaceAll(f.Name, "\\", "/")
		b.files[key] = data
	}

	if manifestBytes, ok := b.files["imsmanifest.xml"]; ok {
		var m imsManifest
		if err := xml.Unmarshal(manifestBytes, &m); err == nil {
			b.manifest = &m
		}
		// Don't propagate a manifest parse failure — fall through to
		// the file-extension heuristics in findAssessmentFiles().
	}

	return b, nil
}

// findAssessmentFiles returns the paths inside the bundle that contain
// QTI assessment definitions. Detection is two-stage:
//
//  1. Prefer the manifest's resource list — Canvas writes either
//     "imsqti_xmlv1p2/imscc_xmlv1p1/assessment" (Classic) or
//     "imsqti_xmlv2p2" (NQ) for the resource type.
//  2. Fall back to scanning for *.xml files under known prefixes:
//     "non_cc_assessments/", or any file containing `<assessment` /
//     `<assessmentItem` near the top.
//
// We split into two slices so callers can route Classic vs NQ files to
// the right parser. assessmentBankFiles is a separate slice for the
// Canvas Classic <objectbank> files.
func (b *imsccBundle) findAssessmentFiles() (classic []string, newQuizzes []string, banks []string) {
	if b.manifest != nil {
		for _, r := range b.manifest.Resources {
			rtype := strings.ToLower(r.Type)
			// QTI 1.2 / Classic.
			if strings.Contains(rtype, "imsqti_xmlv1p2") ||
				strings.Contains(rtype, "imsqti_xmlv1p1") ||
				strings.Contains(rtype, "assessment_question_bank") {
				// Heuristic: if the href is under
				// assessment_question_banks/, it's a bank, otherwise
				// an assessment.
				href := r.Href
				if href == "" && len(r.Files) > 0 {
					href = r.Files[0].Href
				}
				if href == "" {
					continue
				}
				if strings.Contains(href, "assessment_question_banks") ||
					strings.Contains(rtype, "assessment_question_bank") {
					banks = append(banks, href)
				} else {
					classic = append(classic, href)
				}
				continue
			}
			// QTI 2.2 / New Quizzes.
			if strings.Contains(rtype, "imsqti_xmlv2p2") ||
				strings.Contains(rtype, "imsqti_item_xmlv2p2") ||
				strings.Contains(rtype, "imsqti_test_xmlv2p2") {
				href := r.Href
				if href == "" && len(r.Files) > 0 {
					href = r.Files[0].Href
				}
				if href != "" {
					newQuizzes = append(newQuizzes, href)
				}
				continue
			}
		}
	}

	// Fallback: scan all *.xml files when the manifest didn't help OR
	// when the manifest only listed a subset (we've seen both). De-dupe
	// against the lists above.
	seen := map[string]bool{}
	for _, p := range classic {
		seen[p] = true
	}
	for _, p := range newQuizzes {
		seen[p] = true
	}
	for _, p := range banks {
		seen[p] = true
	}
	for name, data := range b.files {
		if !strings.HasSuffix(strings.ToLower(name), ".xml") {
			continue
		}
		if seen[name] {
			continue
		}
		if name == "imsmanifest.xml" {
			continue
		}
		head := peekHead(data)
		// New Quizzes uses <assessmentItem> or <assessmentTest> as
		// the top-level QTI 2.2 element.
		if strings.Contains(head, "<assessmentItem") || strings.Contains(head, "<assessmentTest") {
			newQuizzes = append(newQuizzes, name)
			seen[name] = true
			continue
		}
		// Classic uses <questestinterop> or <objectbank>.
		if strings.Contains(head, "<questestinterop") || strings.Contains(head, "<objectbank") {
			if strings.Contains(name, "assessment_question_banks") || strings.Contains(head, "<objectbank") {
				banks = append(banks, name)
			} else {
				classic = append(classic, name)
			}
			seen[name] = true
		}
	}
	return classic, newQuizzes, banks
}

// peekHead returns the first ~512 bytes of an XML file as a string, used
// for quick top-element sniffing. We don't fully parse here — that
// happens in the dialect-specific parsers.
func peekHead(data []byte) string {
	if len(data) > 512 {
		return string(data[:512])
	}
	return string(data)
}

// resolvePath resolves a manifest-relative href to a key in b.files.
// Canvas writes hrefs in the cartridge-relative form already, so this
// is mostly a passthrough — but we also try a few common variations
// (with/without leading slash, posix-joined against the resource dir)
// because hand-rolled exports occasionally drift.
func (b *imsccBundle) resolvePath(href string) ([]byte, string, bool) {
	candidates := []string{
		href,
		strings.TrimPrefix(href, "/"),
		strings.TrimPrefix(href, "./"),
		path.Clean(href),
	}
	for _, c := range candidates {
		if data, ok := b.files[c]; ok {
			return data, c, true
		}
	}
	return nil, "", false
}

