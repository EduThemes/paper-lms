import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import CourseStaffBar from '../CourseStaffBar';
import { api } from '../../services/api';
import { pagesFilesApi } from '../../services/api_pages_files';

vi.mock('../../services/api', () => ({ api: { updateCourse: vi.fn() } }));
vi.mock('../../services/api_pages_files', () => ({ pagesFilesApi: { enterStudentView: vi.fn() } }));

describe('CourseStaffBar (G076 / G077)', () => {
  const assign = vi.fn();
  const realLocation = window.location;
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(window, 'location', { configurable: true, value: { ...realLocation, assign } });
  });
  afterEach(() => {
    Object.defineProperty(window, 'location', { configurable: true, value: realLocation });
  });

  test('publishes a new course through every state step', async () => {
    api.updateCourse
      .mockResolvedValueOnce({ id: 5, workflow_state: 'unpublished' })
      .mockResolvedValueOnce({ id: 5, workflow_state: 'available' });
    const onCourseChange = vi.fn();
    render(<CourseStaffBar course={{ id: 5, workflow_state: 'created' }} onCourseChange={onCourseChange} />);

    expect(screen.getByText('Unpublished')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Publish course/i }));

    await waitFor(() => expect(onCourseChange).toHaveBeenCalledWith({ id: 5, workflow_state: 'available' }));
    expect(api.updateCourse.mock.calls).toEqual([
      [5, { workflow_state: 'unpublished' }],
      [5, { workflow_state: 'available' }],
    ]);
  });

  test('shows the server refusal when unpublishing is blocked', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    api.updateCourse.mockRejectedValue(new Error("This course can't be unpublished because students have graded submissions in it"));
    render(<CourseStaffBar course={{ id: 5, workflow_state: 'available' }} />);

    expect(screen.queryByRole('button', { name: /Publish course/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^Unpublish$/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/graded submissions/i);
  });

  test('enters Student View and reloads the course as the test student', async () => {
    pagesFilesApi.enterStudentView.mockResolvedValue({ student_view: true, course_id: 5 });
    render(<CourseStaffBar course={{ id: 5, workflow_state: 'available' }} />);

    fireEvent.click(screen.getByRole('button', { name: /Student View/i }));

    await waitFor(() => expect(assign).toHaveBeenCalledWith('/courses/5'));
    expect(pagesFilesApi.enterStudentView).toHaveBeenCalledWith(5);
  });
});
