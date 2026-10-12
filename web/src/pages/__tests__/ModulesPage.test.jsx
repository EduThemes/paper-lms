import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { TooltipProvider } from '../../components/ui/tooltip';
import ModulesPage from '../ModulesPage';
import { api } from '../../services/api';

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
  default: () => true,
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
  return render(
    <TooltipProvider>
      <MemoryRouter initialEntries={['/courses/1/modules']}>
        <Routes>
          <Route path="/courses/:courseId/modules" element={<ModulesPage />} />
        </Routes>
      </MemoryRouter>
    </TooltipProvider>,
  );
}

describe('ModulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  test('renders inline error banner when getModules rejects', async () => {
    api.getModules.mockRejectedValue(new Error('modules failed'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('modules failed')).toBeInTheDocument();
    });
  });
});
