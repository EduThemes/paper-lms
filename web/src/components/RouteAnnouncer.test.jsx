import React from 'react';
import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, screen, act, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route, Link } from 'react-router-dom';
import RouteAnnouncer from './RouteAnnouncer';
import { titleForPath, ROUTE_TITLES } from '../utils/routeTitles';
import appSource from '../App.jsx?raw';

afterEach(() => vi.useRealTimers());

describe('route titles (G159)', () => {
  test('every route has a distinct, specific title', () => {
    expect(titleForPath('/')).toBe('Dashboard');
    expect(titleForPath('/courses/12/assignments/4')).toBe('Assignment');
    expect(titleForPath('/courses/12/assignments')).toBe('Assignments');
    expect(titleForPath('/courses/12/quizzes/3/take')).toBe('Take quiz');
    expect(titleForPath('/courses/12')).toBe('Course home');
    expect(titleForPath('/nope/nothing')).toBe('Page not found');
    const patterns = ROUTE_TITLES.map(([p]) => p);
    expect(new Set(patterns).size).toBe(patterns.length);
  });

  test('every route in App.jsx has a title', () => {
    // Nested routes sit under /courses/:courseId; "*" routes are the
    // not-found catch-alls.
    const paths = [...appSource.matchAll(/path="([^"]+)"/g)]
      .map(([, p]) => (p.startsWith('/') ? p : `/courses/:courseId/${p}`))
      .filter((p) => !p.endsWith('*'));
    const untitled = paths.filter((p) => titleForPath(p.replace(/:\w+/g, '7')) === 'Page not found');
    expect(untitled).toEqual([]);
  });
});

function Shell() {
  return (
    <>
      <RouteAnnouncer />
      <Routes>
        <Route path="/" element={<main id="main-content"><h1>Home</h1><Link to="/courses">Go</Link></main>} />
        <Route path="/courses" element={<main id="main-content"><h1>Courses heading</h1></main>} />
      </Routes>
    </>
  );
}

test('route change sets the title, moves focus to the heading and announces', () => {
  vi.useFakeTimers();
  render(<MemoryRouter initialEntries={['/']}><Shell /></MemoryRouter>);
  expect(document.title).toBe('Dashboard · Paper LMS');
  const link = screen.getByRole('link', { name: 'Go' });
  link.focus();
  fireEvent.click(link);
  act(() => { vi.advanceTimersByTime(200); });
  expect(document.title).toBe('All courses · Paper LMS');
  expect(document.activeElement).toBe(screen.getByRole('heading', { name: 'Courses heading' }));
  expect(screen.getByTestId('route-announcer').textContent).toBe('All courses · Paper LMS');
});
