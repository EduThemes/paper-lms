/**
 * Mapping between the ePortfolio API (snake_case, backend field names) and
 * the shape the portfolio pages render.
 *
 * The API returns portfolios with `workflow_state` and `theme_id`, sections
 * with `section_type` / `is_visible` / `position`, and artifacts with
 * `artifact_type` / `content_url` / `is_featured` / `tags` (a JSON string).
 * The pages were written against `status`, `theme`, `type`, `visible`,
 * `order`, `url`, `featured` and a `tags` array, so every read goes through
 * normalizePortfolio and every write through the to*Payload helpers.
 */

// UI theme ids (used for styling) <-> backend theme ids (used by exports).
const THEME_UI_TO_API = {
  clean_modern: 'clean-modern',
  creative_bold: 'creative-bold',
  academic_classic: 'academic-classic',
  minimal_dark: 'minimal-dark',
  developer_portfolio: 'portfolio-developer',
};
const THEME_API_TO_UI = Object.fromEntries(
  Object.entries(THEME_UI_TO_API).map(([ui, apiId]) => [apiId, ui]),
);

export function themeFromApi(themeId) {
  if (THEME_API_TO_UI[themeId]) return THEME_API_TO_UI[themeId];
  if (THEME_UI_TO_API[themeId]) return themeId;
  return 'clean_modern';
}

export function themeToApi(theme) {
  return THEME_UI_TO_API[theme] || theme;
}

export function parseTags(tags) {
  if (Array.isArray(tags)) return tags.filter(Boolean);
  if (typeof tags !== 'string' || tags.trim() === '') return [];
  try {
    const parsed = JSON.parse(tags);
    if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean);
  } catch {
    // not JSON — fall through to comma-separated
  }
  return tags.split(',').map((t) => t.trim()).filter(Boolean);
}

/** Artifact source the editor's tabs use: file | link | course_submission. */
function artifactKind(a) {
  if (a.source_type === 'upload') return 'file';
  if (a.source_type === 'course_submission' || a.artifact_type === 'course_work') return 'course_submission';
  if (a.artifact_type === 'link') return 'link';
  return 'file';
}

export function normalizeSection(s) {
  if (!s) return s;
  return {
    ...s,
    type: s.section_type ?? s.type,
    visible: s.is_visible ?? s.visible ?? true,
    order: s.position ?? s.order ?? 0,
  };
}

export function normalizeArtifact(a) {
  if (!a) return a;
  return {
    ...a,
    type: artifactKind(a),
    url: a.content_url ?? a.url ?? '',
    featured: a.is_featured ?? a.featured ?? false,
    tags: parseTags(a.tags),
  };
}

export function normalizePortfolio(p) {
  if (!p) return p;
  return {
    ...p,
    status: p.workflow_state ?? p.status ?? 'draft',
    theme: themeFromApi(p.theme_id ?? p.theme),
    cover_image_url: p.header_image_url ?? p.cover_image_url ?? '',
    sections: (p.sections || []).map(normalizeSection),
    artifacts: (p.artifacts || []).map(normalizeArtifact),
  };
}

/** Portfolio field updates (UI names) -> PUT /portfolios/:id body fields. */
export function toPortfolioPayload(fields) {
  const out = {};
  for (const [key, value] of Object.entries(fields || {})) {
    switch (key) {
      case 'theme':
        out.theme_id = themeToApi(value);
        break;
      case 'cover_image_url':
        out.header_image_url = value;
        break;
      case 'status':
        out.workflow_state = value;
        break;
      case 'sections':
      case 'artifacts':
        break;
      default:
        out[key] = value;
    }
  }
  return out;
}

/** Section updates (UI names) -> section body fields. */
export function toSectionPayload(updates) {
  const out = {};
  for (const [key, value] of Object.entries(updates || {})) {
    switch (key) {
      case 'type':
        out.section_type = value;
        break;
      case 'visible':
        out.is_visible = value;
        break;
      case 'order':
        out.position = value;
        break;
      default:
        out[key] = value;
    }
  }
  return out;
}

const KIND_TO_ARTIFACT_TYPE = { link: 'link', course_submission: 'course_work', file: 'document' };

/** The editor's "Add Artifact" form -> POST /portfolios/:id/artifacts body. */
export function toArtifactPayload(form) {
  const tags = parseTags(form.tags);
  const payload = {
    title: form.title,
    description: form.description || '',
    artifact_type: KIND_TO_ARTIFACT_TYPE[form.type] || 'document',
    content_url: form.url || '',
    is_featured: !!form.featured,
    tags: JSON.stringify(tags),
  };
  if (form.section_id) payload.section_id = form.section_id;
  if (form.type === 'link') payload.source_type = 'external_link';
  return payload;
}

/** Public, shareable path of a published portfolio (served by App.jsx). */
export function publicPortfolioPath(p) {
  return `/portfolios/public/${encodeURIComponent(p?.slug || '')}`;
}
