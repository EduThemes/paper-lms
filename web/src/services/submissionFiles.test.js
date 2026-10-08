import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { api } from './api';

// B013: students upload submission files through the submission-file
// endpoint (the course files route is instructor-only).
describe('api.uploadSubmissionFile', () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ id: 77, display_name: 'essay.txt' }),
      headers: new Headers(),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  test('posts multipart to the assignment submission-files route', async () => {
    const file = new File(['my essay'], 'essay.txt', { type: 'text/plain' });
    await api.uploadSubmissionFile(3, 9, file);

    expect(global.fetch).toHaveBeenCalledTimes(1);
    const [url, opts] = global.fetch.mock.calls[0];
    expect(url).toBe('/api/v1/courses/3/assignments/9/submissions/self/files');
    expect(url).not.toContain('/courses/3/files');
    expect(opts.method).toBe('POST');
    expect(opts.body).toBeInstanceOf(FormData);
    expect(opts.body.get('file')).toBeInstanceOf(File);
  });
});
