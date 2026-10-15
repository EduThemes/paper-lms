import { THEMES } from '../lib/engine';

/** Made-up test data only. */
export function makeDoc(sections) {
  return {
    version: 1,
    id: 'doc-test-1',
    themeId: THEMES[0].id,
    paletteId: THEMES[0].palettes[0].id,
    layout: 'classic-band',
    kind: 'home',
    locale: 'en',
    sections: sections ?? [
      { id: 'a', type: 'title', text: 'Biology 101' },
      { id: 'b', type: 'welcome', heading: 'Welcome', body: '<p>Hi</p>' },
      { id: 'c', type: 'divider' },
    ],
    options: { density: 'comfortable', headingStyle: 'band' },
  };
}

/** Paper's modules API shape (GET /courses/:id/modules?include[]=items). Made-up course data. */
export const PAPER_MODULES = [
  {
    id: 5000,
    name: 'Week 1: Cells',
    position: 1,
    unlock_at: '2026-09-01T12:00:00Z',
    workflow_state: 'active',
    published: true,
    items: [
      { id: 1, title: 'Read chapter 1', type: 'Page', page_url: 'chapter-1', position: 1, published: true, workflow_state: 'active' },
      { id: 2, title: 'Quiz 1', type: 'Assignment', content_id: 77, position: 2, published: true, workflow_state: 'active' },
    ],
  },
  { id: 5001, name: 'Week 2: Energy', position: 2, unlock_at: null, workflow_state: 'active', published: true, items: [] },
  { id: 5002, name: 'Draft unit', position: 3, unlock_at: null, workflow_state: 'unpublished', published: false, items: [] },
];
