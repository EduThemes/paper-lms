import {
  ltiApi,
  ltiLaunchUrl,
  parseDeepLinkMessage,
  buildRegistrationPayload,
  registrationToForm,
  dynamicRegistrationUrl,
} from '../api_lti';
import { request } from '../api';

vi.mock('../api', () => ({
  request: vi.fn(),
}));

describe('ltiLaunchUrl', () => {
  test('builds the launch endpoint with only the set parameters', () => {
    expect(ltiLaunchUrl(10, 5)).toBe('/api/v1/courses/10/external_tools/5/launch');
    expect(ltiLaunchUrl(10, 5, { placement: 'editor_button', deep_link: 'editor', module_id: '' })).toBe(
      '/api/v1/courses/10/external_tools/5/launch?placement=editor_button&deep_link=editor'
    );
    expect(ltiLaunchUrl(10, 5, { resource_link_id: 'a b' })).toContain('resource_link_id=a+b');
  });
});

describe('parseDeepLinkMessage', () => {
  const origin = 'https://lms.example.com';
  test('accepts only same-origin LtiDeepLinkingResponse messages', () => {
    const items = [{ type: 'link', url: 'https://x.example.com' }];
    expect(parseDeepLinkMessage({ origin, data: { subject: 'LtiDeepLinkingResponse', result: { items } } }, origin)).toEqual({
      error: null,
      result: { items },
      items,
    });
    expect(parseDeepLinkMessage({ origin: 'https://evil.example.net', data: { subject: 'LtiDeepLinkingResponse' } }, origin)).toBeNull();
    expect(parseDeepLinkMessage({ origin, data: { subject: 'other' } }, origin)).toBeNull();
    expect(parseDeepLinkMessage({ origin, data: 'string' }, origin)).toBeNull();
  });
  test('surfaces the error of a refused response', () => {
    const parsed = parseDeepLinkMessage({ origin, data: { subject: 'LtiDeepLinkingResponse', error: 'nope' } }, origin);
    expect(parsed.error).toBe('nope');
    expect(parsed.items).toEqual([]);
  });
});

describe('registration form <-> payload', () => {
  test('builds the API payload from the form and back', () => {
    const form = registrationToForm(null);
    Object.assign(form, {
      title: 'Example Tool',
      target_link_uri: 'https://tool.example.com/launch',
      oidc_initiation_url: 'https://tool.example.com/login',
      public_jwk_url: 'https://tool.example.com/jwks',
      privacy_level: 'name_only',
      redirect_uris: 'https://tool.example.com/launch\n\n https://tool.example.com/alt ',
      custom_fields: '{"course": "$Context.title"}',
      scopes: ['https://purl.imsglobal.org/spec/lti-ags/scope/score'],
    });
    form.placements = form.placements.map((p) =>
      p.placement === 'editor_button' ? { ...p, enabled: true, text: 'Insert from tool' } : p
    );
    const payload = buildRegistrationPayload(form);
    expect(payload.name).toBe('Example Tool');
    expect(payload.redirect_uris).toEqual(['https://tool.example.com/launch', 'https://tool.example.com/alt']);
    expect(payload.tool_configuration.custom_fields).toEqual({ course: '$Context.title' });
    expect(payload.tool_configuration.placements).toEqual([{ placement: 'editor_button', text: 'Insert from tool' }]);
    expect(payload.tool_configuration.public_jwk).toBeNull();

    const back = registrationToForm({ ...payload, id: 3, workflow_state: 'active' });
    expect(back.title).toBe('Example Tool');
    expect(back.placements.find((p) => p.placement === 'editor_button').enabled).toBe(true);
    expect(back.placements.find((p) => p.placement === 'course_navigation').enabled).toBe(false);
  });

  test('rejects malformed custom fields and JWK', () => {
    const form = { ...registrationToForm(null), title: 'T', custom_fields: '[1]' };
    expect(() => buildRegistrationPayload(form)).toThrow(/JSON object/);
    expect(() => buildRegistrationPayload({ ...form, custom_fields: '', public_jwk: '{nope' })).toThrow(/valid JSON/);
  });
});

describe('dynamicRegistrationUrl', () => {
  test('appends openid_configuration and registration_token', () => {
    const u = new URL(dynamicRegistrationUrl('https://tool.example.com/register?x=1', {
      openid_configuration: 'https://lms.example.com/api/v1/lti/security/openid-configuration',
      registration_token: 'tok',
    }));
    expect(u.searchParams.get('x')).toBe('1');
    expect(u.searchParams.get('registration_token')).toBe('tok');
    expect(u.searchParams.get('openid_configuration')).toContain('openid-configuration');
  });
});

describe('ltiApi', () => {
  beforeEach(() => vi.clearAllMocks());
  test('courseTools passes the placement filter', async () => {
    request.mockResolvedValue({ data: [{ id: 1 }] });
    await expect(ltiApi.courseTools(7, 'course_navigation')).resolves.toEqual([{ id: 1 }]);
    expect(request).toHaveBeenCalledWith('/courses/7/lti/tools?placement=course_navigation');
  });
  test('setAccountInstall PUTs the enabled flag', async () => {
    request.mockResolvedValue({ data: { id: 3 } });
    await ltiApi.setAccountInstall(42, 3, true);
    expect(request).toHaveBeenCalledWith('/accounts/42/lti_registrations/3/account_install', {
      method: 'PUT',
      body: JSON.stringify({ enabled: true }),
    });
  });
});
