import React from 'react';
import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, act, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useNavigate } from 'react-router-dom';

vi.mock('../../services/api_learner_outcomes', () => ({
  recordPageView: vi.fn(),
  sendPageViewSeconds: vi.fn(),
}));

import { recordPageView, sendPageViewSeconds } from '../../services/api_learner_outcomes';
import CoursePageViewTracker from './CoursePageViewTracker';

let navigate;
function NavGrab() {
  navigate = useNavigate();
  return null;
}

function renderAt(path) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <CoursePageViewTracker />
      <Routes><Route path="*" element={<NavGrab />} /></Routes>
    </MemoryRouter>,
  );
}

describe('CoursePageViewTracker (G154)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    recordPageView.mockResolvedValue({ id: 77 });
  });

  test('records each course page and reports time on page when leaving it', async () => {
    renderAt('/courses/12/assignments/3');
    await waitFor(() => expect(recordPageView).toHaveBeenCalledWith({ courseId: '12', url: '/courses/12/assignments/3', category: 'assignments' }));
    await act(async () => { await Promise.resolve(); });

    await act(async () => { navigate('/courses/12/pages/intro'); });
    expect(sendPageViewSeconds).toHaveBeenCalledWith(77, expect.any(Number));
    await waitFor(() => expect(recordPageView).toHaveBeenLastCalledWith({ courseId: '12', url: '/courses/12/pages/intro', category: 'pages' }));
  });

  test('does not track pages outside a course', async () => {
    renderAt('/dashboard');
    await act(async () => { await Promise.resolve(); });
    expect(recordPageView).not.toHaveBeenCalled();
  });

  test('a failed report never throws', async () => {
    recordPageView.mockRejectedValue(new Error('offline'));
    renderAt('/courses/5');
    await waitFor(() => expect(recordPageView).toHaveBeenCalled());
  });
});
