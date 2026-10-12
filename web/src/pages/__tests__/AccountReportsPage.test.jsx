import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import AccountReportsPage from '../AccountReportsPage';

// G117 — account reports page. All data is made up.

const startMutate = vi.fn();

vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../services/api', () => ({
  api: { getEnrollmentTerms: vi.fn(async () => ({ data: { enrollment_terms: [{ id: 4, name: 'Fall 2026' }] } })) },
}));
vi.mock('../../services/adminAccountsQueries', () => ({
  useAccountReports: () => ({
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    data: [
      { report: 'grade_export_csv', title: 'Grade Export', description: 'Scores', parameters: ['enrollment_term_id', 'include_deleted'], last_run: { id: 9, status: 'complete', downloadable: true, file_name: 'grade_export.csv', created_at: '2026-01-01T00:00:00Z' } },
      { report: 'recently_deleted_courses_csv', title: 'Recently Deleted Courses', description: 'Deleted', parameters: [] },
    ],
  }),
  useStartReport: () => ({ mutate: startMutate, isPending: false }),
  useDeleteReportRun: () => ({ mutate: vi.fn(), isPending: false }),
}));

describe('AccountReportsPage', () => {
  beforeEach(() => startMutate.mockReset());

  it('links the finished run for download', () => {
    render(<AccountReportsPage />);
    expect(screen.getByRole('link', { name: /grade_export\.csv/ }).getAttribute('href')).toContain('/reports/grade_export_csv/9/download');
  });

  it('runs a parameterless report straight away and asks for options otherwise', async () => {
    render(<AccountReportsPage />);
    const runButtons = screen.getAllByRole('button', { name: /^Run$/ });
    fireEvent.click(runButtons[1]);
    expect(startMutate).toHaveBeenCalledWith({ report: 'recently_deleted_courses_csv', parameters: {} }, expect.anything());

    fireEvent.click(runButtons[0]);
    await screen.findByRole('option', { name: 'Fall 2026' });
    fireEvent.change(screen.getByLabelText('Term'), { target: { value: '4' } });
    fireEvent.click(screen.getByLabelText('Include deleted objects'));
    fireEvent.click(screen.getByRole('button', { name: 'Run report' }));
    expect(startMutate).toHaveBeenLastCalledWith(
      { report: 'grade_export_csv', parameters: { enrollment_term_id: 4, include_deleted: true } },
      expect.anything(),
    );
  });
});
