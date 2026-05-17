package auth

import (
	"context"
	"crypto/tls"
	"encoding/asn1"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// BackfillLDAPBindPasswords seals any LDAP bind passwords that are
// still stored in the legacy plaintext column. Invoked at boot (right
// after EnsureKeysLoaded) so the Phase 9-PRE encryption-at-rest
// contract is enforced as a closed loop without operator action.
//
// Idempotent: rows whose ldap_bind_password_encrypted column already
// has bytes are skipped. On a freshly-encrypted DB this is a single
// SELECT that returns nothing. The function returns the number of
// rows it sealed so the caller can log the migration footprint.
//
// Failure modes: a row whose plaintext cannot be sealed (e.g. the
// encryption key is unavailable) aborts the whole backfill — we'd
// rather leave the row plaintext for one more boot than partially
// migrate. Callers should treat a non-nil error as boot-non-fatal in
// development (so a missing MFA_ENCRYPTION_KEY doesn't lock out the
// dev loop) but fatal in production.
func BackfillLDAPBindPasswords(ctx context.Context, db *gorm.DB) (int, error) {
	if db == nil {
		return 0, nil
	}
	// Probe the keychain first so we fail fast rather than after the
	// SELECT. EnsureKeysLoaded is idempotent (sync.Once) so this is
	// free if the caller already invoked it.
	if err := EnsureKeysLoaded(); err != nil {
		return 0, fmt.Errorf("encryption keys unavailable: %w", err)
	}

	type row struct {
		ID               uint
		LDAPBindPassword string
	}
	var rows []row
	q := db.WithContext(ctx).
		Table("authentication_providers").
		Select("id, ldap_bind_password").
		Where("auth_type = ?", "ldap").
		Where("ldap_bind_password IS NOT NULL AND ldap_bind_password <> ''").
		Where("ldap_bind_password_encrypted IS NULL OR octet_length(ldap_bind_password_encrypted) = 0")
	if err := q.Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("list backfill candidates: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	sealed := 0
	for _, r := range rows {
		ct, err := Encrypt([]byte(r.LDAPBindPassword))
		if err != nil {
			return sealed, fmt.Errorf("seal row %d: %w", r.ID, err)
		}
		// Write the ciphertext and clear the plaintext column in the
		// same UPDATE so a crash mid-loop can't leave a row with both
		// columns populated. The plaintext column staying nullable
		// after Wave-A is intentional — Wave-B drops it.
		err = db.WithContext(ctx).
			Table("authentication_providers").
			Where("id = ?", r.ID).
			Updates(map[string]any{
				"ldap_bind_password_encrypted": ct,
				"ldap_bind_password":           "",
			}).Error
		if err != nil {
			return sealed, fmt.Errorf("update row %d: %w", r.ID, err)
		}
		sealed++
	}
	return sealed, nil
}

// LDAPAuthenticator provides LDAP authentication against an
// AuthenticationProvider.
//
// Sprint 10-C: this type no longer carries a UserRepository — the
// post-credential JIT / auto-link / audit flow moved to
// LoginPipeline. Callers (SSOHandler.HandleLDAPLogin) receive an
// SSOOutcome from BuildOutcome and run it through the pipeline.
type LDAPAuthenticator struct{}

// NewLDAPAuthenticator creates a new LDAPAuthenticator.
func NewLDAPAuthenticator() *LDAPAuthenticator {
	return &LDAPAuthenticator{}
}

// BuildOutcome validates a username/password combination against the
// LDAP server configured in the given AuthenticationProvider. On
// success it returns an SSOOutcome that the caller hands to
// LoginPipeline.Execute for JIT / auto-link / MFA / audit.
//
// The LDAP-side work — service-account bind, search filter, user-bind
// password verification — is byte-identical to the pre-10-C
// implementation. Only the post-credential block (user lookup +
// HashPassword + Create) is gone, replaced by the SSOOutcome return.
func (a *LDAPAuthenticator) BuildOutcome(ctx context.Context, provider *models.AuthenticationProvider, username, password string) (SSOOutcome, error) {
	if provider.AuthType != "ldap" {
		return SSOOutcome{}, fmt.Errorf("provider is not an LDAP provider")
	}
	if provider.LDAPHost == "" {
		return SSOOutcome{}, fmt.Errorf("LDAP host is not configured")
	}
	if provider.LDAPPort == 0 {
		return SSOOutcome{}, fmt.Errorf("LDAP port is not configured")
	}
	if provider.LDAPBase == "" {
		return SSOOutcome{}, fmt.Errorf("LDAP base DN is not configured")
	}

	// Determine the login attribute
	loginAttr := provider.LDAPLoginAttribute
	if loginAttr == "" {
		loginAttr = "uid"
	}

	// Connect to LDAP server
	conn, err := a.connect(provider)
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	// Step 1: Bind with service account (if configured) to search for the user.
	// Read the bind password through resolveLDAPBindPassword: prefer the
	// encrypted column (Phase 9-PRE encryption-at-rest), fall back to the
	// legacy plaintext field for rows that haven't been rotated through
	// the encryption path yet. The plaintext fallback goes away when
	// Wave-B drops the column.
	bindPassword, err := resolveLDAPBindPassword(provider)
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("ldap bind password unavailable: %w", err)
	}
	if provider.LDAPBindDN != "" && bindPassword != "" {
		if err := ldapBind(conn, provider.LDAPBindDN, bindPassword); err != nil {
			return SSOOutcome{}, fmt.Errorf("service account bind failed: %w", err)
		}
	}

	// Step 2: Search for the user
	filter := provider.LDAPFilter
	if filter == "" {
		filter = "(" + loginAttr + "=%s)"
	}
	// Replace %s with the actual username (sanitize for LDAP injection)
	searchFilter := strings.ReplaceAll(filter, "%s", ldapEscapeFilter(username))

	entries, err := ldapSearch(conn, provider.LDAPBase, searchFilter, []string{"dn", "cn", "mail", loginAttr, "givenName", "sn", "displayName"})
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("LDAP search failed: %w", err)
	}
	if len(entries) == 0 {
		return SSOOutcome{}, fmt.Errorf("user not found in LDAP directory")
	}

	entry := entries[0]
	userDN := entry.dn

	// Step 3: Bind as the found user to verify their password
	// We need a fresh connection for the user bind since the previous one is
	// bound as the service account.
	userConn, err := a.connect(provider)
	if err != nil {
		return SSOOutcome{}, fmt.Errorf("failed to connect for user bind: %w", err)
	}
	defer userConn.Close()

	if err := ldapBind(userConn, userDN, password); err != nil {
		return SSOOutcome{}, fmt.Errorf("invalid credentials")
	}

	// Step 4: Extract user attributes
	email := entry.getAttribute("mail")
	cn := entry.getAttribute("cn")
	givenName := entry.getAttribute("givenName")
	sn := entry.getAttribute("sn")
	displayName := entry.getAttribute("displayName")
	uid := entry.getAttribute(loginAttr)

	if email == "" {
		email = username
	}
	if displayName == "" {
		displayName = cn
	}
	if displayName == "" && givenName != "" {
		displayName = givenName
		if sn != "" {
			displayName = givenName + " " + sn
		}
	}
	if displayName == "" {
		displayName = username
	}
	loginID := uid
	if loginID == "" {
		loginID = username
	}

	// Step 5: Build the SSOOutcome. The caller hands this to
	// LoginPipeline.Execute, which decides whether to JIT-create a
	// user, auto-link to an existing one by verified email, or
	// resolve via federated_identities. EmailVerified is true here
	// because the directory authenticated the user on the bind in
	// Step 3 — there is no separate "the email belongs to you"
	// proof to seek.
	return SSOOutcome{
		ProviderID:      provider.ID,
		ProviderType:    "ldap",
		ExternalSubject: userDN, // DN is the IdP-stable identifier
		Email:           email,
		EmailVerified:   true,
		Name:            displayName,
		Attributes: map[string]any{
			"login_id":     loginID,
			"display_name": displayName,
			"given_name":   givenName,
			"sn":           sn,
			"email":        email,
		},
	}, nil
}


