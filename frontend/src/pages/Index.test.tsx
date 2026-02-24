import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import Index from '@/pages/Index';
import { mockUser, mockChange } from '@/test/helpers';

const API_URL = 'http://localhost:8081';

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: vi.fn(() => ({
    user: mockUser(),
    isAuthenticated: true,
    loading: false,
    can: (action: string) => action !== 'manage_auth' && action !== 'view_admin',
    logout: vi.fn(),
  })),
}));

function renderIndex() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Index />
    </MemoryRouter>,
  );
}

describe('Index', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows loading state initially', () => {
    // Use a handler that never resolves
    server.use(
      http.get(`${API_URL}/api/changes`, () => {
        return new Promise(() => {});
      }),
    );

    renderIndex();
    expect(screen.getByText('Loading changes...')).toBeInTheDocument();
  });

  it('renders change list from API response', async () => {
    server.use(
      http.get(`${API_URL}/api/changes`, () => {
        return HttpResponse.json({
          changes: [
            mockChange({ id: '1', system: 'api-gateway', description: 'First change' }),
            mockChange({ id: '2', system: 'frontend', description: 'Second change' }),
          ],
          total: 2,
          limit: 50,
          offset: 0,
        });
      }),
    );

    renderIndex();

    await waitFor(() => {
      expect(screen.getByText('First change')).toBeInTheDocument();
      expect(screen.getByText('Second change')).toBeInTheDocument();
    });
  });

  it('error state shows retry button', async () => {
    server.use(
      http.get(`${API_URL}/api/changes`, () => {
        return HttpResponse.json({ error: 'Database error' }, { status: 500 });
      }),
    );

    renderIndex();

    await waitFor(() => {
      expect(screen.getByText('Database error')).toBeInTheDocument();
      expect(screen.getByText('Retry')).toBeInTheDocument();
    });
  });

  it('retry re-fetches changes', async () => {
    let callCount = 0;
    server.use(
      http.get(`${API_URL}/api/changes`, () => {
        callCount++;
        if (callCount === 1) {
          return HttpResponse.json({ error: 'Temporary error' }, { status: 500 });
        }
        return HttpResponse.json({
          changes: [mockChange()],
          total: 1,
          limit: 50,
          offset: 0,
        });
      }),
    );

    const user = userEvent.setup();
    renderIndex();

    // Wait for error state
    await waitFor(() => {
      expect(screen.getByText('Retry')).toBeInTheDocument();
    });

    // Click retry
    await user.click(screen.getByText('Retry'));

    // Wait for success
    await waitFor(() => {
      expect(screen.getByText('Deployed v2.3.1 with bug fixes')).toBeInTheDocument();
    });
  });

  it('empty state shows no changes message', async () => {
    server.use(
      http.get(`${API_URL}/api/changes`, () => {
        return HttpResponse.json({
          changes: [],
          total: 0,
          limit: 50,
          offset: 0,
        });
      }),
    );

    renderIndex();

    await waitFor(() => {
      expect(screen.getByText('No changes match the current filters.')).toBeInTheDocument();
    });
  });
});
