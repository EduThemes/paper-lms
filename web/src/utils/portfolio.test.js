import { describe, test, expect } from 'vitest';
import {
  normalizePortfolio,
  toArtifactPayload,
  toPortfolioPayload,
  toSectionPayload,
  publicPortfolioPath,
  parseTags,
} from './portfolio';

describe('portfolio API mapping', () => {
  test('normalizePortfolio maps workflow_state, theme_id, sections and artifacts', () => {
    const p = normalizePortfolio({
      id: 1,
      slug: 'my-work-123',
      workflow_state: 'published',
      theme_id: 'portfolio-developer',
      header_image_url: 'https://example.com/cover.png',
      sections: [{ id: 3, section_type: 'about', is_visible: false, position: 2 }],
      artifacts: [
        { id: 4, artifact_type: 'link', content_url: 'https://example.com', is_featured: true, tags: '["a","b"]' },
        { id: 5, artifact_type: 'document', source_type: 'upload', content_url: '/api/v1/portfolios/1/files/9', tags: '' },
      ],
    });
    // B197: the UI reads status, not workflow_state.
    expect(p.status).toBe('published');
    expect(p.theme).toBe('developer_portfolio');
    expect(p.cover_image_url).toBe('https://example.com/cover.png');
    expect(p.sections[0]).toMatchObject({ type: 'about', visible: false, order: 2 });
    expect(p.artifacts[0]).toMatchObject({ type: 'link', url: 'https://example.com', featured: true, tags: ['a', 'b'] });
    expect(p.artifacts[1]).toMatchObject({ type: 'file', tags: [] });
  });

  test('toArtifactPayload sends backend field names and tags as a JSON string (B097)', () => {
    const payload = toArtifactPayload({
      title: 'Essay', description: 'd', tags: 'writing, history', type: 'link',
      url: 'https://example.com/essay', featured: true, section_id: 7,
    });
    expect(payload).toEqual({
      title: 'Essay',
      description: 'd',
      artifact_type: 'link',
      content_url: 'https://example.com/essay',
      is_featured: true,
      tags: '["writing","history"]',
      section_id: 7,
      source_type: 'external_link',
    });
    expect(typeof payload.tags).toBe('string');
  });

  test('toSectionPayload and toPortfolioPayload rename UI fields', () => {
    expect(toSectionPayload({ type: 'skills', visible: true, order: 3, title: 'T' }))
      .toEqual({ section_type: 'skills', is_visible: true, position: 3, title: 'T' });
    expect(toPortfolioPayload({ theme: 'creative_bold', status: 'archived', title: 'x', sections: [] }))
      .toEqual({ theme_id: 'creative-bold', workflow_state: 'archived', title: 'x' });
  });

  test('publicPortfolioPath points at the routed public page (B196)', () => {
    expect(publicPortfolioPath({ slug: 'my-work-123' })).toBe('/portfolios/public/my-work-123');
  });

  test('parseTags accepts arrays, JSON strings and comma lists', () => {
    expect(parseTags(['a'])).toEqual(['a']);
    expect(parseTags('["x"]')).toEqual(['x']);
    expect(parseTags('x, y')).toEqual(['x', 'y']);
    expect(parseTags(null)).toEqual([]);
  });
});
