// Pure helpers for the account-administration pages (unit-tested).

const HEX = /^#[0-9a-fA-F]{6}$/;

export function isHexColor(value) {
  return typeof value === 'string' && HEX.test(value);
}

export function hexToRgb(hex) {
  if (!isHexColor(hex)) return null;
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function mix(rgb, target, amount) {
  return rgb.map((c, i) => Math.round(c + (target[i] - c) * amount));
}

// The brand scale the Tailwind tokens read (--color-brand-50..900, as
// "r g b"). The chosen color sits at 600 (the default theme's primary);
// lighter steps mix toward white, darker toward black.
const SCALE = {
  50: ['w', 0.92], 100: ['w', 0.84], 200: ['w', 0.68], 300: ['w', 0.5], 400: ['w', 0.3],
  500: ['w', 0.12], 600: ['w', 0], 700: ['b', 0.18], 800: ['b', 0.34], 900: ['b', 0.48],
};

export function brandScaleFromHex(hex) {
  const rgb = hexToRgb(hex);
  if (!rgb) return null;
  const out = {};
  Object.entries(SCALE).forEach(([step, [toward, amount]]) => {
    const target = toward === 'w' ? [255, 255, 255] : [0, 0, 0];
    out[step] = mix(rgb, target, amount).join(' ');
  });
  return out;
}

function rgbToHslString([r, g, b]) {
  const rn = r / 255; const gn = g / 255; const bn = b / 255;
  const max = Math.max(rn, gn, bn); const min = Math.min(rn, gn, bn);
  let h = 0; let s = 0;
  const l = (max + min) / 2;
  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    if (max === rn) h = (gn - bn) / d + (gn < bn ? 6 : 0);
    else if (max === gn) h = (bn - rn) / d + 2;
    else h = (rn - gn) / d + 4;
    h /= 6;
  }
  return `${Math.round(h * 360)} ${Math.round(s * 100)}% ${Math.round(l * 100)}%`;
}

// buildBrandCss turns a theme's variables into CSS custom properties for
// the light theme. Only #RRGGBB values are used (the server allow-lists
// them too), so the output can never carry arbitrary CSS.
export function buildBrandCss(variables = {}) {
  const decls = [];
  const scale = brandScaleFromHex(variables.brand_primary);
  if (scale) {
    Object.entries(scale).forEach(([step, rgb]) => decls.push(`--color-brand-${step}: ${rgb};`));
  }
  const button = hexToRgb(variables.button_primary);
  if (button) {
    decls.push(`--primary: ${rgbToHslString(button)};`);
    decls.push(`--ring: ${rgbToHslString(button)};`);
  }
  const buttonText = hexToRgb(variables.button_primary_text);
  if (buttonText) decls.push(`--primary-foreground: ${rgbToHslString(buttonText)};`);
  // The app shell's sidebar reads --color-chrome-sidebar(-fg).
  const navBg = hexToRgb(variables.nav_background);
  if (navBg) decls.push(`--color-chrome-sidebar: ${navBg.join(' ')};`);
  const navText = hexToRgb(variables.nav_text);
  if (navText) decls.push(`--color-chrome-sidebar-fg: ${navText.join(' ')};`);
  if (decls.length === 0) return '';
  return `:root:not(.dark) { ${decls.join(' ')} }`;
}

// Role choices in the Add People picker: a base type ("TaEnrollment") or a
// custom role on top of it ("TaEnrollment|7").
export function roleChoiceValue(type, roleId) {
  return roleId ? `${type}|${roleId}` : type;
}

export function parseRoleChoice(value) {
  const [type, roleId] = String(value || 'StudentEnrollment').split('|');
  const id = roleId ? Number(roleId) : null;
  return Number.isInteger(id) && id > 0 ? { type, role_id: id } : { type };
}

export function formatBytes(bytes) {
  const n = Number(bytes) || 0;
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 10 ? 0 : 1)} ${units[i]}`;
}

// Staff-only course navigation tabs and the permission that governs the
// routes behind each (G102). For an enrollment with a custom role the
// permission decides whether the tab shows (the routes would 403 or, for
// a granted permission, now pass).
export const TAB_PERMISSIONS = {
  grades: 'view_all_grades', // the staff "Grades" tab opens the gradebook
  analytics: 'view_analytics',
  audit_log: 'view_audit_log',
  pacing: 'manage_pacing',
  blueprint: 'manage_blueprint',
  question_banks: 'manage_quizzes',
  content_import: 'manage_content',
  external_tools: 'manage_content',
  settings: 'manage_content',
};

// customRoleTabDecision returns true/false when a custom role decides the
// tab, or null to fall back to the type-based rule.
export function customRoleTabDecision(tabId, customRole) {
  if (!customRole?.custom_role) return null;
  const perm = TAB_PERMISSIONS[tabId];
  if (!perm) return null;
  return customRole.permission_map?.[perm] === true;
}

// extractTerms normalises api.getEnrollmentTerms() responses
// ({data: {enrollment_terms: [...]}}) to an array.
export function extractTerms(res) {
  const d = res?.data ?? res;
  if (Array.isArray(d)) return d;
  if (Array.isArray(d?.enrollment_terms)) return d.enrollment_terms;
  return [];
}

// A report run is still working until it completes or errors.
export function reportRunning(run) {
  return !!run && (run.status === 'created' || run.status === 'running');
}

// A SIS batch is still working until it reaches a final state.
export function sisBatchRunning(batch) {
  return !!batch && (batch.workflow_state === 'created' || batch.workflow_state === 'importing');
}
