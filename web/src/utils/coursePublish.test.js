import { isCoursePublished, publishSteps, unpublishSteps, courseStateLabel } from './coursePublish';

describe('course publish state (G077)', () => {
  test('published means available or concluded', () => {
    expect(isCoursePublished('available')).toBe(true);
    expect(isCoursePublished('completed')).toBe(true);
    expect(isCoursePublished('unpublished')).toBe(false);
    expect(isCoursePublished('claimed')).toBe(false);
  });

  test('publishing walks the one-step state machine', () => {
    expect(publishSteps('claimed')).toEqual(['created', 'unpublished', 'available']);
    expect(publishSteps('created')).toEqual(['unpublished', 'available']);
    expect(publishSteps('unpublished')).toEqual(['available']);
    expect(publishSteps(undefined)).toEqual(['available']);
    expect(publishSteps('available')).toEqual([]);
    expect(publishSteps('completed')).toEqual([]);
    expect(publishSteps('deleted')).toEqual([]);
  });

  test('only an available course can be unpublished', () => {
    expect(unpublishSteps('available')).toEqual(['unpublished']);
    expect(unpublishSteps('completed')).toEqual([]);
    expect(unpublishSteps('unpublished')).toEqual([]);
  });

  test('labels', () => {
    expect(courseStateLabel('available')).toBe('Published');
    expect(courseStateLabel('completed')).toBe('Concluded');
    expect(courseStateLabel('created')).toBe('Unpublished');
  });
});