// TestConnection verifies connectivity to the LDAP server configured in the
// given AuthenticationProvider. It attempts to connect and optionally bind with
// the service account.
func (a *LDAPAuthenticator) TestConnection(ctx context.Context, provider *models.AuthenticationProvider) error {
	if provider.AuthType != "ldap" {
		return fmt.Errorf("provider is not an LDAP provider")
	}
	if provider.LDAPHost == "" {
		return fmt.Errorf("LDAP host is not configured")
	}
	if provider.LDAPPort == 0 {
		return fmt.Errorf("LDAP port is not configured")
	}

	conn, err := a.connect(provider)
	if err != nil {
		return fmt.Errorf("LDAP connection failed: %w", err)
	}
	defer conn.Close()

	// If a bind DN is configured, test the service account bind.
	// Same encrypted-first / plaintext-fallback resolution as BuildOutcome
	// — keeps the test surface honest about which storage path will be
	// used in production for this provider.
	bindPassword, err := resolveLDAPBindPassword(provider)
	if err != nil {
		return fmt.Errorf("ldap bind password unavailable: %w", err)
	}
	if provider.LDAPBindDN != "" && bindPassword != "" {
		if err := ldapBind(conn, provider.LDAPBindDN, bindPassword); err != nil {
			return fmt.Errorf("service account bind failed: %w", err)
		}
	}

	return nil
}

