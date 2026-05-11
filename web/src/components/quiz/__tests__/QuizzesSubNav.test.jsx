import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import QuizzesSubNav from '../QuizzesSubNav';

function renderAt(path, opts = {}) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/courses/:courseId/*" element={<QuizzesSubNav {...opts} />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('QuizzesSubNav', () => {
  it('renders the three core tabs', () => {
    renderAt('/courses/1/quizzes');
    expect(screen.getByText('Quizzes')).toBeInTheDocument();
    expect(screen.getByText('Item Banks')).toBeInTheDocument();
    expect(screen.getByText('Stimuli')).toBeInTheDocument();
    expect(screen.queryByText('Item Analysis')).toBeNull();
  });

  it('reveals the Item Analysis tab when a quizId is supplied', () => {
    renderAt('/courses/1/quizzes/9/edit', { quizId: 9 });
    expect(screen.getByText('Item Analysis')).toBeInTheDocument();
  });

  it('highlights the active tab based on the current pathname', () => {
    renderAt('/courses/1/item-banks');
    const link = screen.getByText('Item Banks').closest('a');
    expect(link.className).toMatch(/border-brand-600/);
  });
});
