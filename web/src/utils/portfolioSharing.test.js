import { test, expect } from 'vitest';
import { sharedPortfolioUrl, withShareToken } from './portfolioSharing';

test('shared portfolio link and token-scoped API paths (G155)', () => {
  expect(sharedPortfolioUrl('abc-123', 'https://lms.example.test')).toBe('https://lms.example.test/portfolios/shared/abc-123');
  expect(sharedPortfolioUrl('')).toBe('');
  expect(withShareToken('/portfolios/4/comments', 'a b')).toBe('/portfolios/4/comments?share_token=a%20b');
  expect(withShareToken('/portfolios/4/comments?page=1', 'tok')).toBe('/portfolios/4/comments?page=1&share_token=tok');
  expect(withShareToken('/portfolios/4/comments', undefined)).toBe('/portfolios/4/comments');
});