// resolveLDAPBindPassword reads the bind password from whichever column
// is populated. Prefers the AES-256-GCM ciphertext column (the Phase
// 9-PRE encryption-at-rest contract; see internal/auth/secretbox.go and
// the docstring on AuthenticationProvider.LDAPBindPasswordEncrypted),
// falls back to the legacy plaintext column for rows that predate the
// rotation. The plaintext fallback is the bridge that keeps this PR
// backward-compatible — Wave-B will drop the plaintext column and this
// branch with it. Empty inputs return "" with no error so callers can
// skip the service-account bind when no credential is configured at
// all (anonymous LDAP search).
func resolveLDAPBindPassword(provider *models.AuthenticationProvider) (string, error) {
	if len(provider.LDAPBindPasswordEncrypted) > 0 {
		pt, err := Decrypt(provider.LDAPBindPasswordEncrypted)
		if err != nil {
			return "", fmt.Errorf("decrypt ldap_bind_password_encrypted: %w", err)
		}
		return string(pt), nil
	}
	return provider.LDAPBindPassword, nil
}

// connect establishes a TCP or TLS connection to the LDAP server.
func (a *LDAPAuthenticator) connect(provider *models.AuthenticationProvider) (net.Conn, error) {
	addr := net.JoinHostPort(provider.LDAPHost, strconv.Itoa(provider.LDAPPort))
	dialer := net.Dialer{Timeout: 10 * time.Second}

	if provider.LDAPUseTLS {
		return tls.DialWithDialer(&dialer, "tcp", addr, &tls.Config{
			ServerName: provider.LDAPHost,
			MinVersion: tls.VersionTLS12,
		})
	}

	return dialer.Dial("tcp", addr)
}

// ---------------------------------------------------------------------------
// Lightweight LDAP protocol implementation using raw BER encoding
// Supports: Bind (simple) and Search operations
// ---------------------------------------------------------------------------

