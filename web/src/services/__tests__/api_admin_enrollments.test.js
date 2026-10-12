import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { enrollmentsAdminApi, rosterPath } from '../api_admin_enrollments';

// The admin-enrollments API module hits the Canvas-shaped endpoints.

const okJSON = (body, link) => ({
  ok: true,
  status: 200,
  json: async () => body,
  text: async () => JSON.stringify(body),
  headers: new Headers(link ? { Link: link } : {}),
});

const call = (i = 0) => {
  const [url, opts] = global.fetch.mock.calls[i];
  return { url, method: opts?.method || 'GET', body: opts?.body ? JSON.parse(opts.body) : undefined };
};

describe('enrollmentsAdminApi', () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue(okJSON({ id: 1 }));
  });
  afterEach(() => vi.restoreAllMocks());

  test('rosterPath encodes state filters', () => {
    expect(rosterPath(9)).toBe('/courses/9/enrollments');
    expect(rosterPath(9, ['active', 'invited'])).toBe('/courses/9/enrollments?state[]=active&state[]=invited');
  });

  test('listRoster walks pages with the state filter', async () => {
    const page1 = Array.from({ length: 100 }, (_, i) => ({ id: i }));
    global.fetch
      .mockResolvedValueOnce(okJSON(page1))
      .mockResolvedValueOnce(okJSON([{ id: 100 }]));
    const rows = await enrollmentsAdminApi.listRoster(9, ['inactive']);
    expect(rows).toHaveLength(101);
    expect(call(0).url).toContain('/courses/9/enrollments?state[]=inactive&page=1&per_page=100');
    expect(call(1).url).toContain('page=2');
  });

  test('lifecycle calls', async () => {
    await enrollmentsAdminApi.updateEnrollment(9, 7, { type: 'TaEnrollment' });
    expect(call(0)).toMatchObject({ method: 'PUT', body: { enrollment: { type: 'TaEnrollment' } } });
    expect(call(0).url).toContain('/courses/9/enrollments/7');

    await enrollmentsAdminApi.applyTask(9, 7, 'conclude');
    expect(call(1).method).toBe('DELETE');
    expect(call(1).url).toContain('/courses/9/enrollments/7?task=conclude');

    await enrollmentsAdminApi.reactivate(9, 7);
    expect(call(2)).toMatchObject({ method: 'PUT' });
    expect(call(2).url).toContain('/courses/9/enrollments/7/reactivate');
  });

  test('invitation calls', async () => {
    global.fetch.mockResolvedValueOnce(okJSON([{ id: 3, course_id: 9 }]));
    expect(await enrollmentsAdminApi.listInvitations()).toHaveLength(1);
    expect(call(0).url).toContain('/users/self/course_invitations');
    await enrollmentsAdminApi.acceptInvitation(9, 3);
    expect(call(1)).toMatchObject({ method: 'POST' });
    expect(call(1).url).toContain('/courses/9/enrollments/3/accept');
    await enrollmentsAdminApi.rejectInvitation(9, 3);
    expect(call(2).url).toContain('/courses/9/enrollments/3/reject');
  });

  test('section and cross-list calls', async () => {
    await enrollmentsAdminApi.updateSection(4, { name: 'Evening' });
    expect(call(0)).toMatchObject({ method: 'PUT', body: { course_section: { name: 'Evening' } } });
    expect(call(0).url).toContain('/sections/4');
    await enrollmentsAdminApi.deleteSection(4);
    expect(call(1).method).toBe('DELETE');
    await enrollmentsAdminApi.crosslistSection(4, 12);
    expect(call(2)).toMatchObject({ method: 'POST' });
    expect(call(2).url).toContain('/sections/4/crosslist/12');
    await enrollmentsAdminApi.uncrosslistSection(4);
    expect(call(3)).toMatchObject({ method: 'DELETE' });
    expect(call(3).url).toContain('/sections/4/crosslist');
  });

  test('course date-access settings go through PUT /courses/:id', async () => {
    await enrollmentsAdminApi.updateCourseDateAccess(9, { restrict_student_past_view: true });
    expect(call(0)).toMatchObject({ method: 'PUT', body: { course: { restrict_student_past_view: true } } });
  });
});
