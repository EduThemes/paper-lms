import {
  brandScaleFromHex, buildBrandCss, parseRoleChoice, roleChoiceValue, formatBytes,
  customRoleTabDecision, extractTerms, reportRunning, sisBatchRunning, isHexColor,
} from './adminAccounts';

describe('theme helpers (G118)', () => {
  it('derives a 50-900 brand scale with the chosen color at 600', () => {
    const scale = brandScaleFromHex('#0A7B3E');
    expect(Object.keys(scale)).toHaveLength(10);
    expect(scale[600]).toBe('10 123 62');
    expect(scale[50]).not.toBe(scale[900]);
  });

  it('only emits CSS for #RRGGBB values', () => {
    expect(buildBrandCss({ brand_primary: 'red; } body { display:none' })).toBe('');
    expect(buildBrandCss({})).toBe('');
    const css = buildBrandCss({ brand_primary: '#0A7B3E', nav_background: '#112233', button_primary: '#FF0000' });
    expect(css.startsWith(':root:not(.dark) {')).toBe(true);
    expect(css).toContain('--color-brand-600: 10 123 62;');
    expect(css).toContain('--color-chrome-sidebar: 17 34 51;');
    expect(css).toContain('--primary: 0 100% 50%;');
    expect(css).not.toContain('display:none');
    expect(isHexColor('#12345')).toBe(false);
  });
});

describe('role choice (G102)', () => {
  it('round-trips base and custom roles', () => {
    expect(parseRoleChoice('TaEnrollment')).toEqual({ type: 'TaEnrollment' });
    expect(parseRoleChoice(roleChoiceValue('TaEnrollment', 7))).toEqual({ type: 'TaEnrollment', role_id: 7 });
    expect(parseRoleChoice('TaEnrollment|x')).toEqual({ type: 'TaEnrollment' });
    expect(parseRoleChoice(undefined)).toEqual({ type: 'StudentEnrollment' });
  });

  it('lets a custom role decide only its permission-gated tabs', () => {
    const custom = { custom_role: true, permission_map: { view_audit_log: true, view_all_grades: false } };
    expect(customRoleTabDecision('audit_log', custom)).toBe(true);
    expect(customRoleTabDecision('grades', custom)).toBe(false);
    expect(customRoleTabDecision('analytics', custom)).toBe(false);
    expect(customRoleTabDecision('modules', custom)).toBeNull();
    expect(customRoleTabDecision('audit_log', null)).toBeNull();
    expect(customRoleTabDecision('audit_log', { custom_role: false })).toBeNull();
  });
});

describe('misc admin helpers', () => {
  it('formats byte counts', () => {
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(50 * 1024 * 1024)).toBe('50 MB');
  });

  it('normalises term responses', () => {
    expect(extractTerms({ data: { enrollment_terms: [{ id: 1 }] } })).toEqual([{ id: 1 }]);
    expect(extractTerms({ data: [{ id: 2 }] })).toEqual([{ id: 2 }]);
    expect(extractTerms(null)).toEqual([]);
  });

  it('knows when reports and SIS batches are still working', () => {
    expect(reportRunning({ status: 'running' })).toBe(true);
    expect(reportRunning({ status: 'complete' })).toBe(false);
    expect(reportRunning(undefined)).toBe(false);
    expect(sisBatchRunning({ workflow_state: 'created' })).toBe(true);
    expect(sisBatchRunning({ workflow_state: 'aborted' })).toBe(false);
  });
});