// ldapBind performs a simple LDAP bind operation (authentication).
func ldapBind(conn net.Conn, dn, password string) error {
	// Build BindRequest
	// BindRequest ::= [APPLICATION 0] SEQUENCE {
	//     version INTEGER (1..127),
	//     name LDAPDN,
	//     authentication AuthenticationChoice }
	// AuthenticationChoice ::= CHOICE {
	//     simple [0] OCTET STRING }

	version := berEncodeInteger(3)
	nameBER := berEncodeOctetString([]byte(dn))
	// Simple authentication: context-specific [0]
	simpleBind := berEncodeContextSpecific(0, []byte(password))

	bindRequest := berEncodeSequence(append(version, append(nameBER, simpleBind...)...))
	// Wrap as APPLICATION 0 (BindRequest)
	bindRequestApp := berEncodeApplication(0, bindRequest[2:]) // strip outer SEQUENCE tag/length
	// Wait, we need the whole thing as APPLICATION 0 containing version, name, auth
	bindRequestApp = berEncodeApplication(0, append(version, append(nameBER, simpleBind...)...))

	messageID := berEncodeInteger(1)
	ldapMessage := berEncodeSequence(append(messageID, bindRequestApp...))

	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write(ldapMessage); err != nil {
		return fmt.Errorf("failed to send bind request: %w", err)
	}

	// Read response
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		return fmt.Errorf("failed to read bind response: %w", err)
	}

	// Parse the response to check result code
	resultCode, errMsg, parseErr := parseLDAPResult(respBuf[:n])
	if parseErr != nil {
		return fmt.Errorf("failed to parse bind response: %w", parseErr)
	}
	if resultCode != 0 {
		if errMsg != "" {
			return fmt.Errorf("LDAP bind failed (code %d): %s", resultCode, errMsg)
		}
		return fmt.Errorf("LDAP bind failed with result code %d", resultCode)
	}

	return nil
}

// ldapEntry represents a single LDAP search result entry.
type ldapEntry struct {
	dn         string
	attributes map[string][]string
}

func (e *ldapEntry) getAttribute(name string) string {
	vals, ok := e.attributes[strings.ToLower(name)]
	if !ok || len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// ldapSearch performs an LDAP search operation and returns matching entries.
func ldapSearch(conn net.Conn, baseDN, filter string, attributes []string) ([]ldapEntry, error) {
	// Build SearchRequest
	// SearchRequest ::= [APPLICATION 3] SEQUENCE {
	//     baseObject   LDAPDN,
	//     scope        ENUMERATED { baseObject(0), singleLevel(1), wholeSubtree(2) },
	//     derefAliases ENUMERATED { neverDerefAliases(0), ... },
	//     sizeLimit    INTEGER (0..maxInt),
	//     timeLimit    INTEGER (0..maxInt),
	//     typesOnly    BOOLEAN,
	//     filter       Filter,
	//     attributes   AttributeSelection }

	baseDNBER := berEncodeOctetString([]byte(baseDN))
	scope := berEncodeEnumerated(2) // wholeSubtree
	derefAliases := berEncodeEnumerated(0)
	sizeLimit := berEncodeInteger(100)
	timeLimit := berEncodeInteger(30)
	typesOnly := berEncodeBoolean(false)
	filterBER := encodeLDAPFilter(filter)
	attrsBER := encodeLDAPAttributes(attributes)

	searchPayload := concat(baseDNBER, scope, derefAliases, sizeLimit, timeLimit, typesOnly, filterBER, attrsBER)
	searchRequest := berEncodeApplication(3, searchPayload)

	messageID := berEncodeInteger(2)
	ldapMessage := berEncodeSequence(append(messageID, searchRequest...))

	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, err
	}
	if _, err := conn.Write(ldapMessage); err != nil {
		return nil, fmt.Errorf("failed to send search request: %w", err)
	}

	// Read all response packets until we get SearchResultDone
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return nil, err
	}

	var entries []ldapEntry
	buf := make([]byte, 0, 65536)
	readBuf := make([]byte, 8192)

	for {
		n, err := conn.Read(readBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to read search response: %w", err)
		}
		buf = append(buf, readBuf[:n]...)

		// Try to parse complete LDAP messages from the buffer
		for {
			if len(buf) < 2 {
				break
			}

			// Parse the outer SEQUENCE length to determine message boundaries
			msgLen, headerLen, err := berDecodeLength(buf[1:])
			if err != nil || len(buf) < headerLen+1+msgLen {
				break // Need more data
			}

			msgData := buf[:headerLen+1+msgLen]
			buf = buf[headerLen+1+msgLen:]

			entry, isDone, parseErr := parseLDAPSearchResponse(msgData)
			if parseErr != nil {
				// Skip unparseable messages but continue
				continue
			}
			if isDone {
				return entries, nil
			}
			if entry != nil {
				entries = append(entries, *entry)
			}
		}
	}
}

