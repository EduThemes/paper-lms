import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { api } from './api';

const API_URL = '/api/v1';

describe('api service', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // Helper to create a mock successful fetch response
  function mockFetchSuccess(data, headers = new Headers()) {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => data,
      headers,
    });
  }

  // Helper to create a mock error fetch response
  function mockFetchError(status, body = {}) {
    global.fetch.mockResolvedValueOnce({
      ok: false,
      status,
      json: async () => body,
      headers: new Headers(),
    });
  }

  test('request success returns data', async () => {
    mockFetchSuccess({ token: 'abc', user: { id: 1, name: 'Test User' } });

    const result = await api.login('test@example.com', 'password123');

    expect(result).toHaveProperty('user');
    expect(result.user.id).toBe(1);
    expect(result.token).toBe('abc');
  });

  test('request error parses error message from response body', async () => {
    mockFetchError(422, {
      errors: [{ message: 'Invalid email or password' }],
    });

    await expect(api.login('bad@example.com', 'wrong')).rejects.toThrow(
      'Invalid email or password'
    );
  });

  test('request error falls back to status code when no error body', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => { throw new Error('no json'); },
      headers: new Headers(),
    });

    await expect(api.getSelf()).rejects.toThrow('Request failed: 500');
  });

  test('request network error propagates', async () => {
    global.fetch.mockRejectedValueOnce(new Error('Network failure'));

    await expect(api.getSelf()).rejects.toThrow('Network failure');
  });

  test('login sends correct URL and body', async () => {
    mockFetchSuccess({ token: 'abc', user: { id: 1 } });

    await api.login('user@test.com', 'mypassword');

    expect(global.fetch).toHaveBeenCalledTimes(1);
    const [url, options] = global.fetch.mock.calls[0];
    expect(url).toBe(`${API_URL}/login`);
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({
      email: 'user@test.com',
      password: 'mypassword',
    });
  });

  test('register sends correct body', async () => {
    mockFetchSuccess({ user: { id: 2, name: 'New User' } });

    await api.register('New User', 'new@test.com', 'pass123');

    const [url, options] = global.fetch.mock.calls[0];
    expect(url).toBe(`${API_URL}/register`);
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({
      name: 'New User',
      email: 'new@test.com',
      password: 'pass123',
    });
  });

  test('getSelf calls correct endpoint', async () => {
    mockFetchSuccess({ id: 1, name: 'Self User', email: 'self@test.com' });

    const result = await api.getSelf();

    const [url, options] = global.fetch.mock.calls[0];
    expect(url).toBe(`${API_URL}/users/self`);
    expect(options.method).toBeUndefined(); // GET by default
    expect(result).toEqual({ id: 1, name: 'Self User', email: 'self@test.com' });
  });

  test('getCourses includes pagination from Link header', async () => {
    const linkHeader =
      '<http://localhost/api/v1/courses?page=2&per_page=10>; rel="next", ' +
      '<http://localhost/api/v1/courses?page=1&per_page=10>; rel="current"';

    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [{ id: 1, name: 'Course 1' }],
      headers: new Headers({ Link: linkHeader }),
    });

    const result = await api.getCourses(1, 10);

    expect(result.data).toEqual([{ id: 1, name: 'Course 1' }]);
    expect(result.pagination).toHaveProperty('next');
    expect(result.pagination.next).toBe(
      'http://localhost/api/v1/courses?page=2&per_page=10'
    );
    expect(result.pagination).toHaveProperty('current');
    expect(result.pagination.current).toBe(
      'http://localhost/api/v1/courses?page=1&per_page=10'
    );
  });

  test('createCourse sends nested course body', async () => {
    mockFetchSuccess({ id: 5, name: 'New Course' });

    await api.createCourse({ name: 'New Course', course_code: 'NC101' });

    const [url, options] = global.fetch.mock.calls[0];
    expect(url).toBe(`${API_URL}/courses`);
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({
      course: { name: 'New Course', course_code: 'NC101' },
    });
  });

  test('createSubmission sends correct body', async () => {
    mockFetchSuccess({ id: 10, submission_type: 'online_text_entry' });

    await api.createSubmission(1, 2, {
      submission_type: 'online_text_entry',
      body: 'My submission text',
    });

    const [url, options] = global.fetch.mock.calls[0];
    expect(url).toBe(`${API_URL}/courses/1/assignments/2/submissions`);
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({
      submission: {
        submission_type: 'online_text_entry',
        body: 'My submission text',
      },
    });
  });

  test('credentials: include is sent with every request', async () => {
    mockFetchSuccess({ id: 1 });

    await api.getSelf();

    const [, options] = global.fetch.mock.calls[0];
    expect(options.credentials).toBe('include');
  });

  test('Content-Type application/json header is sent with every request', async () => {
    mockFetchSuccess({ id: 1 });

    await api.getSelf();

    const [, options] = global.fetch.mock.calls[0];
    expect(options.headers).toHaveProperty('Content-Type', 'application/json');
  });

  test('parseLinkHeader handles multiple rels via getCourses', async () => {
    const linkHeader =
      '<http://localhost/api/v1/courses?page=3&per_page=10>; rel="next", ' +
      '<http://localhost/api/v1/courses?page=1&per_page=10>; rel="prev", ' +
      '<http://localhost/api/v1/courses?page=1&per_page=10>; rel="first", ' +
      '<http://localhost/api/v1/courses?page=5&per_page=10>; rel="last"';

    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [],
      headers: new Headers({ Link: linkHeader }),
    });

    const result = await api.getCourses(2, 10);

    expect(result.pagination.next).toBe(
      'http://localhost/api/v1/courses?page=3&per_page=10'
    );
    expect(result.pagination.prev).toBe(
      'http://localhost/api/v1/courses?page=1&per_page=10'
    );
    expect(result.pagination.first).toBe(
      'http://localhost/api/v1/courses?page=1&per_page=10'
    );
    expect(result.pagination.last).toBe(
      'http://localhost/api/v1/courses?page=5&per_page=10'
    );
  });

  test('pagination is empty object when no Link header present', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [{ id: 1 }],
      headers: new Headers(),
    });

    const result = await api.getCourses();

    expect(result.pagination).toEqual({});
  });
});
