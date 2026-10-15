package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service/settings"
)

// Account-admin settings: the Settings Engine keys a tenant admin may set
// for their own account (settings.Definition.AccountAdminEditable; today
// only pages.custom_css_enabled). The super-admin surface
// (/superadmin/settings) stays the place for every other key and scope.
//
//	GET /api/v1/accounts/:account_id/settings
//	PUT /api/v1/accounts/:account_id/settings/:key   { "value": true | false | null }
//
// Both sit behind RequireAdmin (tenant admins for their own account,
// super-admins for any). A key that isn't account-admin editable is 404,
// like an unknown one. PUT null clears the account's own row (inherit).
// Turning an "off anywhere above wins" key on while the instance or a
// parent account has it off is 422. Writes go through Set / Clear, which
// emit the setting_change audit row with the admin's user id.

// AccountSettingsStore is the Settings Engine surface the handler uses
// (*settings.Service).
type AccountSettingsStore interface {
	ViewForAccount(ctx context.Context, key string, accountID uint) (settings.AccountView, error)
	SetForAccount(ctx context.Context, accountID uint, key string, value *string, byUserID uint) error
}

// AccountFinder loads an account by id (repository.AccountRepository).
type AccountFinder interface {
	FindByID(ctx context.Context, id uint) (*models.Account, error)
}

// AccountSettingsHandler serves the account-admin settings endpoints.
type AccountSettingsHandler struct {
	store    AccountSettingsStore
	accounts AccountFinder
}

// NewAccountSettingsHandler builds the handler.
func NewAccountSettingsHandler(store AccountSettingsStore, accounts AccountFinder) *AccountSettingsHandler {
	return &AccountSettingsHandler{store: store, accounts: accounts}
}

// accountSettingJSON is one key as the account admin sees it.
type accountSettingJSON struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Value       interface{}       `json:"value"`     // this account's own value; null = inherits
	Effective   interface{}       `json:"effective"` // what applies to this account
	Locked      bool              `json:"locked"`
	LockedBy    *settingsLockJSON `json:"locked_by"`
}

// settingsLockJSON names the level that holds a setting off above the
// account: the instance ("your Paper administrator") or an ancestor.
type settingsLockJSON struct {
	Scope       string `json:"scope"`
	AccountID   *uint  `json:"account_id,omitempty"`
	AccountName string `json:"account_name,omitempty"`
}

// settingsAccount resolves :account_id to an account the caller may
// administer. wrote=true means a response (400/404) was written.
//
// Today that is the caller's own account (a super-admin: any existing
// account), the same rule RequireAdmin applied before the handler ran.
// When sub-accounts arrive, a parent-account admin acting on a descendant
// needs only this check widened (the caller's account or one of its
// ancestors); the reads and writes below already work on any account.
func (h *AccountSettingsHandler) settingsAccount(c *fiber.Ctx) (*models.Account, bool) {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil, true
	}
	if h.accounts == nil {
		return &models.Account{ID: accountID}, false
	}
	acct, err := h.accounts.FindByID(c.Context(), accountID)
	if err != nil || acct == nil {
		_ = responses.NotFound(c, "account")
		return nil, true
	}
	return acct, false
}

// editableSetting returns the account-admin editable definition for key.
func editableSetting(key string) (settings.Definition, bool) {
	for _, d := range settings.AccountAdminKeys() {
		if d.Key == key {
			return d, true
		}
	}
	return settings.Definition{}, false
}

// List handles GET /accounts/:account_id/settings.
func (h *AccountSettingsHandler) List(c *fiber.Ctx) error {
	acct, wrote := h.settingsAccount(c)
	if wrote {
		return nil
	}
	defs := settings.AccountAdminKeys()
	out := make([]accountSettingJSON, 0, len(defs))
	for _, def := range defs {
		item, err := h.render(c.Context(), def.Key, acct.ID)
		if err != nil {
			return responses.InternalError(c, "Could not load settings")
		}
		out = append(out, item)
	}
	return c.JSON(out)
}

