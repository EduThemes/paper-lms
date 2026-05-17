package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ── Errors ──────────────────────────────────────────────────────────

// ErrUnknownKey is returned for any read or write against a key that is
// not declared in the catalog. The service is strict: stringly-typed
// callers can't accidentally write to "smpt.host" and discover the typo
// at the next deploy.
var ErrUnknownKey = errors.New("settings: unknown key")

// ErrScopeNotAllowed is returned when a write targets a scope the
// catalog entry doesn't list. E.g. trying to set storage.backend at
// account scope when the catalog declares it instance-only.
var ErrScopeNotAllowed = errors.New("settings: scope not allowed for key")

// ErrInvalidValue is returned when a value fails the type validator
// (e.g. "abc" for ValueType=int, "yeah" for ValueType=bool).
var ErrInvalidValue = errors.New("settings: invalid value for type")

// ── Public shapes ──────────────────────────────────────────────────

// Source labels where in the resolution chain a value came from. Drives
// the "Source" column in the super-admin UI so an operator can see
// whether a blank field means "no override" vs "explicitly cleared".
type Source string

const (
	SourceUser     Source = "user"
	SourceAccount  Source = "account"
	SourceInstance Source = "instance"
	SourceEnv      Source = "env"
	SourceDefault  Source = "default"
	SourceNone     Source = "none"
)

// ScopeHints carry the caller-context the service uses to walk the
// resolution chain. UserID and AccountID are zero when not applicable
// (e.g. a server-side consumer asking for an instance-only setting
// passes both as zero and the chain skips the user/account scopes).
type ScopeHints struct {
	UserID    uint
	AccountID uint
}

// EffectiveValue is the resolved view of a single setting at the
// requested scope. Value is the decrypted plaintext when IsSecret is
// true — the API surface MUST call Mask() before serializing, but
// server-side consumers (smtp_service, ai_assist_service, etc.) read
// the plaintext directly.
//
// HasValue distinguishes "the chain produced something" from "no env,
// no default, nothing set anywhere" — handlers render the latter as
// "Unset" rather than as the empty string.
type EffectiveValue struct {
	Key       string
	Value     string
	HasValue  bool
	IsSecret  bool
	Source    Source
	ScopeID   uint
	UpdatedAt *time.Time
	UpdatedBy *uint
}

// Mask returns a copy with the plaintext stripped for secret values.
// Wave 2's read API uses this before serializing; non-secret values
// pass through unchanged.
func (e EffectiveValue) Mask() EffectiveValue {
	if e.IsSecret {
		e.Value = ""
	}
	return e
}

// ── Service ────────────────────────────────────────────────────────

// AuditSink is the minimal audit-emission surface the settings service
// needs. *service.AuditService satisfies this structurally so the wider
// AuditService can be injected as-is at boot. Nil is permitted —
// audit-log emission becomes a no-op, matching audit_service.go's own
// "no-op when wired with a nil repo" contract.
type AuditSink interface {
	LogEvent(ctx context.Context, eventType string, userID uint, courseID, accountID *uint, contextType string, contextID uint, action, payload, ipAddress, userAgent string) error
}

// AccountAncestry walks the parent_account_id chain. A narrow interface
// rather than depending on repository.AccountRepository directly so
// tests can stub the walk without standing up the whole user/account
// mock surface.
type AccountAncestry interface {
	FindByID(ctx context.Context, id uint) (*models.Account, error)
}

// Service is the single read/write surface for the settings store.
type Service struct {
	repo     repository.SettingRepository
	ancestry AccountAncestry
	audit    AuditSink
	getEnv   func(string) string

	// maxAncestryDepth caps the account parent-chain walk so a
	// circular parent_account_id (operator-induced bug, not a normal
	// state) can't infinite-loop a setting read. Three levels matches
	// the canonical district → school → sub-school depth that the
	// plan calls out.
	maxAncestryDepth int
}

// NewService constructs the settings service. audit may be nil
// (audit-log emission becomes a no-op); ancestry must be non-nil
// when any catalog entry permits ScopeAccount.
func NewService(repo repository.SettingRepository, ancestry AccountAncestry, audit AuditSink) *Service {
	return &Service{
		repo:             repo,
		ancestry:         ancestry,
		audit:            audit,
		getEnv:           os.Getenv,
		maxAncestryDepth: 8,
	}
}

// SetEnvReader overrides the env-var reader. Tests use this to inject
// a deterministic map without mutating process env.
func (s *Service) SetEnvReader(f func(string) string) {
	if f != nil {
		s.getEnv = f
	}
}

