import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import CalendarFeedPanel from '../CalendarFeedPanel';
import { calendarApi } from '../../../services/api_calendar';

vi.mock('../../../services/api_calendar', () => ({
  calendarApi: {
    getFeed: vi.fn(),
    rotateFeed: vi.fn(),
    revokeFeed: vi.fn(),
  },
}));

const live = {
  exists: true,
  url_available: true,
  feed_url: 'https://lms.example.test/api/v1/feeds/calendars/tok123.ics',
  webcal_url: 'webcal://lms.example.test/api/v1/feeds/calendars/tok123.ics',
};

describe('CalendarFeedPanel (G096)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  test('creates a feed link when there is none', async () => {
    calendarApi.getFeed.mockResolvedValue({ data: { exists: false, url_available: false } });
    calendarApi.rotateFeed.mockResolvedValue({ data: live });
    render(<CalendarFeedPanel onClose={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Create feed link' }));
    expect(await screen.findByDisplayValue(live.feed_url)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open in calendar app' })).toHaveAttribute('href', live.webcal_url);
  });

  test('resets and turns off an existing feed', async () => {
    calendarApi.getFeed.mockResolvedValue({ data: live });
    calendarApi.rotateFeed.mockResolvedValue({ data: { ...live, feed_url: 'https://lms.example.test/api/v1/feeds/calendars/new.ics' } });
    calendarApi.revokeFeed.mockResolvedValue({ data: { exists: false } });
    render(<CalendarFeedPanel onClose={() => {}} />);
    expect(await screen.findByDisplayValue(live.feed_url)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Reset link/ }));
    expect(await screen.findByDisplayValue(/new\.ics/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Turn off feed' }));
    expect(await screen.findByRole('button', { name: 'Create feed link' })).toBeInTheDocument();
    await waitFor(() => expect(calendarApi.revokeFeed).toHaveBeenCalled());
  });
});
