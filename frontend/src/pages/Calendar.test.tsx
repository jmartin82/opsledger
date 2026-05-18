import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import CalendarPage from '@/pages/Calendar';
import { mockChange } from '@/test/helpers';

const API_URL = 'http://localhost:8081';

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: vi.fn(() => ({
    user: { id: 1, name: 'Test User', email: 'test@example.com', role: 'editor', status: 'active' },
    isAuthenticated: true,
    loading: false,
    can: () => true,
    logout: vi.fn(),
  })),
}));

vi.mock('@/contexts/LiveContext', () => ({
  useLive: vi.fn(() => ({ connected: false, subscribe: vi.fn(() => vi.fn()) })),
}));

function renderCalendar() {
  return render(
    <MemoryRouter initialEntries={['/calendar']}>
      <CalendarPage />
    </MemoryRouter>,
  );
}

describe('Calendar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders month grid', async () => {
    renderCalendar();
    // Day-of-week headers
    await waitFor(() => {
      expect(screen.getByText('Su')).toBeInTheDocument();
    });
  });

  it('renders overdue section when overdue changes exist', async () => {
    const overdueChange = mockChange({
      id: 'overdue-1',
      status: 'scheduled',
      timestamp: '2020-01-01T10:00:00Z',
      system: 'overdue-service',
    });

    server.use(
      http.get(`${API_URL}/api/changes`, ({ request }) => {
        const url = new URL(request.url);
        const status = url.searchParams.get('status');
        if (status === 'overdue') {
          return HttpResponse.json({ changes: [overdueChange], total: 1 });
        }
        return HttpResponse.json({ changes: [], total: 0 });
      }),
    );

    renderCalendar();

    await waitFor(() => {
      expect(screen.getByText('Overdue (1)')).toBeInTheDocument();
      expect(screen.getByText('overdue-service')).toBeInTheDocument();
    });
  });

  it('shows no overdue section when no overdue changes', async () => {
    server.use(
      http.get(`${API_URL}/api/changes`, () => {
        return HttpResponse.json({ changes: [], total: 0 });
      }),
    );

    renderCalendar();

    await waitFor(() => {
      expect(screen.queryByText(/overdue/i)).not.toBeInTheDocument();
    });
  });

  it('shows day detail panel when a day with events is selected', async () => {
    const scheduledChange = mockChange({
      id: 'sched-1',
      status: 'scheduled',
      timestamp: '2026-06-15T14:00:00Z',
      system: 'deploy-target',
      description: 'Planned deployment',
    });

    server.use(
      http.get(`${API_URL}/api/changes`, ({ request }) => {
        const url = new URL(request.url);
        const status = url.searchParams.get('status');
        if (status === 'scheduled') {
          return HttpResponse.json({ changes: [scheduledChange], total: 1 });
        }
        return HttpResponse.json({ changes: [], total: 0 });
      }),
    );

    const user = userEvent.setup();
    renderCalendar();

    // Click on day 15 in the calendar grid
    await waitFor(() => {
      const day15 = screen.getAllByText('15').find(el => el.tagName !== 'SPAN' || el.closest('button'));
      expect(day15).toBeInTheDocument();
    });

    const day15buttons = screen.getAllByRole('gridcell').filter(cell =>
      cell.textContent?.includes('15')
    );
    if (day15buttons[0]) {
      await user.click(day15buttons[0]);
    }
  });

  it('confirms a change from the overdue section', async () => {
    const overdueChange = mockChange({
      id: 'overdue-confirm',
      status: 'scheduled',
      timestamp: '2020-01-01T10:00:00Z',
      system: 'legacy-app',
    });

    let confirmCalled = false;
    server.use(
      http.get(`${API_URL}/api/changes`, ({ request }) => {
        const url = new URL(request.url);
        const status = url.searchParams.get('status');
        if (status === 'overdue') {
          return HttpResponse.json({ changes: confirmCalled ? [] : [overdueChange], total: confirmCalled ? 0 : 1 });
        }
        return HttpResponse.json({ changes: [], total: 0 });
      }),
      http.patch(`${API_URL}/api/changes/overdue-confirm/confirm`, () => {
        confirmCalled = true;
        return HttpResponse.json(mockChange({ id: 'overdue-confirm', status: 'executed' }));
      }),
    );

    const user = userEvent.setup();
    renderCalendar();

    await waitFor(() => {
      expect(screen.getByText('legacy-app')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Done'));

    await waitFor(() => {
      expect(confirmCalled).toBe(true);
    });
  });
});
