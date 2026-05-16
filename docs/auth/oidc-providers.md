# OIDC provider setup — Paper LMS admin guide

This document walks through configuring each supported OIDC identity provider against Paper LMS. The Paper LMS admin form lives at **Admin → Authentication Providers → Add Provider → OIDC**.

The Paper LMS callback URL is always:

```
{FRONTEND_URL}/api/v1/auth/oidc/callback
```

For local dev, that's `http://localhost:3000/api/v1/auth/oidc/callback`. Paste this verbatim into the IdP's "redirect URI" / "reply URL" field — most providers reject the login if it doesn't match byte-for-byte.

After each IdP-side step, return to Paper LMS and fill in the form fields shown in the "Paper LMS configuration" boxes.

---

## Google Workspace

**When to pick this**: K-12 / HigherEd schools running Google Workspace for Education, or any institution where users have @yourdomain Google accounts.

**Prerequisites**:
- A Google Cloud project with a billing/admin account.
- Workspace admin access if you want to restrict logins to one domain.

**Steps**:

1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials).
2. Click **Create Credentials → OAuth client ID**.
3. If prompted, configure the OAuth consent screen first:
   - User type: **Internal** if Workspace-only, otherwise **External**.
   - App name: "Paper LMS" (or your school's brand).
   - Scopes: `openid`, `email`, `profile` (the default selection).
4. Back at the Credentials page, pick **Web application** as the application type.
5. Under **Authorized redirect URIs**, add Paper LMS's callback:
   ```
   https://your-paper-lms.example.edu/api/v1/auth/oidc/callback
   ```
6. Click **Create**. Google shows you the **Client ID** and **Client secret**. Copy both.

**Paper LMS configuration**:

| Field | Value |
|---|---|
| Provider preset | **Google Workspace** |
| Issuer URL | `https://accounts.google.com` (pre-filled) |
| Client ID | (from step 6) |
| Client Secret | (from step 6) |
| Scopes | `openid email profile` |
| Auto-provision | On (first-time Workspace users create accounts) |

**Restricting to one Workspace domain**: Google's `hd` claim carries the user's domain. Paper LMS does not enforce this at the moment — the recommended pattern is to set the OAuth consent screen to **Internal** so non-domain users can't even authenticate.

---

## Microsoft Entra ID (formerly Azure AD)

**When to pick this**: Institutions running Microsoft 365 / Entra ID. Common in higher-ed, corporate-training, and many K-12 districts.

**Prerequisites**:
- Entra ID tenant admin access.
- The tenant's directory ID (UUID).

**Steps**:

1. Go to the [Microsoft Entra admin center](https://entra.microsoft.com/) → **Applications → App registrations → New registration**.
2. Name it "Paper LMS". For **Supported account types**, pick one:
   - **Single tenant** (most schools): users in your tenant only.
   - **Multitenant**: users from any organization (rare; usually wrong for a school LMS).
3. Under **Redirect URI**, choose **Web** and paste:
   ```
   https://your-paper-lms.example.edu/api/v1/auth/oidc/callback
   ```
4. Click **Register**. Note the **Application (client) ID** and the **Directory (tenant) ID** from the Overview page.
5. Go to **Certificates & secrets → New client secret**. Pick an expiry (Microsoft caps this at 24 months). Copy the **Value** column immediately — it's not shown again.
6. Go to **API permissions**. The defaults usually include `openid`, `email`, and `profile` under "Microsoft Graph delegated permissions". Add them explicitly if missing. No admin consent required for these three.

**Paper LMS configuration**:

| Field | Value |
|---|---|
| Provider preset | **Microsoft Entra ID** |
| Issuer URL | `https://login.microsoftonline.com/{TENANT_ID}/v2.0` — replace `{TENANT_ID}` with the directory ID from step 4 |
| Client ID | (from step 4) |
| Client Secret | (from step 5) |
| Scopes | `openid email profile` |
| Auto-provision | On for trusted tenants |

**Multi-tenant variation**: substitute `organizations` (any Microsoft tenant) or `common` (any tenant + personal accounts) for the tenant id segment. Be aware that multi-tenant requires admin consent flows the basic OIDC client doesn't trigger; usually you want single-tenant.

---

## Apple Sign-In

**When to pick this**: Institutions issuing personal Apple IDs (less common in K-12, more common in HigherEd and continuing-education contexts). Required by the App Store if you ever bundle Paper LMS as a native iOS app.

**Prerequisites**:
- A paid Apple Developer account ($99/year).
- A registered App ID.

**Steps**:

1. Sign in to [Apple Developer → Certificates, Identifiers & Profiles](https://developer.apple.com/account/resources/identifiers/list).
2. Create a **Services ID**:
   - Click **+ → Services IDs → Continue**.
   - Description: "Paper LMS Sign In".
   - Identifier: a reverse-DNS string like `edu.example.paper-lms.signin`.
   - Enable **Sign In with Apple**, then **Configure**:
     - Primary App ID: an existing App ID (create one first under **App IDs** if needed).
     - Domains and Subdomains: `your-paper-lms.example.edu`.
     - Return URLs: `https://your-paper-lms.example.edu/api/v1/auth/oidc/callback`.
   - Save.
3. Create a **Key** for the Services ID:
   - Click **Keys → + → Sign In with Apple → Configure**.
   - Pick the primary App ID from step 2.
   - Save, then download the `.p8` private key file. **Apple shows the key once** — store it in a password manager immediately.
   - Note the **Key ID** (10-char alphanumeric) and your **Team ID** (visible in the developer portal header).
4. **Generate the client secret JWT**. Unlike other OIDC providers, Apple's "client secret" is a short-lived (≤6 months) JWT signed with the `.p8` key. Generate it offline:

   ```bash
   # Replace TEAM_ID, KEY_ID, SERVICES_ID, and the path to your .p8.
   # Outputs a JWT valid for 6 months.
   go run ./cmd/apple-client-secret \
     --team=ABCD123456 \
     --key-id=ABC1234567 \
     --client-id=edu.example.paper-lms.signin \
     --p8=/path/to/AuthKey_ABC1234567.p8
   ```

   (If this CLI helper isn't shipped yet, generate the JWT manually with `jwt-cli` or the snippet in [Apple's reference docs](https://developer.apple.com/documentation/sign_in_with_apple/generate_and_validate_tokens).)
5. Set a calendar reminder to rotate the client secret before the JWT expires.

**Paper LMS configuration**:

| Field | Value |
|---|---|
| Provider preset | **Apple Sign-In** |
| Issuer URL | `https://appleid.apple.com` (pre-filled) |
| Client ID | The Services ID from step 2 (e.g. `edu.example.paper-lms.signin`) |
| Client Secret | The signed JWT from step 4 |
| Scopes | `openid email name` |
| Auto-provision | Usually on, with caveats below |

**Apple-specific quirks** (Paper LMS already handles these — no admin action required):
- Apple sends `email` and `name` claims **only on the first consent**. Subsequent logins omit them. Paper LMS captures them at first-login into `federated_identities.claims_snapshot` so re-login works without re-prompting.
- Apple supports a "Hide My Email" feature that issues a random `@privaterelay.appleid.com` address. The user can sign in that way perfectly fine; the relay forwards to their real address.
- The `email_verified` claim is always `true` for Apple-issued addresses but `false` for "hidden" relay addresses on the first login. Paper LMS auto-links only on `email_verified=true`, so hidden-email users will need to be pre-provisioned OR `auto_provision=true` on the provider.

---

## Generic OIDC (Authentik / Authelia / Keycloak / Zitadel / Okta / Auth0 / etc.)

**When to pick this**: Any IdP that speaks standards-compliant OpenID Connect. The four products in the heading are the most common self-hosted choices for schools/universities; the Auth0/Okta variants apply to commercial deployments.

The setup pattern is the same across all of them:

1. In the IdP admin console, create a new **OAuth/OIDC client** or **application**.
2. Pick the **Authorization Code flow** (also called "web app", "regular web app", or "server-side").
3. Set the **Redirect URI** / **Callback URL** to:
   ```
   https://your-paper-lms.example.edu/api/v1/auth/oidc/callback
   ```
4. Copy the resulting **Client ID** and **Client Secret**.
5. Find the IdP's **issuer URL** — this is the base URL of the OIDC discovery document. Test it:
   ```bash
   curl https://idp.example.com/.well-known/openid-configuration
   ```
   If you get a JSON response with `authorization_endpoint` and `token_endpoint`, that's the right URL. (For Keycloak it's typically `https://keycloak.example.com/realms/your-realm`; for Authentik it's `https://authentik.example.com/application/o/your-app/`; for Zitadel it's `https://zitadel-instance.cloud`.)

**Paper LMS configuration**:

| Field | Value |
|---|---|
| Provider preset | **Generic OIDC** |
| Issuer URL | The discovery base from step 5 |
| Client ID | (from step 4) |
| Client Secret | (from step 4) |
| Scopes | `openid email profile` (most IdPs accept this verbatim; adjust if your IdP exposes role/group scopes you want to claim) |
| Auto-provision | Your call — usually **on** for self-hosted IdPs (the directory is the source of truth) |

### Product-specific tips

**Authentik**: under the Application's *Edit → Provider* settings, ensure the *Client type* is **Confidential** (so a secret is generated) and that the *Signing Key* is set. Default scopes already include `openid email profile`.

**Authelia**: configure the client in `configuration.yml` under `identity_providers.oidc.clients`. Use `client_secret: '$argon2id$...'` (Argon2 hashed) and `grant_types: ['authorization_code']`. The issuer URL is your Authelia base URL.

**Keycloak**: in the realm console, **Clients → Create** with *Client Protocol = openid-connect*. *Access Type = confidential*. Add the redirect URI under *Valid Redirect URIs*. The client secret appears under the *Credentials* tab after save.

**Zitadel**: create a project → application → "Web" type → "Code" auth flow. Zitadel issues the secret once after creation — copy it immediately.

**Okta**: applications dashboard → *Create App Integration* → "OIDC - OpenID Connect" → "Web Application". Standard.

**Auth0**: applications → *Create Application* → "Regular Web Applications". Allowed Callback URL = the Paper LMS callback.

---

## Security checklist (every provider)

Run through this list once each provider is wired:

- [ ] Redirect URI matches the deployment domain exactly (no trailing slash, no path drift, scheme matches).
- [ ] Client secret is rotated annually (or on the IdP's schedule for shorter-lived secrets like Apple's 6-month JWT).
- [ ] Auto-provision is **off** for IdPs that include users you don't want to grant Paper LMS access (e.g. a multi-tenant IdP that includes external partners).
- [ ] If users from multiple email domains can sign in via the same IdP, decide whether your `mfa_policy` should be tightened (set `accounts.mfa_policy = required_admin` or `required_all`).
- [ ] After a successful login via the new IdP, verify:
  - `federated_identities` has a fresh row for `(provider_id, external_subject)`.
  - The user can log out and log back in via the same provider (path (a) — federation lookup — should hit; no second `auth.account_linked` audit event).

---

## Troubleshooting

**"invalid_grant" / "redirect_uri_mismatch"**: The redirect URI the IdP recorded does not match what Paper LMS sends. Paper LMS always sends `{FRONTEND_URL}/api/v1/auth/oidc/callback`. Check the FRONTEND_URL env var and the IdP's allowed-redirect setting.

**"id_token missing 'sub' claim"**: The IdP returned an id_token without `sub`. Standards-compliant IdPs always set this — if you're seeing it, the IdP is misconfigured (possibly using JWT-bearer instead of id_token; check the auth flow type).

**"could not parse id_token claims"**: The id_token signature couldn't be verified against the IdP's JWKS. Usually means the issuer URL is wrong (Paper LMS fetches `{issuer}/.well-known/openid-configuration`, then `jwks_uri` from that doc). Confirm the discovery URL responds.

**User signs in but no account is created**: The provider has `auto_provision=false` AND no existing Paper user matched by `email_verified=true` email auto-link. Either flip auto-provision on, or pre-create the user with the matching email.

**Apple users sign in but show as the relay email**: This is expected — Apple's "Hide My Email" feature. The relay email is the user's account identifier on Paper. They can change to a non-relay email later via the user profile page (which is a per-user opt-out of Apple's privacy feature).
