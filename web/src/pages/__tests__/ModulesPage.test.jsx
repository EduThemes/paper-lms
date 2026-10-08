import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '../../components/ui/tooltip';
import ModulesPage from '../ModulesPage';
import { api } from '../../services/api';
import * as modulesApi from '../../services/api_modules';
import useIsTeacher from '../../hooks/useIsTeacher';

vi.mock('../../services/api', () => ({
  api: {
    getModules: vi.fn(),
    getModulePrerequisites: vi.fn(),
    getEnrollments: vi.fn(),
    getAssignments: vi.fn(),
    createModuleItem: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'admin' },
    loading: false,
  }),
}));

vi.mock('../../hooks/useIsTeacher', () => ({
  default: vi.fn(() => true),
}));

const markDoneMutate = vi.fn();
vi.mock('../../services/api_modules', () => ({
  useModuleProgress: vi.fn(() => ({ data: undefined, refetch: vi.fn() })),
  useMarkModuleItemDone: vi.fn(() => ({ mutate: markDoneMutate })),
  markModuleItemRead: vi.fn(() => Promise.resolve()),
  launchModuleTool: vi.fn(() => Promise.resolve()),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

vi.mock('../../components/ModuleSettingsModal', () => ({
  default: () => null,
}));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <MemoryRouter initialEntries={['/courses/1/modules']}>
          <Routes>
            <Route path="/courses/:courseId/modules" element={<ModulesPage />} />
          </Routes>
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

describe('ModulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useIsTeacher.mockReturnValue(true);
    modulesApi.useModuleProgress.mockReturnValue({ data: undefined, refetch: vi.fn() });
    api.getModulePrerequisites.mockResolvedValue({ prerequisite_module_ids: [] });
  });

  test('renders skeleton while modules are loading', () => {
    api.getModules.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders module names after fetch resolves', async () => {
    api.getModules.mockResolvedValue({
      data: [
        { id: 1, name: 'Week 1', position: 1, published: true, items: [] },
        { id: 2, name: 'Week 2', position: 2, published: true, items: [] },
      ],
    });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Week 1')).toBeInTheDocument();
    });
    expect(screen.getByText('Week 2')).toBeInTheDocument();
  });

  test('shows empty state when no modules exist', async () => {
    api.getModules.mockResolvedValue({ data: [] });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No modules yet/i)).toBeInTheDocument();
    });
  });

  // B042: adding an Assignment item links the chosen assignment.
  test('adding an assignment item sends the chosen content_id', async () => {
    api.getModules.mockResolvedValue({
      data: [{ id: 1, name: 'Week 1', position: 1, published: true, items: [] }],
    });
    api.getAssignments.mockResolvedValue({ data: [{ id: 42, name: 'Essay 1' }] });
    api.createModuleItem.mockResolvedValue({ id: 9 });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /Add item/i }));

    const picker = await screen.findByRole('combobox', { name: /Choose assignment/i });
    await waitFor(() => expect(screen.getByRole('option', { name: 'Essay 1' })).toBeInTheDocument());
    fireEvent.change(picker, { target: { value: '42' } });
    const submit = screen.getAllByRole('button', { name: /^Add Item$/i }).find((b) => b.getAttribute('type') === 'submit');
    fireEvent.click(submit);

    await waitFor(() => expect(api.createModuleItem).toHaveBeenCalled());
    const [, moduleId, payload] = api.createModuleItem.mock.calls[0];
    expect(moduleId).toBe(1);
    expect(payload).toMatchObject({ type: 'Assignment', content_id: 42 });
  });

  // B146: imported file items are clickable.
  test('imported file items link to the file download', async () => {
    api.getModules.mockResolvedValue({
      data: [{ id: 1, name: 'Week 1', position: 1, published: true, items: [
        { id: 5, title: 'Handout', type: 'File', content_id: 77, published: true },
      ] }],
    });
    renderPage();
    const link = await screen.findByRole('link', { name: /Handout/i });
    expect(link.getAttribute('href')).toMatch(/\/files\/77\/download$/);
  });

  // G061/G062: the student view shows module state, requirement check
  // marks, locked items without links, and "Mark as done".
  test('student view reflects module progress and locks', async () => {
    useIsTeacher.mockReturnValue(false);
    api.getModules.mockResolvedValue({
      data: [
        { id: 1, name: 'Week 1', position: 1, published: true, items: [
          { id: 11, title: 'Welcome', type: 'Page', page_url: 'welcome', published: true, completion_type: 'must_view' },
          { id: 12, title: 'Reading notes', type: 'Page', page_url: 'reading', published: true, completion_type: 'must_mark_done' },
        ] },
        { id: 2, name: 'Week 2', position: 2, published: true, items: [
          { id: 21, title: 'Lab report', type: 'Assignment', content_id: 5, published: true, completion_type: 'min_score', min_score: 7 },
        ] },
      ],
    });
    modulesApi.useModuleProgress.mockReturnValue({
      refetch: vi.fn(),
      data: {
        student: true,
        modules: [
          { module_id: 1, state: 'started', lock_explanation: '', items: [
            { item_id: 11, completed: true, locked: false, requirement_type: 'must_view' },
            { item_id: 12, completed: false, locked: false, requirement_type: 'must_mark_done' },
          ] },
          { module_id: 2, state: 'locked', lock_explanation: 'Complete "Week 1" to unlock this module.', items: [
            { item_id: 21, completed: false, locked: true, requirement_type: 'min_score', min_score: 7 },
          ] },
        ],
      },
    });

    renderPage();
    expect(await screen.findByText('In progress')).toBeInTheDocument();
    expect(screen.getByText('Locked')).toBeInTheDocument();
    expect(screen.getByText('Complete "Week 1" to unlock this module.')).toBeInTheDocument();
    expect(screen.getByTestId('requirement-11')).toHaveTextContent('View');
    expect(screen.getByTestId('requirement-11')).toHaveTextContent('(completed)');
    expect(screen.getByTestId('requirement-21')).toHaveTextContent('Score at least 7');

    // The locked item is not a link.
    expect(screen.queryByRole('link', { name: /Lab report/i })).toBeNull();
    expect(screen.getByRole('link', { name: /Welcome/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Mark as done' }));
    expect(markDoneMutate).toHaveBeenCalledWith({ moduleId: 1, itemId: 12, done: true }, expect.any(Object));
  });

  test('teachers see requirements and the student progress link', async () => {
    api.getModules.mockResolvedValue({
      data: [{ id: 1, name: 'Week 1', position: 1, published: true, items: [
        { id: 11, title: 'Welcome', type: 'Page', page_url: 'welcome', published: true, completion_type: 'must_view' },
      ] }],
    });
    renderPage();
    expect(await screen.findByTestId('requirement-11')).toHaveTextContent('View');
    expect(screen.getByRole('link', { name: /Student progress/i })).toHaveAttribute('href', '/courses/1/modules/progressions');
    expect(screen.queryByRole('button', { name: 'Mark as done' })).toBeNull();
  });

  // G064: File items pick an existing course file (no "new" option).
  test('adding a File item requires choosing a course file', async () => {
    api.getModules.mockResolvedValue({
      data: [{ id: 1, name: 'Week 1', position: 1, published: true, items: [] }],
    });
    api.getAssignments.mockResolvedValue({ data: [] });
    api.getCourseFiles = vi.fn().mockResolvedValue({ data: [{ id: 33, display_name: 'syllabus.pdf' }] });
    api.createModuleItem.mockResolvedValue({ id: 9 });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /Add item/i }));
    fireEvent.change(screen.getByRole('combobox', { name: /Item type/i }), { target: { value: 'File' } });
    const picker = await screen.findByRole('combobox', { name: /Choose file/i });
    await waitFor(() => expect(screen.getByRole('option', { name: 'syllabus.pdf' })).toBeInTheDocument());
    const submit = screen.getAllByRole('button', { name: /^Add Item$/i }).find((b) => b.getAttribute('type') === 'submit');
    expect(submit).toBeDisabled();
    fireEvent.change(picker, { target: { value: '33' } });
    fireEvent.click(submit);
    await waitFor(() => expect(api.createModuleItem).toHaveBeenCalled());
    expect(api.createModuleItem.mock.calls[0][2]).toMatchObject({ type: 'File', content_id: 33 });
  });

  test('renders inline error banner when getModules rejects', async () => {
    api.getModules.mockRejectedValue(new Error('modules failed'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('modules failed')).toBeInTheDocument();
    });
  });
});
