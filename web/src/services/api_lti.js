// LTI 1.3 / Advantage API calls (LTI parity lane, G123-G134). Kept out of
// api.js so the lane does not grow the shared module; uses its request
// helper (CSRF header, ApiError, Link pagination).

import { request } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const json = (method, body) => ({ method, body: JSON.stringify(body) });

// Placements a tool configuration may declare, with the message type each
// defaults to (selection placements launch deep linking).
export const LTI_PLACEMENTS = [
  { value: 'course_navigation', label: 'Course navigation', messageType: 'LtiResourceLinkRequest' },
  { value: 'editor_button', label: 'Rich content editor button', messageType: 'LtiDeepLinkingRequest' },
  { value: 'assignment_selection', label: 'Assignment selection', messageType: 'LtiDeepLinkingRequest' },
  { value: 'link_selection', label: 'Module link selection', messageType: 'LtiDeepLinkingRequest' },
  { value: 'module_index_menu', label: 'Modules menu', messageType: 'LtiDeepLinkingRequest' },
  { value: 'account_navigation', label: 'Account navigation', messageType: 'LtiResourceLinkRequest' },
];

export const LTI_SCOPES = [
  { value: 'https://purl.imsglobal.org/spec/lti-ags/scope/lineitem', label: 'AGS: manage line items' },
  { value: 'https://purl.imsglobal.org/spec/lti-ags/scope/lineitem.readonly', label: 'AGS: read line items' },
  { value: 'https://purl.imsglobal.org/spec/lti-ags/scope/result.readonly', label: 'AGS: read results' },
  { value: 'https://purl.imsglobal.org/spec/lti-ags/scope/score', label: 'AGS: post scores' },
  { value: 'https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly', label: 'NRPS: read course roster' },
];

export const LTI_PRIVACY_LEVELS = [
  { value: 'anonymous', label: 'Anonymous (no name or email)' },
  { value: 'name_only', label: 'Name only' },
  { value: 'email_only', label: 'Email only' },
  { value: 'public', label: 'Public (name, email, SIS id, avatar)' },
];

// URL of the launch endpoint, used as an iframe src (the browser follows
// the redirect to the tool's OIDC login). `opts` keys map to query
// parameters: placement, resource_link_id, module_item_id, assignment_id,
// deep_link, module_id.
export function ltiLaunchUrl(courseId, toolId, opts = {}) {
  const q = new URLSearchParams();
  Object.entries(opts).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') q.set(k, String(v));
  });
  const qs = q.toString();
  return `${API_URL}/courses/${courseId}/external_tools/${toolId}/launch${qs ? `?${qs}` : ''}`;
}

// The deep linking return page relays the result with postMessage. Only
// messages from Paper's own origin with that subject are accepted.
export function parseDeepLinkMessage(event, expectedOrigin = window.location.origin) {
  if (!event || event.origin !== expectedOrigin) return null;
  const data = event.data;
  if (!data || typeof data !== 'object' || data.subject !== 'LtiDeepLinkingResponse') return null;
  return {
    error: data.error || data.errormsg || null,
    result: data.result || null,
    items: Array.isArray(data.result?.items) ? data.result.items : [],
  };
}

// Builds the registration payload the API expects from the admin form.
export function buildRegistrationPayload(form) {
  const lines = (s) => String(s || '').split('\n').map((x) => x.trim()).filter(Boolean);
  let customFields = {};
  const rawCustom = String(form.custom_fields || '').trim();
  if (rawCustom) {
    customFields = JSON.parse(rawCustom);
    if (!customFields || typeof customFields !== 'object' || Array.isArray(customFields)) {
      throw new Error('Custom fields must be a JSON object');
    }
  }
  let publicJwk = String(form.public_jwk || '').trim();
  if (publicJwk) {
    try {
      publicJwk = JSON.parse(publicJwk);
    } catch {
      throw new Error('Public JWK must be valid JSON');
    }
  }
  const placements = (form.placements || [])
    .filter((p) => p.enabled)
    .map((p) => {
      const out = { placement: p.placement };
      if (p.message_type) out.message_type = p.message_type;
      if (p.text) out.text = p.text;
      if (p.target_link_uri) out.target_link_uri = p.target_link_uri;
      if (p.icon_url) out.icon_url = p.icon_url;
      return out;
    });
  return {
    name: form.name || form.title,
    email: form.email || '',
    notes: form.notes || '',
    icon_url: form.icon_url || '',
    redirect_uris: lines(form.redirect_uris),
    workflow_state: form.workflow_state || undefined,
    tool_configuration: {
      title: form.title,
      description: form.description || '',
      target_link_uri: form.target_link_uri,
      oidc_initiation_url: form.oidc_initiation_url,
      domain: form.domain || '',
      privacy_level: form.privacy_level || 'anonymous',
      public_jwk_url: publicJwk ? '' : form.public_jwk_url || '',
      public_jwk: publicJwk || null,
      custom_fields: customFields,
      placements,
      scopes: form.scopes || [],
    },
  };
}