// parseLDAPResult extracts the result code and error message from an LDAP response.
func parseLDAPResult(data []byte) (int, string, error) {
	// LDAP Message is a SEQUENCE containing:
	//   messageID INTEGER
	//   protocolOp (APPLICATION-tagged)
	// The protocolOp for BindResponse [APPLICATION 1] contains:
	//   resultCode ENUMERATED
	//   matchedDN  LDAPDN
	//   diagnosticMessage LDAPString

	var outer asn1.RawValue
	rest, err := asn1.Unmarshal(data, &outer)
	_ = rest
	if err != nil {
		return -1, "", fmt.Errorf("failed to unmarshal outer SEQUENCE: %w", err)
	}

	// Parse inner elements
	innerData := outer.Bytes
	// Skip messageID
	var msgID asn1.RawValue
	innerData, err = asn1.Unmarshal(innerData, &msgID)
	if err != nil {
		return -1, "", fmt.Errorf("failed to unmarshal messageID: %w", err)
	}

	// Parse protocol op (BindResponse = APPLICATION 1, or SearchResultDone = APPLICATION 5)
	var protocolOp asn1.RawValue
	_, err = asn1.Unmarshal(innerData, &protocolOp)
	if err != nil {
		return -1, "", fmt.Errorf("failed to unmarshal protocolOp: %w", err)
	}

	// Parse the contents of the protocol op
	opData := protocolOp.Bytes
	// resultCode ENUMERATED
	var resultCode asn1.RawValue
	opData, err = asn1.Unmarshal(opData, &resultCode)
	if err != nil {
		return -1, "", fmt.Errorf("failed to unmarshal resultCode: %w", err)
	}

	code := 0
	if len(resultCode.Bytes) > 0 {
		code = int(resultCode.Bytes[0])
	}

	// matchedDN
	var matchedDN asn1.RawValue
	opData, err = asn1.Unmarshal(opData, &matchedDN)
	if err != nil {
		return code, "", nil
	}

	// diagnosticMessage
	var diagMsg asn1.RawValue
	_, err = asn1.Unmarshal(opData, &diagMsg)
	if err != nil {
		return code, "", nil
	}

	return code, string(diagMsg.Bytes), nil
}

// parseLDAPSearchResponse parses a single LDAP message from a search response stream.
// Returns an entry for SearchResultEntry, or isDone=true for SearchResultDone.
func parseLDAPSearchResponse(data []byte) (*ldapEntry, bool, error) {
	var outer asn1.RawValue
	_, err := asn1.Unmarshal(data, &outer)
	if err != nil {
		return nil, false, err
	}

	innerData := outer.Bytes
	// Skip messageID
	var msgID asn1.RawValue
	innerData, err = asn1.Unmarshal(innerData, &msgID)
	if err != nil {
		return nil, false, err
	}

	var protocolOp asn1.RawValue
	_, err = asn1.Unmarshal(innerData, &protocolOp)
	if err != nil {
		return nil, false, err
	}

	// APPLICATION 4 = SearchResultEntry
	// APPLICATION 5 = SearchResultDone
	tag := protocolOp.Tag
	if protocolOp.Class == asn1.ClassApplication {
		tag = protocolOp.Tag
	}

	if tag == 5 {
		// SearchResultDone
		return nil, true, nil
	}

	if tag == 4 {
		// SearchResultEntry
		return parseSearchResultEntry(protocolOp.Bytes)
	}

	// SearchResultReference (tag 19) or other - skip
	return nil, false, nil
}

