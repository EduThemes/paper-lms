import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { api, ApiError } from '../api';

// Focused tests for the unified fetchApi helper + ApiError type. The
// broader call-site coverage lives in services/api.test.js — these
// tests lock the typed-error shape and the 401 session-expired
// dispatch so callers can rely on them.

describe('ApiError', () => {
  test('constructor preserves status, body, errors[], and code', () => {
    const body = {
      errors: [
        { code: 'not_found', message: 'No such snapshot' },
        { code: 'extra', message: 'second' },
      ],
    };
    const err = new ApiError({ status: 404, body });

    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.name).toBe('ApiError');
    expect(err.status).toBe(404);
    expect(err.body).toBe(body);
    expect(err.errors).toEqual(body.errors);
    expect(err.code).toBe('not_found');
    expect(err.message).toBe('No such snapshot');
  });

  test('message falls back to status when body has no errors[0].message', () => {
    const err = new ApiError({ status: 500, body: {} });

    expect(err.message).toBe('Request failed: 500');
    expect(err.errors).toEqual([]);
    expect(err.code).toBeUndefined();
  });

  test('message falls back to status when body is missing entirely', () => {
    const err = new ApiError({ status: 502 });

    expect(err.message).toBe('Request failed: 502');
    expect(err.body).toBeNull();
    expect(err.errors).toEqual([]);
  });

  test('explicit message argument wins over body', () => {
    const err = new ApiError({
      status: 400,
      body: { errors: [{ code: 'x', message: 'from body' }] },
      message: 'explicit override',
    });

    expect(err.message).toBe('explicit override');
    // Other typed fields still populate from body.
    expect(err.code).toBe('x');
    expect(err.errors).toHaveLength(1);
  });
});

describe('fetchApi (via api.* helpers)', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  test('throws ApiError with typed status and code on error response', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: async () => ({
        errors: [{ code: 'snapshot_missing', message: 'Not published yet' }],
      }),
      headers: new Headers(),
    });

    let caught;
    try {
      await api.getSelf();
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(ApiError);
    expect(caught.status).toBe(404);
    expect(caught.code).toBe('snapshot_missing');
    expect(caught.message).toBe('Not published yet');
  });

  test('401 dispatches auth:session-expired event', async () => {
    const handler = vi.fn();
    window.addEventListener('auth:session-expired', handler);

    global.fetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({ errors: [{ message: 'session gone' }] }),
      headers: new Headers(),
    });

    await expect(api.getSelf()).rejects.toBeInstanceOf(ApiError);

    expect(handler).toHaveBeenCalledTimes(1);
    window.removeEventListener('auth:session-expired', handler);
  });

  test('error message falls back when body has no errors[0].message', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      // Body parses but contains no `errors` array.
      json: async () => ({}),
      headers: new Headers(),
    });

    let caught;
    try {
      await api.getSelf();
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(ApiError);
    expect(caught.status).toBe(500);
    expect(caught.message).toBe('Request failed: 500');
    expect(caught.code).toBeUndefined();
    expect(caught.errors).toEqual([]);
  });

  test('error message falls back when body is unparseable', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: async () => { throw new Error('not json'); },
      headers: new Headers(),
    });

    let caught;
    try {
      await api.getSelf();
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(ApiError);
    expect(caught.status).toBe(503);
    expect(caught.message).toBe('Request failed: 503');
  });

  test('multipart upload omits Content-Type so browser sets the boundary', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: 1 }),
      headers: new Headers(),
    });

    const file = new Blob(['hi'], { type: 'text/plain' });
    await api.uploadCourseFile(7, file);

    const [, options] = global.fetch.mock.calls[0];
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBeUndefined();
    // CSRF is still set on multipart uploads.
    expect(options.headers).toHaveProperty('X-CSRF-Token');
    expect(options.body).toBeInstanceOf(FormData);
  });
});
