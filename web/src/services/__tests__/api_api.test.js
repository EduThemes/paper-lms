import {
  buildCanvasQuery,
  expiryToISO,
  listAccountUsers,
  tokenStatus,
  updateAccessToken,
} from '../api_api';
import { api } from '../api';

vi.mock('../api', () => ({
  api: { request: vi.fn() },
  requestPaged: vi.fn(),
}));

describe('buildCanvasQuery', () => {
  it('repeats array params Canvas-style and drops empty values', () => {
    expect(buildCanvasQuery({ include: ['term', 'teachers'], search_term: 'bio lab', page: 2, empty: '', none: null })).toBe(
      '?include%5B%5D=term&include%5B%5D=teachers&search_term=bio%20lab&page=2',
    );
    expect(buildCanvasQuery({})).toBe('');
    expect(buildCanvasQuery({ include: [] })).toBe('');
  });
});

describe('tokenStatus', () => {
  const now = new Date('2026-03-01T12:00:00Z').getTime();
  it('classifies personal tokens by expiry', () => {
    expect(tokenStatus({ kind: 'personal', expires_at: null }, now)).toBe('active');
    expect(tokenStatus({ kind: 'personal', expires_at: '2026-02-28T00:00:00Z' }, now)).toBe('expired');
    expect(tokenStatus({ kind: 'personal', expires_at: '2026-03-04T00:00:00Z' }, now)).toBe('expiring');
    expect(tokenStatus({ kind: 'personal', expires_at: '2026-06-01T00:00:00Z' }, now)).toBe('active');
    expect(tokenStatus({ kind: 'personal', workflow_state: 'expired' }, now)).toBe('expired');
  });
  it('never marks an OAuth app token as expired', () => {
    expect(tokenStatus({ kind: 'oauth', expires_at: '2020-01-01T00:00:00Z' }, now)).toBe('integration');
  });
});

describe('expiryToISO', () => {
  it('turns a date input into the end of that local day', () => {
    const iso = expiryToISO('2026-05-10');
    const d = new Date(iso);
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(4);
    expect(d.getDate()).toBe(10);
    expect(d.getHours()).toBe(23);
    expect(expiryToISO('')).toBeUndefined();
    expect(expiryToISO('garbage')).toBeUndefined();
  });
});

describe('request shapes', () => {
  beforeEach(() => api.request.mockReset());

  it('searches account users with Canvas params', async () => {
    api.request.mockResolvedValue({ data: [{ id: 1 }] });
    const users = await listAccountUsers('self', { search_term: 'ann', include: ['email'] });
    expect(users).toEqual([{ id: 1 }]);
    expect(api.request).toHaveBeenCalledWith('/accounts/self/users?per_page=20&search_term=ann&include%5B%5D=email');
  });

  it('regenerates a token through PUT token[regenerate]', async () => {
    api.request.mockResolvedValue({ data: { id: 4, token: 'new' } });
    await updateAccessToken(7, 4, { regenerate: true });
    expect(api.request).toHaveBeenCalledWith('/users/7/tokens/4', {
      method: 'PUT',
      body: JSON.stringify({ token: { regenerate: true } }),
    });
  });
});
