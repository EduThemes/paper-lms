/* eslint-disable react/prop-types */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { render } from '@testing-library/react';
import { vi } from 'vitest';

/** Made-up course data only. */
export const COURSE = { id: 42, name: 'Biology 101', course_code: 'BIO101', ui_mode: 'standard', custom_css_enabled: false, default_view: 'modules' };

/** A fake of the api.js surface the designer uses. */
export function makeApi(over = {}) {
  let pageId = 900;
  const api = {
    getCourse: vi.fn(async () => COURSE),
    getPage: vi.fn(async () => {
      throw Object.assign(new Error('Not found'), { status: 404 });
    }),
    createPage: vi.fn(async (courseId, wp) => ({
      page_id: ++pageId,
      url: wp.title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''),
      title: wp.title,
      published: wp.published,
      front_page: wp.front_page,
    })),
    updatePage: vi.fn(async (courseId, slug, wp) => ({ page_id: 901, url: slug, title: wp.title, published: wp.published, front_page: wp.front_page })),
    updateCourse: vi.fn(async (id, c) => ({ ...COURSE, ...c })),
    getModules: vi.fn(async () => ({ data: [], pagination: {} })),
    createModuleItem: vi.fn(async () => ({ id: 1 })),
    uploadCourseFile: vi.fn(async () => ({ id: 77 })),
    ...over,
  };
  const request = vi.fn(async (path) => {
    if (path.startsWith('/courses/42/users')) return { data: [{ id: 1, name: 'Ms. Example', email: 'teacher@example.test' }] };
    if (path.startsWith('/users/self/design_colour_schemes')) return { data: [] };
    return { data: null };
  });
  const requestPaged = vi.fn(async () => ({ data: [], pagination: {} }));
  return { api, request, requestPaged };
}

export function renderRoute(element, { path, url }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[url]}>
        <Routes>
          <Route path={path} element={element} />
          <Route path="*" element={<p>Elsewhere</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}