// ── Reads ──────────────────────────────────────────────────────────

// Get walks the resolution chain and returns the highest-priority
// value bound to key. Returns ErrUnknownKey if key is not in the
// catalog. The chain:
//
//	user → account → ...parent accounts → instance → env → default
//
// Each step is skipped when the catalog doesn't permit it or the
// hint is zero. The service returns plaintext for secret values —
// callers crossing the API boundary MUST call Mask() before
// serializing.
func (s *Service) Get(ctx context.Context, key string, hints ScopeHints) (EffectiveValue, error) {
	def, ok := Find(key)
	if !ok {
		return EffectiveValue{}, fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}

	// 1. User scope.
	if def.AllowsScope(ScopeUser) && hints.UserID != 0 {
		if ev, found, err := s.readScope(ctx, def, ScopeUser, hints.UserID); err != nil {
			return EffectiveValue{}, err
		} else if found {
			return ev, nil
		}
	}

	// 2. Account scope — walk the parent chain to root.
	if def.AllowsScope(ScopeAccount) && hints.AccountID != 0 {
		ev, found, err := s.walkAccountChain(ctx, def, hints.AccountID)
		if err != nil {
			return EffectiveValue{}, err
		}
		if found {
			return ev, nil
		}
	}

	// 3. Instance scope.
	if def.AllowsScope(ScopeInstance) {
		if ev, found, err := s.readScope(ctx, def, ScopeInstance, 0); err != nil {
			return EffectiveValue{}, err
		} else if found {
			return ev, nil
		}
	}

	// 4. Env-var fallback.
	if def.EnvFallback != "" {
		if envVal := s.getEnv(def.EnvFallback); envVal != "" {
			return EffectiveValue{
				Key:      def.Key,
				Value:    envVal,
				HasValue: true,
				IsSecret: def.IsSecret(),
				Source:   SourceEnv,
			}, nil
		}
	}

	// 5. Hard-coded default.
	if def.Default != "" {
		return EffectiveValue{
			Key:      def.Key,
			Value:    def.Default,
			HasValue: true,
			IsSecret: def.IsSecret(),
			Source:   SourceDefault,
		}, nil
	}

	// 6. Nothing in the chain.
	return EffectiveValue{
		Key:      def.Key,
		IsSecret: def.IsSecret(),
		Source:   SourceNone,
		HasValue: false,
	}, nil
}

// GetEffective returns every setting in the given UI group, each
// resolved through the chain. Empty group returns the whole catalog.
// Wave 2's read API serializes this (masking secrets) as the per-group
// editor payload.
func (s *Service) GetEffective(ctx context.Context, group string, hints ScopeHints) (map[string]EffectiveValue, error) {
	out := make(map[string]EffectiveValue, len(Catalog))
	for _, def := range Catalog {
		if group != "" && def.Group != group {
			continue
		}
		ev, err := s.Get(ctx, def.Key, hints)
		if err != nil {
			return nil, err
		}
		out[def.Key] = ev
	}
	return out, nil
}

// readScope fetches one explicit row at (scope, scopeID, key) and
// hydrates it into an EffectiveValue. found=false (with nil err) means
// "no row exists" — the chain continues. Decryption errors are real
// errors and abort the walk.
func (s *Service) readScope(ctx context.Context, def Definition, scope ScopeType, scopeID uint) (EffectiveValue, bool, error) {
	row, err := s.repo.FindByScope(ctx, string(scope), scopeID, def.Key)
	if errors.Is(err, repository.ErrSettingNotFound) {
		return EffectiveValue{}, false, nil
	}
	if err != nil {
		return EffectiveValue{}, false, err
	}

	val, err := decodeRow(row, def)
	if err != nil {
		return EffectiveValue{}, false, err
	}

	src := SourceInstance
	switch scope {
	case ScopeUser:
		src = SourceUser
	case ScopeAccount:
		src = SourceAccount
	}

	updatedAt := row.UpdatedAt
	return EffectiveValue{
		Key:       def.Key,
		Value:     val,
		HasValue:  true,
		IsSecret:  def.IsSecret(),
		Source:    src,
		ScopeID:   scopeID,
		UpdatedAt: &updatedAt,
		UpdatedBy: row.UpdatedBy,
	}, true, nil
}