// parseSearchResultEntry parses the body of a SearchResultEntry.
func parseSearchResultEntry(data []byte) (*ldapEntry, bool, error) {
	// SearchResultEntry ::= [APPLICATION 4] SEQUENCE {
	//     objectName LDAPDN,
	//     attributes PartialAttributeList }
	// PartialAttributeList ::= SEQUENCE OF PartialAttribute
	// PartialAttribute ::= SEQUENCE {
	//     type AttributeDescription,
	//     vals SET OF AttributeValue }

	entry := &ldapEntry{
		attributes: make(map[string][]string),
	}

	// Parse objectName (DN)
	var dnVal asn1.RawValue
	rest, err := asn1.Unmarshal(data, &dnVal)
	if err != nil {
		return nil, false, err
	}
	entry.dn = string(dnVal.Bytes)

	// Parse attributes SEQUENCE
	var attrsSeq asn1.RawValue
	_, err = asn1.Unmarshal(rest, &attrsSeq)
	if err != nil {
		return entry, false, nil // Return entry with DN at least
	}

	attrData := attrsSeq.Bytes
	for len(attrData) > 0 {
		var attrSeq asn1.RawValue
		attrData, err = asn1.Unmarshal(attrData, &attrSeq)
		if err != nil {
			break
		}

		// Parse attribute type
		innerAttr := attrSeq.Bytes
		var attrType asn1.RawValue
		innerAttr, err = asn1.Unmarshal(innerAttr, &attrType)
		if err != nil {
			continue
		}
		attrName := strings.ToLower(string(attrType.Bytes))

		// Parse attribute values (SET OF)
		var valsSet asn1.RawValue
		_, err = asn1.Unmarshal(innerAttr, &valsSet)
		if err != nil {
			continue
		}

		valData := valsSet.Bytes
		var vals []string
		for len(valData) > 0 {
			var val asn1.RawValue
			valData, err = asn1.Unmarshal(valData, &val)
			if err != nil {
				break
			}
			if utf8.Valid(val.Bytes) {
				vals = append(vals, string(val.Bytes))
			}
		}
		entry.attributes[attrName] = vals
	}

	return entry, false, nil
}

// ldapEscapeFilter escapes special characters in an LDAP filter value to prevent
// LDAP injection attacks per RFC 4515.
func ldapEscapeFilter(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			buf.WriteString("\\5c")
		case '*':
			buf.WriteString("\\2a")
		case '(':
			buf.WriteString("\\28")
		case ')':
			buf.WriteString("\\29")
		case '\x00':
			buf.WriteString("\\00")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// ---------------------------------------------------------------------------
// BER encoding helpers for LDAP protocol
// ---------------------------------------------------------------------------

func berEncodeLength(length int) []byte {
	if length < 0x80 {
		return []byte{byte(length)}
	}
	// Multi-byte length
	var lenBytes []byte
	l := length
	for l > 0 {
		lenBytes = append([]byte{byte(l & 0xff)}, lenBytes...)
		l >>= 8
	}
	return append([]byte{byte(0x80 | len(lenBytes))}, lenBytes...)
}

func berEncodeSequence(content []byte) []byte {
	return append([]byte{0x30}, append(berEncodeLength(len(content)), content...)...)
}

func berEncodeInteger(val int) []byte {
	b, _ := asn1.Marshal(val)
	return b
}

func berEncodeEnumerated(val int) []byte {
	content := []byte{byte(val)}
	return append([]byte{0x0a}, append(berEncodeLength(len(content)), content...)...)
}

func berEncodeBoolean(val bool) []byte {
	v := byte(0x00)
	if val {
		v = 0xff
	}
	return []byte{0x01, 0x01, v}
}

func berEncodeOctetString(data []byte) []byte {
	return append([]byte{0x04}, append(berEncodeLength(len(data)), data...)...)
}

func berEncodeContextSpecific(tag int, data []byte) []byte {
	tagByte := byte(0x80 | tag)
	return append([]byte{tagByte}, append(berEncodeLength(len(data)), data...)...)
}

func berEncodeApplication(tag int, content []byte) []byte {
	tagByte := byte(0x60 | tag) // CONSTRUCTED | APPLICATION
	return append([]byte{tagByte}, append(berEncodeLength(len(content)), content...)...)
}

func berDecodeLength(data []byte) (int, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("empty length data")
	}
	if data[0] < 0x80 {
		return int(data[0]), 1, nil
	}
	numBytes := int(data[0] & 0x7f)
	if numBytes == 0 || numBytes > 4 || len(data) < numBytes+1 {
		return 0, 0, fmt.Errorf("invalid BER length encoding")
	}
	length := 0
	for i := 1; i <= numBytes; i++ {
		length = (length << 8) | int(data[i])
	}
	return length, numBytes + 1, nil
}

