// API calls and react-query hooks for the Page Designer.
//
// Builds on the helpers in api.js (session cookie + CSRF, ApiError) and adds the designer-only calls:
// colour schemes, designed pages (`wiki_page[css]` / `wiki_page[design]`), the course look, course teachers
// and assignment due dates. Nothing here imports the design engine, so this module stays light for any page
// that wants it (e.g. Course settings → Look).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, request, requestPaged } from './api';
import { queryKeys } from './apiQueries';

export const designerKeys = {
  all: ['designer'],
  page: (courseId, slug) => ['designer', 'page', String(courseId), String(slug)],
  teachers: (courseId) => ['designer', 'teachers', String(courseId)],
  dueDates: (courseId) => ['designer', 'dueDates', String(courseId)],
  schemes: ['designer', 'colourSchemes'],
};

// ---------------------------------------------------------------------------
// Plain calls
// ---------------------------------------------------------------------------

/** GET a page; Show includes `css`, `scoped_css` and `design` (object or null). */
export const getDesignedPage = (courseId, slug) => api.getPage(courseId, slug);

/** POST a new page: wiki_page = { title, body, css, design, published, front_page }. */
export const createDesignedPage = (courseId, wikiPage) => api.createPage(courseId, wikiPage);

/** PUT an existing page by slug, same payload as create. */
export const updateDesignedPage = (courseId, slug, wikiPage) => api.updatePage(courseId, slug, wikiPage);

/** PUT course[default_view] = 'wiki', so the course opens on its front page. */
export const setCourseHomeToFrontPage = (courseId) => api.updateCourse(courseId, { default_view: 'wiki' });

/** PUT course[look] (null clears it). */
export const updateCourseLook = (courseId, look) => api.updateCourse(courseId, { look });

/** Adds a page to the top of a module (POST module_item { type: 'Page', content_id, title, position: 1 }). */
export const addPageToModule = (courseId, moduleId, { pageId, title }) =>
  api.createModuleItem(courseId, moduleId, { type: 'Page', content_id: pageId, title, position: 1 });

/** Uploads an image into the course's Files; returns the file and the address to use as an image src. */
export async function uploadDesignImage(courseId, file) {
  const res = await api.uploadCourseFile(courseId, file);
  const f = res?.attachment ?? res?.file ?? res;
  const id = f?.id;
  if (!id) throw new Error('Upload failed');
  return { file: f, src: `/api/v1/files/${id}/download` };
}

/** The course's teachers: [{ id, name, email? }]. */
export async function getCourseTeachers(courseId) {
  const { data } = await request(`/courses/${courseId}/users?enrollment_type[]=teacher&include[]=email&per_page=10`);
  return (Array.isArray(data) ? data : []).map((u) => ({
    id: u.id,
    name: u.short_name || u.name || u.sortable_name || '',
    email: u.email || u.login_id || '',
  }));
}

/** Assignment id → due_at, for "This week" checklists and module dates. */
export async function getAssignmentDueDates(courseId) {
  const { data } = await requestPaged(`/courses/${courseId}/assignments`, 1, 400);
  const out = {};
  for (const a of Array.isArray(data) ? data : []) if (a?.id && a.due_at) out[String(a.id)] = a.due_at;
  return out;
}

const SCHEMES_PATH = '/users/self/design_colour_schemes';

const normalizeScheme = (s) => ({
  id: String(s.id),
  name: String(s.name ?? ''),
  palette: s.palette ?? null,
  updatedAt: s.updated_at ?? s.updatedAt ?? null,
});

/** GET my colour schemes: [{ id, name, palette, updatedAt }]. */
export async function listColourSchemes() {
  const { data } = await request(SCHEMES_PATH);
  const list = Array.isArray(data) ? data : Array.isArray(data?.schemes) ? data.schemes : [];
  return list.map(normalizeScheme);
}

/** POST (no id) or PUT a colour scheme. Body { name, palette }. */
export async function saveColourScheme({ id, name, palette }) {
  const body = JSON.stringify({ name, palette });
  const { data } = id
    ? await request(`${SCHEMES_PATH}/${encodeURIComponent(id)}`, { method: 'PUT', body })
    : await request(SCHEMES_PATH, { method: 'POST', body });
  return normalizeScheme(data?.scheme ?? data ?? { id, name, palette });
}

