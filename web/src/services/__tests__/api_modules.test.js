import { request } from '../api';
import {
  buildToolLaunchForm,
  createBlackoutDate,
  getCoursePaceTimeline,
  getModuleProgress,
  markModuleItemRead,
  setModuleItemDone,
} from '../api_modules';

vi.mock('../api', () => ({
  request: vi.fn(),
}));

describe('api_modules', () => {
  beforeEach(() => {
    request.mockReset();
  });

  test('reads own or a student progress', async () => {
    request.mockResolvedValue({ data: { modules: [] } });
    await getModuleProgress(3);
    expect(request).toHaveBeenLastCalledWith('/courses/3/module_progress');
    await getModuleProgress(3, 44);
    expect(request).toHaveBeenLastCalledWith('/courses/3/module_progress?user_id=44');
  });

  test('marks items read and done / not done', async () => {
    request.mockResolvedValue({ data: null });
    await markModuleItemRead(3, 4, 5);
    expect(request).toHaveBeenLastCalledWith('/courses/3/modules/4/items/5/mark_read', { method: 'POST' });
    await setModuleItemDone(3, 4, 5, true);
    expect(request).toHaveBeenLastCalledWith('/courses/3/modules/4/items/5/done', { method: 'PUT' });
    await setModuleItemDone(3, 4, 5, false);
    expect(request).toHaveBeenLastCalledWith('/courses/3/modules/4/items/5/done', { method: 'DELETE' });
  });

  test('pacing timeline and blackout dates', async () => {
    request.mockResolvedValue({ data: [{ module_item_id: 1, projected_date: '2026-09-09T00:00:00Z' }] });
    const tl = await getCoursePaceTimeline(3, 8);
    expect(request).toHaveBeenLastCalledWith('/courses/3/course_pacing/8/timeline');
    expect(tl).toHaveLength(1);

    request.mockResolvedValue({ data: { id: 2 } });
    await createBlackoutDate(3, { event_title: 'Break', start_date: '2026-09-07', end_date: '2026-09-08' });
    const [path, opts] = request.mock.calls.at(-1);
    expect(path).toBe('/courses/3/blackout_dates');
    expect(opts.method).toBe('POST');
    expect(JSON.parse(opts.body)).toEqual({ blackout_date: { event_title: 'Break', start_date: '2026-09-07', end_date: '2026-09-08' } });
  });

  test('builds the LTI login-initiation form for a tool item', () => {
    const form = buildToolLaunchForm(
      { client_id: 'abc', target_link_uri: 'https://tool.example.org/x', lti_message_hint: '3:module_item_9:4' },
      { newTab: true },
    );
    expect(form.method.toLowerCase()).toBe('post');
    expect(form.action).toMatch(/\/lti\/oidc\/login$/);
    expect(form.target).toBe('_blank');
    const fields = Object.fromEntries([...form.querySelectorAll('input')].map((i) => [i.name, i.value]));
    expect(fields).toEqual({ client_id: 'abc', target_link_uri: 'https://tool.example.org/x', lti_message_hint: '3:module_item_9:4' });
  });
});
