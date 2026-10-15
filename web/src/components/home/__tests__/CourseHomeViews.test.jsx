/* global vi, describe, test, expect, beforeEach */
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { CourseFrontPageView, CourseAnnouncementsView } from '../CourseHomeViews';
import { api } from '../../../services/api';

vi.mock('../../../services/api', () => ({
  api: {
    getCourseFrontPage: vi.fn(),
    getCourseAnnouncements: vi.fn(),
  },
}));

function renderWithQuery(ui) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

const notFound = () => Object.assign(new Error('not found'), { status: 404 });

// B147: the "Front Page" and "Announcements" course home settings render
// their content instead of falling back to the modules list.
describe('CourseHomeViews', () => {
  beforeEach(() => vi.clearAllMocks());

  test('front page view renders the course front page', async () => {
    api.getCourseFrontPage.mockResolvedValue({ title: 'Welcome to Biology', body: '<p>Start with Unit 1.</p>' });
    renderWithQuery(<CourseFrontPageView courseId="4" />);
    expect(await screen.findByText('Welcome to Biology')).toBeInTheDocument();
    expect(screen.getByText('Start with Unit 1.')).toBeInTheDocument();
    expect(api.getCourseFrontPage).toHaveBeenCalledWith('4');
  });

  test('front page view explains when no front page is set', async () => {
    api.getCourseFrontPage.mockRejectedValue(notFound());
    renderWithQuery(<CourseFrontPageView courseId="4" />);
    expect(await screen.findByText(/no front page yet/i)).toBeInTheDocument();
  });

  test('front page view offers Try Again when the page fails to load', async () => {
    api.getCourseFrontPage.mockRejectedValueOnce(Object.assign(new Error('boom'), { status: 500 }));
    renderWithQuery(<CourseFrontPageView courseId="4" />);
    const retry = await screen.findByRole('button', { name: 'Try Again' });
    api.getCourseFrontPage.mockResolvedValue({ title: 'Home', body: '<p>Back.</p>' });
    fireEvent.click(retry);
    expect(await screen.findByText('Back.')).toBeInTheDocument();
  });

  test('a designed front page renders as designed: no card title, no prose wrapper', async () => {
    api.getCourseFrontPage.mockResolvedValue({
      page_id: 12,
      url: 'home',
      title: 'Home',
      body: '<div class="paper-design"><h2>Welcome, explorers</h2></div>',
      scoped_css: '#paper-page-12 h2 { color: #12355b; }',
      design: { version: 1, id: 'doc-1' },
    });
    const { container } = renderWithQuery(<CourseFrontPageView courseId="4" isTeacher />);
    const region = await screen.findByRole('region', { name: 'Home' });
    expect(region).toHaveAttribute('data-testid', 'designed-front-page');
    expect(screen.getByRole('heading', { name: 'Welcome, explorers' })).toBeInTheDocument();
    expect(container.querySelector('#paper-page-12')).not.toBeNull();
    expect(container.querySelector('.prose')).toBeNull();
    expect(container.querySelector('style').textContent).toBe('#paper-page-12 h2 { color: #12355b; }');
    // Teachers get an edit bar pointing at the designer.
    expect(screen.getByRole('link', { name: /Edit design/i })).toHaveAttribute('href', '/courses/4/designer?page=home');
  });

  test('students see no edit bar on a designed front page', async () => {
    api.getCourseFrontPage.mockResolvedValue({
      page_id: 12, url: 'home', title: 'Home', body: '<p>Hi</p>', design: { version: 1, id: 'd' },
    });
    renderWithQuery(<CourseFrontPageView courseId="4" />);
    expect(await screen.findByText('Hi')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Edit design/i })).toBeNull();
  });

  test('an ordinary front page with page CSS keeps the prose card, scoped', async () => {
    api.getCourseFrontPage.mockResolvedValue({
      page_id: 7, url: 'home', title: 'Home', body: '<p class="note">Note</p>',
      scoped_css: '#paper-page-7 .note { color: red; }', design: null,
    });
    const { container } = renderWithQuery(<CourseFrontPageView courseId="4" />);
    expect(await screen.findByText('Note')).toBeInTheDocument();
    const scope = container.querySelector('#paper-page-7');
    expect(scope).not.toBeNull();
    expect(scope.querySelector('.prose')).not.toBeNull();
    expect(container.querySelector('style').textContent).toContain('.note');
  });

  test('announcements view lists recent announcements', async () => {
    api.getCourseAnnouncements.mockResolvedValue({ data: [{ id: 1, title: 'Field trip Friday', message: 'Bring a lunch.' }] });
    render(<MemoryRouter><CourseAnnouncementsView courseId="4" /></MemoryRouter>);
    expect(await screen.findByText('Field trip Friday')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /All announcements/i })).toHaveAttribute('href', '/courses/4/announcements');
  });
});