// Inverse of buildRegistrationPayload for editing an existing registration.
export function registrationToForm(reg) {
  const tc = reg?.tool_configuration || {};
  const enabled = new Map((tc.placements || []).map((p) => [p.placement, p]));
  return {
    name: reg?.name || '',
    email: reg?.email || '',
    notes: reg?.notes || '',
    icon_url: reg?.icon_url || '',
    redirect_uris: (reg?.redirect_uris || []).join('\n'),
    workflow_state: reg?.workflow_state || 'active',
    title: tc.title || '',
    description: tc.description || '',
    target_link_uri: tc.target_link_uri || '',
    oidc_initiation_url: tc.oidc_initiation_url || '',
    domain: tc.domain || '',
    privacy_level: tc.privacy_level || 'anonymous',
    public_jwk_url: tc.public_jwk_url || '',
    public_jwk: tc.public_jwk ? JSON.stringify(tc.public_jwk, null, 2) : '',
    custom_fields: tc.custom_fields && Object.keys(tc.custom_fields).length ? JSON.stringify(tc.custom_fields, null, 2) : '',
    scopes: tc.scopes || [],
    placements: LTI_PLACEMENTS.map(({ value }) => {
      const p = enabled.get(value);
      return {
        placement: value,
        enabled: Boolean(p),
        message_type: p?.message_type || '',
        text: p?.text || '',
        target_link_uri: p?.target_link_uri || '',
        icon_url: p?.icon_url || '',
      };
    }),
  };
}

// Appends the dynamic registration parameters to a tool's initiation URL.
export function dynamicRegistrationUrl(toolUrl, { openid_configuration, registration_token }) {
  const u = new URL(toolUrl);
  u.searchParams.set('openid_configuration', openid_configuration);
  u.searchParams.set('registration_token', registration_token);
  return u.toString();
}

export const ltiApi = {
  // G123 registrations (admin)
  listRegistrations: async (accountId) => (await request(`/accounts/${accountId}/lti_registrations`)).data || [],
  getRegistration: async (accountId, id) => (await request(`/accounts/${accountId}/lti_registrations/${id}`)).data,
  createRegistration: async (accountId, payload) =>
    (await request(`/accounts/${accountId}/lti_registrations`, json('POST', payload))).data,
  updateRegistration: async (accountId, id, payload) =>
    (await request(`/accounts/${accountId}/lti_registrations/${id}`, json('PUT', payload))).data,
  deleteRegistration: async (accountId, id) =>
    request(`/accounts/${accountId}/lti_registrations/${id}`, { method: 'DELETE' }),
  setAccountInstall: async (accountId, id, enabled) =>
    (await request(`/accounts/${accountId}/lti_registrations/${id}/account_install`, json('PUT', { enabled }))).data,
  platformInfo: async (accountId) => (await request(`/accounts/${accountId}/lti_platform`)).data,
  rotatePlatformKey: async () => (await request('/lti/platform_keys/rotate', { method: 'POST' })).data,
  // G134 dynamic registration
  dynamicRegistrationToken: async (accountId) =>
    (await request(`/accounts/${accountId}/lti_registrations/dynamic_registration_token`, { method: 'POST' })).data,

  // G132 account-wide installs (LTI 1.1 or 1.3)
  listAccountTools: async (accountId) => (await request(`/accounts/${accountId}/external_tools?per_page=100`)).data || [],
  createAccountTool: async (accountId, tool) =>
    (await request(`/accounts/${accountId}/external_tools`, json('POST', { external_tool: tool }))).data,
  deleteAccountTool: async (accountId, id) =>
    request(`/accounts/${accountId}/external_tools/${id}`, { method: 'DELETE' }),

  // G131 course tools by placement
  courseTools: async (courseId, placement) => {
    const qs = placement ? `?placement=${encodeURIComponent(placement)}` : '';
    return (await request(`/courses/${courseId}/lti/tools${qs}`)).data || [];
  },
  getAssignmentTool: async (courseId, assignmentId) =>
    (await request(`/courses/${courseId}/assignments/${assignmentId}/external_tool`)).data,
  setAssignmentTool: async (courseId, assignmentId, body) =>
    (await request(`/courses/${courseId}/assignments/${assignmentId}/external_tool`, json('PUT', body))).data,
  removeAssignmentTool: async (courseId, assignmentId) =>
    request(`/courses/${courseId}/assignments/${assignmentId}/external_tool`, { method: 'DELETE' }),
  createModuleToolItem: async (courseId, moduleId, body) =>
    (await request(`/courses/${courseId}/modules/${moduleId}/external_tool_items`, json('POST', body))).data,
};

export const ltiQueryKeys = {
  registrations: (accountId) => ['lti', 'registrations', String(accountId)],
  platform: (accountId) => ['lti', 'platform', String(accountId)],
  accountTools: (accountId) => ['lti', 'accountTools', String(accountId)],
};
