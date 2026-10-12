import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppointmentGroupEditorPage from '../AppointmentGroupEditorPage';
import { api } from '../../services/api';
import { calendarApi } from '../../services/api_calendar';

vi.mock('../../services/api', () => ({
  api: {
    getAppointmentGroup: vi.fn(),
    createAppointmentGroup: vi.fn(),
    updateAppointmentGroup: vi.fn(),
    listAppointmentSlots: vi.fn(),
    getGroupCategories: vi.fn(),
  },
}));

vi.mock('../../services/api_calendar', () => ({
  calendarApi: {
    listContexts: vi.fn(),
    deleteSlot: vi.fn(),
  },
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderAt(path) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/courses/:courseId/appointment_groups/new" element={<AppointmentGroupEditorPage />} />
        <Route path="/courses/:courseId/appointment_groups/:groupId/edit" element={<AppointmentGroupEditorPage />} />
        <Route path="/courses/:courseId/appointment_groups" element={<div>list</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

const contexts = [
  { asset_string: 'user_1', type: 'user', id: 1, name: 'Jane Doe' },
  { asset_string: 'course_5', type: 'course', id: 5, name: 'Astronomy', is_instructor: true, sections: [{ id: 51, name: 'Section A' }, { id: 52, name: 'Section B' }] },
  { asset_string: 'course_6', type: 'course', id: 6, name: 'Geology', is_instructor: true, sections: [] },
];

describe('AppointmentGroupEditorPage (G097)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    calendarApi.listContexts.mockResolvedValue({ data: contexts });
    api.getGroupCategories.mockResolvedValue({ data: [{ id: 70, name: 'Lab teams' }] });
  });

  test('creates a group for two courses limited to one section', async () => {
    api.createAppointmentGroup.mockResolvedValue({ data: {} });
    renderAt('/courses/5/appointment_groups/new');
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Advising' } });
    fireEvent.click(await screen.findByLabelText('Geology'));
    fireEvent.click(await screen.findByLabelText('Section A'));
    fireEvent.click(screen.getByRole('button', { name: 'Create Group' }));
    await waitFor(() => expect(api.createAppointmentGroup).toHaveBeenCalled());
    const [courseId, payload] = api.createAppointmentGroup.mock.calls[0];
    expect(courseId).toBe('5');
    expect(payload.appointment_group.context_codes).toEqual(['course_5', 'course_6']);
    expect(payload.appointment_group.sub_context_codes).toEqual(['course_section_51']);
    expect(payload.appointment_group.participant_type).toBeUndefined();
  });

  test('creates a group sign-up for a group set', async () => {
    api.createAppointmentGroup.mockResolvedValue({ data: {} });
    renderAt('/courses/5/appointment_groups/new');
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Project check-in' } });
    await screen.findByRole('option', { name: 'Lab teams' });
    fireEvent.change(screen.getByLabelText('Sign up as'), { target: { value: '70' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Group' }));
    await waitFor(() => expect(api.createAppointmentGroup).toHaveBeenCalled());
    const payload = api.createAppointmentGroup.mock.calls[0][1].appointment_group;
    expect(payload.participant_type).toBe('Group');
    expect(payload.sub_context_codes).toEqual(['group_category_70']);
  });

  test('editing shows current slots, deletes one and adds new ones', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    api.getAppointmentGroup.mockResolvedValue({
      data: {
        id: 3, course_id: 5, title: 'Office hours', participants_per_appointment: 1, max_appointments_per_participant: 1,
        workflow_state: 'active', context_codes: ['course_5'], sub_context_codes: ['course_section_52'],
      },
    });
    const start = new Date(Date.now() + 86400000).toISOString();
    api.listAppointmentSlots.mockResolvedValue({ data: [{ id: 30, start_at: start, end_at: start, reservation_count: 0, effective_limit: 1 }] });
    calendarApi.deleteSlot.mockResolvedValue({ data: null });
    api.updateAppointmentGroup.mockResolvedValue({ data: {} });

    renderAt('/courses/5/appointment_groups/3/edit');
    expect(await screen.findByText('Current slots')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: 'Delete slot' }));
    await waitFor(() => expect(calendarApi.deleteSlot).toHaveBeenCalledWith('3', 30));

    // Generate one 15-minute slot and save.
    fireEvent.change(screen.getByLabelText('Date'), { target: { value: '2030-01-07' } });
    fireEvent.change(screen.getByLabelText('Start'), { target: { value: '09:00' } });
    fireEvent.change(screen.getByLabelText('End'), { target: { value: '09:15' } });
    fireEvent.click(screen.getByRole('button', { name: /Add to group/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));
    await waitFor(() => expect(api.updateAppointmentGroup).toHaveBeenCalled());
    const payload = api.updateAppointmentGroup.mock.calls[0][1].appointment_group;
    expect(payload.new_appointments).toHaveLength(1);
    expect(payload.sub_context_codes).toEqual(['course_section_52']);
  });
});