export async function deleteColourScheme(id) {
  await request(`${SCHEMES_PATH}/${encodeURIComponent(id)}`, { method: 'DELETE' });
  return id;
}

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

/**
 * After a page or the course look changes: the course detail, anything keyed ['course', id, …] (e.g. the
 * course front page query), and any query whose key names the front page.
 */
export function invalidateCourse(qc, courseId) {
  const id = String(courseId);
  qc.invalidateQueries({ queryKey: queryKeys.courses.detail(id) });
  qc.invalidateQueries({
    predicate: (q) => {
      const k = q.queryKey;
      if (!Array.isArray(k)) return false;
      if (k[0] === 'course' && String(k[1]) === id) return true;
      return k.some((x) => x === 'frontPage' || x === 'front_page' || x === 'front-page') && k.some((x) => String(x) === id);
    },
  });
}

export function useDesignedPage(courseId, slug, options = {}) {
  return useQuery({
    queryKey: designerKeys.page(courseId, slug),
    queryFn: () => getDesignedPage(courseId, slug),
    enabled: !!courseId && !!slug,
    retry: (n, err) => err?.status !== 404 && n < 2,
    ...options,
  });
}

export function useCourseTeachers(courseId, options = {}) {
  return useQuery({
    queryKey: designerKeys.teachers(courseId),
    queryFn: () => getCourseTeachers(courseId),
    enabled: !!courseId,
    staleTime: 5 * 60 * 1000,
    ...options,
  });
}

export function useAssignmentDueDates(courseId, options = {}) {
  return useQuery({
    queryKey: designerKeys.dueDates(courseId),
    queryFn: () => getAssignmentDueDates(courseId),
    enabled: !!courseId,
    staleTime: 60 * 1000,
    ...options,
  });
}

export function useColourSchemes(options = {}) {
  return useQuery({
    queryKey: designerKeys.schemes,
    queryFn: listColourSchemes,
    staleTime: 60 * 1000,
    ...options,
  });
}

export function useSaveColourScheme(options = {}) {
  const qc = useQueryClient();
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: saveColourScheme,
    ...rest,
    onSuccess: (...args) => {
      qc.invalidateQueries({ queryKey: designerKeys.schemes });
      onSuccess?.(...args);
    },
  });
}

export function useDeleteColourScheme(options = {}) {
  const qc = useQueryClient();
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: deleteColourScheme,
    ...rest,
    onSuccess: (...args) => {
      qc.invalidateQueries({ queryKey: designerKeys.schemes });
      onSuccess?.(...args);
    },
  });
}

/**
 * Saves a designed page. `slug` set → PUT that page; otherwise POST a new one (Paper gives a clashing title
 * a fresh slug, so nothing is overwritten). Optionally sets it as the course front page (+ default_view
 * 'wiki') and adds it to a module.
 *
 * vars: { slug?, wikiPage, makeHome?, moduleId? } → { page, home: bool|Error, moduleItem: obj|Error|null }
 */
export function useSaveDesignedPage(courseId, options = {}) {
  const qc = useQueryClient();
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: async ({ slug, wikiPage, makeHome = false, moduleId = null }) => {
      const page = slug ? await updateDesignedPage(courseId, slug, wikiPage) : await createDesignedPage(courseId, wikiPage);
      let home = false;
      if (makeHome) {
        try {
          await setCourseHomeToFrontPage(courseId);
          home = true;
        } catch (e) {
          home = e;
        }
      }
      let moduleItem = null;
      if (moduleId && page?.page_id) {
        try {
          moduleItem = await addPageToModule(courseId, moduleId, { pageId: page.page_id, title: page.title ?? wikiPage.title });
        } catch (e) {
          moduleItem = e;
        }
      }
      return { page, home, moduleItem };
    },
    ...rest,
    onSuccess: (data, ...args) => {
      if (data?.page?.url) qc.setQueryData(designerKeys.page(courseId, data.page.url), data.page);
      invalidateCourse(qc, courseId);
      qc.invalidateQueries({ queryKey: ['courses', String(courseId), 'modules'] });
      onSuccess?.(data, ...args);
    },
  });
}

export function useUpdateCourseLook(courseId, options = {}) {
  const qc = useQueryClient();
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn: (look) => updateCourseLook(courseId, look),
    ...rest,
    onSuccess: (...args) => {
      invalidateCourse(qc, courseId);
      onSuccess?.(...args);
    },
  });
}