// walkAccountChain starts at accountID and follows parent_account_id
// to the root, returning the first explicit setting it finds. Capped
// by maxAncestryDepth so a self-referential parent (operator bug)
// can't loop forever.
func (s *Service) walkAccountChain(ctx context.Context, def Definition, accountID uint) (EffectiveValue, bool, error) {
	current := accountID
	seen := map[uint]struct{}{}
	for depth := 0; depth < s.maxAncestryDepth && current != 0; depth++ {
		if _, loop := seen[current]; loop {
			break
		}
		seen[current] = struct{}{}

		if ev, found, err := s.readScope(ctx, def, ScopeAccount, current); err != nil {
			return EffectiveValue{}, false, err
		} else if found {
			return ev, true, nil
		}

		if s.ancestry == nil {
			break
		}
		acct, err := s.ancestry.FindByID(ctx, current)
		if err != nil {
			// A missing account row aborts the walk silently — the
			// instance/env/default fallback still runs. Treating this
			// as a hard error would mean a stale parent_account_id
			// (rare but possible after a tenant delete) breaks reads
			// for every setting, which is worse than the alternative.
			return EffectiveValue{}, false, nil
		}
		if acct.ParentAccountID == nil {
			break
		}
		current = *acct.ParentAccountID
	}
	return EffectiveValue{}, false, nil
}

// decodeRow extracts the storage value from a setting row. Secret
// rows go through auth.Decrypt; non-secret rows read ValuePlain.
func decodeRow(row *models.Setting, def Definition) (string, error) {
	if def.IsSecret() {
		if len(row.ValueEncrypted) == 0 {
			return "", errors.New("settings: secret row missing ciphertext")
		}
		pt, err := auth.Decrypt(row.ValueEncrypted)
		if err != nil {
			return "", fmt.Errorf("settings: decrypt %q: %w", def.Key, err)
		}
		return string(pt), nil
	}
	if row.ValuePlain == nil {
		return "", errors.New("settings: non-secret row missing value_plain")
	}
	return *row.ValuePlain, nil
}

// ── Writes ──────────────────────────────────────────────────────────

// Set upserts the value for (scope, scopeID, key). Secret-typed
// catalog entries route through auth.Encrypt before storage; plaintext
// never lands on disk. Every successful Set emits an audit_log row;
// the payload describes the scope and value type but NEVER the value.
//
// byUserID identifies the platform operator making the change — it
// stamps both the setting row's updated_by and the audit_log row's
// user_id.
func (s *Service) Set(ctx context.Context, scope ScopeType, scopeID uint, key, value string, byUserID uint) error {
	def, ok := Find(key)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	if !def.AllowsScope(scope) {
		return fmt.Errorf("%w: %q at %s", ErrScopeNotAllowed, key, scope)
	}
	if scope == ScopeInstance && scopeID != 0 {
		return fmt.Errorf("%w: instance scope requires scope_id=0", ErrScopeNotAllowed)
	}
	if scope != ScopeInstance && scopeID == 0 {
		return fmt.Errorf("%w: %s scope requires a non-zero scope_id", ErrScopeNotAllowed, scope)
	}

	if err := validateValue(def, value); err != nil {
		return err
	}

	// Catalog-level write-time validator (Wave 7 / Wave 6 audit H2).
	// Runs AFTER the type-coercion check so the validator can assume
	// `value` parses as the declared ValueType. The peer callback
	// resolves OTHER keys at the same scope+scope_id so a validator
	// can check cross-key invariants (e.g. RPID-must-be-suffix-of-
	// every-origin) without re-implementing the resolution chain.
	//
	// peer only returns OPERATOR-SET values — when the peer key is
	// resolving from env/default/none, peer returns "" (as if unset).
	// This means a validator on "first config of either key" doesn't
	// fight the catalog default. The general pattern:
	//
	//   if peer == "" { /* defer — operator hasn't chosen yet */ }
	//   else if value doesn't match peer { reject }
	//
	// Without this distinction, setting auth.passkey.rporigins on a
	// fresh deployment would always reject because the default
	// auth.passkey.rpid is "localhost" and no real origin would
	// have localhost as a registrable suffix.
	if def.Validate != nil {
		peerHints := ScopeHints{}
		if scope == ScopeAccount {
			peerHints.AccountID = scopeID
		}
		peer := func(peerKey string) (string, error) {
			ev, err := s.Get(ctx, peerKey, peerHints)
			if err != nil {
				return "", err
			}
			// Treat env/default/none sources as "unset" — operator
			// hasn't chosen this value yet, so coupling checks
			// against it would fight defaults. Validators that
			// genuinely want the resolved value (including fallback)
			// should call s.Get directly with the appropriate ctx.
			switch ev.Source {
			case SourceUser, SourceAccount, SourceInstance:
				return ev.Value, nil
			}
			return "", nil
		}
		if err := def.Validate(ctx, value, peer); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidValue, err)
		}
	}

	row := &models.Setting{
		ScopeType: string(scope),
		ScopeID:   scopeID,
		Key:       key,
		ValueType: string(def.ValueType),
	}
	if byUserID != 0 {
		uid := byUserID
		row.UpdatedBy = &uid
	}

	if def.IsSecret() {
		ct, err := auth.Encrypt([]byte(value))
		if err != nil {
			return fmt.Errorf("settings: encrypt %q: %w", key, err)
		}
		row.ValueEncrypted = ct
	} else {
		v := value
		row.ValuePlain = &v
	}

	if err := s.repo.Upsert(ctx, row); err != nil {
		return err
	}

	s.emitAudit(ctx, "setting.changed", byUserID, scope, scopeID, key, def, row.ID)
	return nil
}