// Update handles PUT /accounts/:account_id/settings/:key.
func (h *AccountSettingsHandler) Update(c *fiber.Ctx) error {
	acct, wrote := h.settingsAccount(c)
	if wrote {
		return nil
	}
	def, ok := editableSetting(c.Params("key"))
	if !ok {
		return responses.NotFound(c, "setting")
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return responses.BadRequest(c, "Invalid JSON body")
	}
	raw, present := body["value"]
	if !present {
		return responses.BadRequest(c, "value is required (null to inherit)")
	}
	value, err := settingValueFromJSON(def, raw)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	byUser, _ := c.Locals("user_id").(uint)
	if err := h.store.SetForAccount(c.Context(), acct.ID, def.Key, value, byUser); err != nil {
		var locked *settings.LockedError
		if errors.As(err, &locked) {
			return responses.Error(c, fiber.StatusUnprocessableEntity, h.lockMessage(c.Context(), locked.By))
		}
		return mapSettingsServiceError(c, err)
	}

	item, err := h.render(c.Context(), def.Key, acct.ID)
	if err != nil {
		return responses.InternalError(c, "Could not load setting")
	}
	return c.JSON(item)
}

func (h *AccountSettingsHandler) render(ctx context.Context, key string, accountID uint) (accountSettingJSON, error) {
	view, err := h.store.ViewForAccount(ctx, key, accountID)
	if err != nil {
		return accountSettingJSON{}, err
	}
	def := view.Def
	item := accountSettingJSON{
		Key:         def.Key,
		Label:       def.Label,
		Description: def.Description,
		Locked:      view.LockedBy != nil,
	}
	if view.Own != nil {
		item.Value = typedSettingValue(def, view.Own.Value)
	}
	if view.Effective.HasValue {
		item.Effective = typedSettingValue(def, view.Effective.Value)
	}
	if view.LockedBy != nil {
		item.LockedBy = h.lockJSON(ctx, *view.LockedBy)
	}
	return item, nil
}

func (h *AccountSettingsHandler) lockJSON(ctx context.Context, l settings.Level) *settingsLockJSON {
	if l.Scope != settings.ScopeAccount {
		return &settingsLockJSON{Scope: string(settings.ScopeInstance)}
	}
	id := l.ScopeID
	out := &settingsLockJSON{Scope: string(settings.ScopeAccount), AccountID: &id}
	if h.accounts != nil {
		if a, err := h.accounts.FindByID(ctx, id); err == nil && a != nil {
			out.AccountName = a.Name
		}
	}
	return out
}

// lockMessage is the 422 text for turning a setting on under a lock.
func (h *AccountSettingsHandler) lockMessage(ctx context.Context, l settings.Level) string {
	lock := h.lockJSON(ctx, l)
	if lock.Scope == string(settings.ScopeAccount) && lock.AccountName != "" {
		return "Turned off by " + lock.AccountName + ". This account can't turn it back on."
	}
	if lock.Scope == string(settings.ScopeAccount) {
		return "Turned off by a parent account. This account can't turn it back on."
	}
	return "Turned off by your Paper administrator. This account can't turn it back on."
}

// typedSettingValue renders a stored value as its JSON type. Secrets are
// never account-admin editable (enforced by a catalog test) and render as
// null here regardless.
func typedSettingValue(def settings.Definition, v string) interface{} {
	switch def.ValueType {
	case settings.TypeSecret:
		return nil
	case settings.TypeBool:
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil
		}
		return b
	case settings.TypeInt:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return nil
		}
		return n
	case settings.TypeJSON:
		if json.Valid([]byte(v)) {
			return json.RawMessage(v)
		}
		return nil
	default:
		return v
	}
}

// settingValueFromJSON turns the request's "value" into the stored string
// (nil for JSON null: clear the account's row). The JSON type must match
// the key's type: a bool key takes true/false, an int key a whole number,
// a string key a string.
func settingValueFromJSON(def settings.Definition, raw json.RawMessage) (*string, error) {
	if strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var s string
	switch def.ValueType {
	case settings.TypeBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, errors.New("value must be true, false or null")
		}
		s = strconv.FormatBool(b)
	case settings.TypeInt:
		var n int
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, errors.New("value must be a whole number or null")
		}
		s = strconv.Itoa(n)
	case settings.TypeJSON:
		s = string(raw)
	case settings.TypeString:
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, errors.New("value must be a string or null")
		}
	default:
		return nil, errors.New("this setting can't be set here")
	}
	return &s, nil
}
