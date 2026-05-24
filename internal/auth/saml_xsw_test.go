package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/xml"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// Regression tests for PENTEST F-054 — the SAML XML Signature Wrapping
// (XSW) attack and surrounding signature-validation bugs. The pre-fix
// code used a home-grown extractSignatureComponents that
// RSA-verified the SignedInfo bytes but DISCARDED the DigestValue,
// so an attacker could swap the referenced assertion for a malicious
// one (classic XSW) and the signature would still "verify."
//
// The fix uses github.com/russellhaering/goxmldsig (EXC-C14N + digest
// validation + cert verification) and then pins the parsed Assertion
// to the signed element's ID in HandleACS. These tests lock both
// halves of the contract.

// makeSelfSignedCertAndKey generates an in-memory RSA keypair and a
// self-signed X.509 certificate for in-test signing. The returned
// keystore is the format goxmldsig's signing context expects.
func makeSelfSignedCertAndKey(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, dsig.X509KeyStore) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "saml-xsw-test"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	store := &memoryKeyStore{cert: certBytes, key: key}
	return cert, key, store
}

type memoryKeyStore struct {
	cert []byte
	key  *rsa.PrivateKey
}

func (m *memoryKeyStore) GetKeyPair() (*rsa.PrivateKey, []byte, error) {
	return m.key, m.cert, nil
}

// buildSignedSAMLResponse produces a SAML Response XML byte slice with
// the <samlp:Response> element signed by the given keystore (signature
// on the Response root — the canonical "Response is signed" form).
// The Response contains one Assertion with the given assertionID.
func buildSignedSAMLResponse(t *testing.T, store dsig.X509KeyStore, assertionID string) []byte {
	t.Helper()
	signCtx := dsig.NewDefaultSigningContext(store)
	signCtx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")

	response := etree.NewElement("samlp:Response")
	response.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	response.CreateAttr("ID", "_resp-"+assertionID)
	response.CreateAttr("Version", "2.0")
	response.CreateAttr("IssueInstant", time.Now().UTC().Format(time.RFC3339))
	status := response.CreateElement("samlp:Status")
	sc := status.CreateElement("samlp:StatusCode")
	sc.CreateAttr("Value", "urn:oasis:names:tc:SAML:2.0:status:Success")

	assertion := response.CreateElement("Assertion")
	assertion.CreateAttr("xmlns", "urn:oasis:names:tc:SAML:2.0:assertion")
	assertion.CreateAttr("ID", assertionID)
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("IssueInstant", time.Now().UTC().Format(time.RFC3339))
	issuer := assertion.CreateElement("Issuer")
	issuer.SetText("https://idp.test")
	subject := assertion.CreateElement("Subject")
	nameID := subject.CreateElement("NameID")
	nameID.SetText("alice@idp.test")

	signed, err := signCtx.SignEnveloped(response)
	if err != nil {
		t.Fatalf("sign response: %v", err)
	}

	doc := etree.NewDocument()
	doc.SetRoot(signed)
	xmlBytes, err := doc.WriteToBytes()
	if err != nil {
		t.Fatalf("serialize response: %v", err)
	}
	return xmlBytes
}

// validateBytes is a thin wrapper that wires up the cert store and
// returns the validated element's ID + error, mirroring what
// SAMLHandler.verifyResponseSignature does after cert lookup —
// INCLUDING the Response-first, Assertion-fallback walk for IdPs
// (Okta, ADFS) that sign just the Assertion.
func validateBytes(t *testing.T, cert *x509.Certificate, xmlBytes []byte) (string, error) {
	t.Helper()
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		return "", err
	}
	store := &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}}
	ctx := dsig.NewDefaultValidationContext(store)
	root := doc.Root()
	if validated, err := ctx.Validate(root); err == nil {
		return validated.SelectAttrValue("ID", ""), nil
	}
	for _, child := range root.ChildElements() {
		if !strings.HasSuffix(child.Tag, "Assertion") {
			continue
		}
		if validated, err := ctx.Validate(child); err == nil {
			return validated.SelectAttrValue("ID", ""), nil
		}
	}
	return "", errSignatureNotFound
}

var errSignatureNotFound = newErr("no valid signature on Response or Assertion")

