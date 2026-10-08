import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { CourseFrontPageView, CourseAnnouncementsView } from '../CourseHomeViews';
import { api } from '../../../services/api';

vi.mock('../../../services/api', () => ({
  api: {
    getCourseFrontPage: vi.fn(),
    getCourseAnnouncements: vi.fn(),
  },
}));

// B147: the "Front Page" and "Announcements" course home settings render
// their content instead of falling back to the modules list.
describe('CourseHomeViews', () => {
  beforeEach(() => vi.clearAllMocks());

  test('front page view renders the course front page', async () => {
    api.getCourseFrontPage.mockResolvedValue({ title: 'Welcome to Biology', body: '<p>Start with Unit 1.</p>' });
    render(<MemoryRouter><CourseFrontPageView courseId="4" /></MemoryRouter>);
    expect(await screen.findByText('Welcome to Biology')).toBeInTheDocument();
    expect(screen.getByText('Start with Unit 1.')).toBeInTheDocument();
    expect(api.getCourseFrontPage).toHaveBeenCalledWith('4');
  });

  test('front page view explains when no front page is set', async () => {
    api.getCourseFrontPage.mockRejectedValue(new Error('not found'));
    render(<MemoryRouter><CourseFrontPageView courseId="4" /></MemoryRouter>);
    expect(await screen.findByText(/no front page yet/i)).toBeInTheDocument();
  });

  test('announcements view lists recent announcements', async () => {
    api.getCourseAnnouncements.mockResolvedValue({ data: [{ id: 1, title: 'Field trip Friday', message: 'Bring a lunch.' }] });
    render(<MemoryRouter><CourseAnnouncementsView courseId="4" /></MemoryRouter>);
    expect(await screen.findByText('Field trip Friday')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /All announcements/i })).toHaveAttribute('href', '/courses/4/announcements');
  });
});
