package service

import (
	"archive/zip"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeFileHeader synthesizes a *multipart.FileHeader by round-tripping
// a real multipart request through net/http. This is the simplest way
// to produce a FileHeader pointer that's safe to call .Open() on,
// without importing fiber.
func makeFileHeader(t *testing.T, filePath, filename string) *multipart.FileHeader {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(part, bytes.NewReader(data))
	mw.Close()

	req, err := http.NewRequest("POST", "/upload", strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(64 << 20); err != nil {
		t.Fatal(err)
	}
	fhs := req.MultipartForm.File["file"]
	if len(fhs) == 0 {
		t.Fatal("no file headers")
	}
	return fhs[0]
}

// buildTinyClassicIMSCC writes a minimal valid Canvas Classic .imscc
// to a temp file and returns its path. Used by ImportFromPath tests.
func buildTinyClassicIMSCC(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "tiny-*.imscc")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	files := map[string]string{
		"imsmanifest.xml": `<?xml version="1.0"?>
<manifest xmlns="http://www.imsglobal.org/xsd/imsccv1p1/imscp_v1p1">
  <resources>
    <resource identifier="a" type="imsqti_xmlv1p2/imscc_xmlv1p1/assessment" href="a.xml.qti">
      <file href="a.xml.qti"/>
    </resource>
  </resources>
</manifest>`,
		"a.xml.qti": `<?xml version="1.0"?>
<questestinterop>
  <assessment ident="a1" title="Tiny">
    <qtimetadata/>
    <section ident="s">
      <item ident="q1" title="MC">
        <itemmetadata><qtimetadata>
          <qtimetadatafield><fieldlabel>question_type</fieldlabel><fieldentry>multiple_choice_question</fieldentry></qtimetadatafield>
          <qtimetadatafield><fieldlabel>points_possible</fieldlabel><fieldentry>1</fieldentry></qtimetadatafield>
        </qtimetadata></itemmetadata>
        <presentation>
          <material><mattext>2+2?</mattext></material>
          <response_lid ident="r1" rcardinality="Single">
            <render_choice>
              <response_label ident="b"><material><mattext>4</mattext></material></response_label>
            </render_choice>
          </response_lid>
        </presentation>
        <resprocessing>
          <outcomes><decvar maxvalue="100" varname="SCORE"/></outcomes>
          <respcondition continue="No">
            <conditionvar><varequal respident="r1">b</varequal></conditionvar>
            <setvar varname="SCORE" action="Set">100</setvar>
          </respcondition>
        </resprocessing>
      </item>
    </section>
  </assessment>
</questestinterop>`,
	}
	for name, data := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(data))
	}
	zw.Close()
	return filepath.Clean(f.Name())
}
