package auth

import (
	"context"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// resolveProviderLogin matches an SSO outcome against the admin-created
// logins of the provider's account (G115). The subject is tried first,
// then the provider's "login_id" attribute (LDAP asserts the DN as the
// subject but the uid as login_id). A match binds a federated identity so
// later sign-ins take the federated path directly.
//
// ok=false means no login matched and resolution continues with the
// email auto-link.
func (p *LoginPipeline) resolveProviderLogin(ctx context.Context, outcome SSOOutcome, providerID, accountID uint, meta RequestMeta) (*models.User, bool, error) {
	if p.logins == nil || accountID == 0 {
		return nil, false, nil
	}
	candidates := []string{outcome.ExternalSubject}
	if v, ok := outcome.Attributes["login_id"].(string); ok && v != "" && v != outcome.ExternalSubject {
		candidates = append(candidates, v)
	}
	var login *models.UserLogin
	for _, subject := range candidates {
		if subject == "" {
			continue
		}
		if l, err := p.logins.FindForProvider(ctx, providerID, accountID, subject); err == nil && l != nil {
			login = l
			break
		}
	}
	if login == nil {
		return nil, false, nil
	}
	// AUTH-INTERNAL: tenant checked explicitly below against the provider.
	user, err := p.users.FindByID(ctx, login.UserID, 0)
	if err != nil || user == nil {
		return nil, false, nil
	}
	if user.AccountID != accountID {
		return nil, false, ErrCrossTenantIdentity
	}
	// A platform super_admin is never signed in through an account-level
	// IdP mapping; link it explicitly (same rule as the email auto-link).
	if user.Role == "super_admin" {
		return nil, false, ErrPrivilegedAutoLink
	}
	if outcome.ExternalSubject != "" {
		fi := &models.FederatedIdentity{
			UserID:          user.ID,
			ProviderID:      providerID,
			ExternalSubject: outcome.ExternalSubject,
			ClaimsSnapshot:  marshalAttrs(outcome.Attributes),
		}
		if err := p.federations.Create(ctx, fi); err != nil {
			return nil, false, fmt.Errorf("create federation binding for login: %w", err)
		}
		p.audit.AccountLinkedViaFederation(ctx, user.ID, outcome.ProviderType, providerID, meta)
	}
	p.logins.TouchLastLogin(ctx, login.ID)
	return user, true, nil
}
