import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import MobileBottomNav from './MobileBottomNav';

const renderAt = (path, props = {}) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <MobileBottomNav {...props} />
    </MemoryRouter>,
  );

describe('MobileBottomNav', () => {
  test('renders 5 nav items', () => {
    renderAt('/');
    const nav = screen.getByRole('navigation', { name: /primary/i });
    expect(nav.querySelectorAll('a')).toHaveLength(5);
  });

  test('marks active route with aria-current="page"', () => {
    renderAt('/courses');
    const link = screen.getByRole('link', { name: /courses/i });
    expect(link).toHaveAttribute('aria-current', 'page');
  });

  test('renders notification dot when count > 0', () => {
    renderAt('/', { notificationCount: 3 });
    const inbox = screen.getByRole('link', { name: /inbox \(3 unread\)/i });
    expect(inbox.querySelector('span.bg-accent-danger')).toBeTruthy();
  });
});