// Clear removes the row at (scope, scopeID, key), causing future reads
// to fall through to the next scope. Idempotent — clearing a key that
// isn't explicitly set is not an error. Emits an audit_log row.
func (s *Service) Clear(ctx context.Context, scope ScopeType, scopeID uint, key string, byUserID uint) error {
	def, ok := Find(key)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	if !def.AllowsScope(scope) {
		return fmt.Errorf("%w: %q at %s", ErrScopeNotAllowed, key, scope)
	}
	if scope == ScopeInstance && scopeID != 0 {
		return fmt.Errorf("%w: instance scope requires scope_id=0", ErrScopeNotAllowed)
	}
	if scope != ScopeInstance && scopeID == 0 {
		return fmt.Errorf("%w: %s scope requires a non-zero scope_id", ErrScopeNotAllowed, scope)
	}

	if err := s.repo.Delete(ctx, string(scope), scopeID, key); err != nil {
		return err
	}

	s.emitAudit(ctx, "setting.cleared", byUserID, scope, scopeID, key, def, 0)
	return nil
}

// validateValue checks the freshly-set value coerces under the catalog
// type. Catalog ValueType drives the parse; failures return
// ErrInvalidValue so handlers can map to 400. JSON values must be
// valid JSON text. Secret values are unconstrained — any string is
// permitted (operators paste their own credentials).
func validateValue(def Definition, value string) error {
	switch def.ValueType {
	case TypeString, TypeSecret:
		return nil
	case TypeInt:
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("%w: %q is not an integer", ErrInvalidValue, value)
		}
	case TypeBool:
		v := strings.ToLower(strings.TrimSpace(value))
		if v != "true" && v != "false" {
			return fmt.Errorf("%w: %q is not 'true' or 'false'", ErrInvalidValue, value)
		}
	case TypeJSON:
		var any json.RawMessage
		if err := json.Unmarshal([]byte(value), &any); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidValue, err)
		}
	default:
		return fmt.Errorf("%w: unknown value type %q", ErrInvalidValue, def.ValueType)
	}
	return nil
}

// emitAudit forwards a setting.changed / setting.cleared row to the
// audit sink. Payload carries the scope + value_type so an operator
// reviewing the audit feed can see WHAT was changed at WHAT scope
// without revealing the value itself — non-secret values are also
// elided so a future env-var-promoted-to-setting like a JWT secret
// can't accidentally leak into the audit table.
func (s *Service) emitAudit(ctx context.Context, action string, userID uint, scope ScopeType, scopeID uint, key string, def Definition, settingID uint) {
	if s.audit == nil {
		return
	}

	payloadStruct := struct {
		Key       string `json:"key"`
		Scope     string `json:"scope"`
		ScopeID   uint   `json:"scope_id"`
		ValueType string `json:"value_type"`
		Group     string `json:"group"`
	}{
		Key:       key,
		Scope:     string(scope),
		ScopeID:   scopeID,
		ValueType: string(def.ValueType),
		Group:     def.Group,
	}
	payload, err := json.Marshal(payloadStruct)
	if err != nil {
		// A marshal failure for a struct of strings + uints is
		// essentially impossible; fall back to a non-empty payload so
		// the row still records.
		payload = []byte(`{"key":"` + key + `"}`)
	}

	var accountID *uint
	if scope == ScopeAccount {
		v := scopeID
		accountID = &v
	}

	_ = s.audit.LogEvent(
		ctx,
		"setting_change",
		userID,
		nil,
		accountID,
		"Setting",
		settingID,
		action,
		string(payload),
		"",
		"",
	)
}