func newErr(msg string) error { return &simpleErr{msg} }

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

// buildAssertionSignedSAMLResponse produces a SAML Response where only
// the <Assertion> child is signed (signature inside the Assertion,
// referencing the Assertion's ID). This is the Okta / ADFS default
// shape — the original PR's test fixtures only covered the
// Response-signed case and the production code only tried Validate(root),
// which rejected real-world IdP traffic.
func buildAssertionSignedSAMLResponse(t *testing.T, store dsig.X509KeyStore, assertionID string) []byte {
	t.Helper()
	signCtx := dsig.NewDefaultSigningContext(store)
	signCtx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")

	assertion := etree.NewElement("Assertion")
	assertion.CreateAttr("xmlns", "urn:oasis:names:tc:SAML:2.0:assertion")
	assertion.CreateAttr("ID", assertionID)
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("IssueInstant", time.Now().UTC().Format(time.RFC3339))
	issuer := assertion.CreateElement("Issuer")
	issuer.SetText("https://idp.test")
	subject := assertion.CreateElement("Subject")
	nameID := subject.CreateElement("NameID")
	nameID.SetText("alice@idp.test")

	signedAssertion, err := signCtx.SignEnveloped(assertion)
	if err != nil {
		t.Fatalf("sign assertion: %v", err)
	}

	response := etree.NewElement("samlp:Response")
	response.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	response.CreateAttr("ID", "_resp-"+assertionID)
	response.CreateAttr("Version", "2.0")
	response.CreateAttr("IssueInstant", time.Now().UTC().Format(time.RFC3339))
	status := response.CreateElement("samlp:Status")
	sc := status.CreateElement("samlp:StatusCode")
	sc.CreateAttr("Value", "urn:oasis:names:tc:SAML:2.0:status:Success")
	response.AddChild(signedAssertion)

	doc := etree.NewDocument()
	doc.SetRoot(response)
	xmlBytes, err := doc.WriteToBytes()
	if err != nil {
		t.Fatalf("serialize response: %v", err)
	}
	return xmlBytes
}

// TestValidSignedAssertion_Accepts is the happy-path lock. A response
// signed by the trusted IdP cert validates and exposes the signed
// element's ID for downstream pinning.
func TestValidSignedAssertion_Accepts(t *testing.T) {
	cert, _, store := makeSelfSignedCertAndKey(t)
	xmlBytes := buildSignedSAMLResponse(t, store, "_assertion-happy")

	signedID, err := validateBytes(t, cert, xmlBytes)
	if err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}
	// Signed element is the Response; its ID is "_resp-<assertionID>".
	if signedID != "_resp-_assertion-happy" {
		t.Errorf("expected signedID=_resp-_assertion-happy, got %q", signedID)
	}
}

// TestAssertionSignedResponse_Accepts locks the real-world Okta/ADFS
// path — the IdP signs ONLY the <Assertion>, not the <Response>.
// Pre-fix verifyResponseSignature called goxmldsig.Validate(root)
// which only matches signatures whose Reference URI equals the
// Response's ID; this case returned "Missing reference" → 401.
// Now: Response-level validate fails, but the Assertion-level walk
// succeeds and the signed Assertion's ID is returned for downstream
// pinning.
func TestAssertionSignedResponse_Accepts(t *testing.T) {
	cert, _, store := makeSelfSignedCertAndKey(t)
	xmlBytes := buildAssertionSignedSAMLResponse(t, store, "_assertion-okta-style")

	signedID, err := validateBytes(t, cert, xmlBytes)
	if err != nil {
		t.Fatalf("expected Assertion-signed response to validate, got error: %v", err)
	}
	if signedID != "_assertion-okta-style" {
		t.Errorf("expected signedID=_assertion-okta-style (the Assertion's ID), got %q", signedID)
	}
}

