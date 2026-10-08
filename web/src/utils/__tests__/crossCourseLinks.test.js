import { describe, it, expect, vi, afterEach } from 'vitest';
import { detectCrossCourseLinks } from '../crossCourseLinks';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('detectCrossCourseLinks', () => {
  it('parses the HTML into an inert document, not the live one (B001)', () => {
    const create = vi.spyOn(document, 'createElement');
    const issues = detectCrossCourseLinks(
      '<p><a href="/courses/9/pages/intro">Intro</a></p><img src="x" onerror="window.__xcPwned = 1">',
      5,
    );
    // No element was created in the live document, where <img> would load
    // and fire its onerror handler.
    expect(create).not.toHaveBeenCalled();
    expect(window.__xcPwned).toBeUndefined();
    expect(issues).toHaveLength(1);
    expect(issues[0]).toMatchObject({ element: 'a', attribute: 'href', referencedCourseId: '9', text: 'Intro' });
  });

  it('ignores links to the current course and external URLs', () => {
    expect(detectCrossCourseLinks(
      '<a href="/courses/5/assignments/1">a</a><a href="https://other.example/courses/9">b</a>',
      5,
    )).toEqual([]);
  });
});