// encodeLDAPFilter encodes a simple LDAP filter string into BER.
// Supports: equality (attr=value), presence (attr=*), and simple AND/OR/NOT.
func encodeLDAPFilter(filter string) []byte {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		// Present filter for objectClass (match all)
		return berEncodeContextSpecific(7, []byte("objectClass"))
	}

	// Strip outer parentheses
	if strings.HasPrefix(filter, "(") && strings.HasSuffix(filter, ")") {
		inner := filter[1 : len(filter)-1]
		// Check if this is a compound filter
		if strings.HasPrefix(inner, "&") {
			return encodeLDAPCompoundFilter(0, inner[1:]) // AND
		}
		if strings.HasPrefix(inner, "|") {
			return encodeLDAPCompoundFilter(1, inner[1:]) // OR
		}
		if strings.HasPrefix(inner, "!") {
			child := encodeLDAPFilter(inner[1:])
			return append([]byte{0xa2}, append(berEncodeLength(len(child)), child...)...)
		}
		filter = inner
	}

	// Simple equality or presence filter
	eqIdx := strings.Index(filter, "=")
	if eqIdx < 0 {
		// Treat as presence filter
		return berEncodeContextSpecific(7, []byte(filter))
	}

	attr := filter[:eqIdx]
	value := filter[eqIdx+1:]

	if value == "*" {
		// Present filter: context [7]
		return berEncodeContextSpecific(7, []byte(attr))
	}

	// Check for substring filter (contains *)
	if strings.Contains(value, "*") {
		return encodeLDAPSubstringFilter(attr, value)
	}

	// Equality match: context [3] SEQUENCE { attributeDesc, assertionValue }
	content := append(berEncodeOctetString([]byte(attr)), berEncodeOctetString([]byte(value))...)
	return append([]byte{0xa3}, append(berEncodeLength(len(content)), content...)...)
}

// encodeLDAPCompoundFilter encodes AND (tag 0) or OR (tag 1) compound filters.
func encodeLDAPCompoundFilter(tag int, filterStr string) []byte {
	// Parse child filters - they should each be in parentheses
	var children []byte
	remaining := strings.TrimSpace(filterStr)
	for len(remaining) > 0 {
		if remaining[0] != '(' {
			break
		}
		depth := 0
		end := 0
		for i, ch := range remaining {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
		if end == 0 {
			break
		}
		child := encodeLDAPFilter(remaining[:end])
		children = append(children, child...)
		remaining = strings.TrimSpace(remaining[end:])
	}

	tagByte := byte(0xa0 | tag) // CONSTRUCTED | CONTEXT-SPECIFIC
	return append([]byte{tagByte}, append(berEncodeLength(len(children)), children...)...)
}

// encodeLDAPSubstringFilter encodes a substring filter (e.g., cn=*john*).
func encodeLDAPSubstringFilter(attr, value string) []byte {
	parts := strings.Split(value, "*")
	var substrings []byte

	for i, part := range parts {
		if part == "" {
			continue
		}
		var tag byte
		if i == 0 {
			tag = 0x80 // initial [0]
		} else if i == len(parts)-1 {
			tag = 0x82 // final [2]
		} else {
			tag = 0x81 // any [1]
		}
		substrings = append(substrings, tag)
		substrings = append(substrings, berEncodeLength(len(part))...)
		substrings = append(substrings, []byte(part)...)
	}

	subsSeq := berEncodeSequence(substrings)
	content := append(berEncodeOctetString([]byte(attr)), subsSeq...)
	// Substrings filter: context [4]
	return append([]byte{0xa4}, append(berEncodeLength(len(content)), content...)...)
}

// encodeLDAPAttributes encodes the attribute list for a search request.
func encodeLDAPAttributes(attrs []string) []byte {
	var content []byte
	for _, attr := range attrs {
		content = append(content, berEncodeOctetString([]byte(attr))...)
	}
	return berEncodeSequence(content)
}

// concat is a helper that concatenates multiple byte slices.
func concat(slices ...[]byte) []byte {
	var result []byte
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}