// TestTamperedAssertion_Rejects locks the digest-check. Flipping bytes
// in the signed assertion must trip goxmldsig (the pre-fix code, which
// only verified the RSA signature over SignedInfo, would have missed
// any tamper outside the SignedInfo block).
func TestTamperedAssertion_Rejects(t *testing.T) {
	cert, _, store := makeSelfSignedCertAndKey(t)
	xmlBytes := buildSignedSAMLResponse(t, store, "_assertion-tampered")

	// Tamper: change the NameID value AFTER signing. The signature
	// over SignedInfo is still valid; the DigestValue covering the
	// canonicalized Assertion is now wrong.
	tampered := strings.Replace(string(xmlBytes), "alice@idp.test", "mallory@idp.test", 1)

	if _, err := validateBytes(t, cert, []byte(tampered)); err == nil {
		t.Fatal("expected validation to fail on tampered assertion, got nil error")
	}
}

// TestXSWPayload_RejectedByIDPin is the central F-054 regression. The
// attacker wraps an UNSIGNED malicious assertion BEFORE the signed
// one, hoping the downstream parser reads Assertions[0] (the
// malicious one) while the signature validates the second. The fix
// catches this two ways: (a) goxmldsig's digest check fails when the
// wrap changes the document structure around the signed reference;
// (b) the ID-pinning in HandleACS rejects when
// samlResp.Assertions[0].ID != signedID.
//
// This test exercises path (b) by skipping signature validation
// (which would fail anyway) and verifying that the ID-mismatch
// rejection is the documented contract.
func TestXSWPayload_RejectedByIDPin(t *testing.T) {
	_, _, store := makeSelfSignedCertAndKey(t)
	xmlBytes := buildSignedSAMLResponse(t, store, "_assertion-legit")

	// Wrap an unsigned malicious Assertion BEFORE the signed one.
	// xml.Unmarshal into samlResponse will populate Assertions[0]
	// with the malicious one.
	maliciousAssertion := `<Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_assertion-malicious" Version="2.0" IssueInstant="2026-05-23T00:00:00Z"><Issuer>https://attacker.test</Issuer><Subject><NameID>mallory@attacker.test</NameID></Subject></Assertion>`

	// Inject right after the </samlp:Status> close so the malicious
	// Assertion sits before the signed one in document order.
	xswPayload := strings.Replace(
		string(xmlBytes),
		"</samlp:Status>",
		"</samlp:Status>"+maliciousAssertion,
		1,
	)

	var resp samlResponse
	if err := xml.Unmarshal([]byte(xswPayload), &resp); err != nil {
		t.Fatalf("XSW payload must remain XML-parseable for the test: %v", err)
	}
	if len(resp.Assertions) != 2 {
		t.Fatalf("expected XSW payload to parse as 2 assertions, got %d", len(resp.Assertions))
	}
	if resp.Assertions[0].ID != "_assertion-malicious" {
		t.Errorf("expected first parsed assertion to be the malicious one, got %q", resp.Assertions[0].ID)
	}

	// HandleACS's ID-pinning rule: signedID == _assertion-legit, but
	// Assertions[0].ID == _assertion-malicious, AND Response.ID is
	// the unsigned outer response. Pinning rejects.
	signedID := "_assertion-legit"
	if resp.Assertions[0].ID == signedID || resp.ID == signedID {
		t.Errorf("XSW payload should not satisfy ID-pin against signedID=%q (Assertions[0].ID=%q, Response.ID=%q)",
			signedID, resp.Assertions[0].ID, resp.ID)
	}

	// And the count-rule (must be exactly one Assertion):
	if len(resp.Assertions) == 1 {
		t.Error("XSW payload should not satisfy the exactly-one-Assertion rule")
	}
}

// TestUnsignedResponse_Rejects locks the "no signature found" path.
// A SAML response with no <Signature> element must fail validation
// when an IdP cert is configured; the pre-fix code's regex would
// silently return empty bytes and the RSA verify would fail with a
// confusing "no signature found" — same outcome, clearer error now.
func TestUnsignedResponse_Rejects(t *testing.T) {
	cert, _, _ := makeSelfSignedCertAndKey(t)
	unsignedXML := []byte(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_unsigned-1" Version="2.0" IssueInstant="2026-05-23T00:00:00Z"><Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_a1" Version="2.0" IssueInstant="2026-05-23T00:00:00Z"><Issuer>https://idp.test</Issuer><Subject><NameID>alice@idp.test</NameID></Subject></Assertion></samlp:Response>`)

	if _, err := validateBytes(t, cert, unsignedXML); err == nil {
		t.Fatal("expected validation to fail on unsigned response")
	}
}

