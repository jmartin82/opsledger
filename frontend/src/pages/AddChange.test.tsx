import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import AddChange from '@/pages/AddChange';
import { mockUser } from '@/test/helpers';

const API_URL = 'http://localhost:8081';
const mockNavigate = vi.fn();

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: vi.fn(() => ({
    user: mockUser(),
    isAuthenticated: true,
    loading: false,
    can: (action: string) => action !== 'manage_auth' && action !== 'view_admin',
    logout: vi.fn(),
  })),
}));

vi.mock('@/contexts/LiveContext', () => ({
  useLive: vi.fn(() => ({ connected: false, subscribe: vi.fn(() => vi.fn()) })),
}));

function renderAddChange() {
  return render(
    <MemoryRouter initialEntries={['/add']}>
      <AddChange />
    </MemoryRouter>,
  );
}

describe('AddChange', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders all form fields', () => {
    renderAddChange();

    // Change type buttons
    expect(screen.getByText('Infrastructure')).toBeInTheDocument();
    expect(screen.getByText('Deployment')).toBeInTheDocument();
    expect(screen.getByText('Configuration')).toBeInTheDocument();

    // System field
    expect(screen.getByPlaceholderText('e.g. api-gateway')).toBeInTheDocument();

    // Environment field
    expect(screen.getByPlaceholderText('e.g. production')).toBeInTheDocument();

    // User field
    expect(screen.getByPlaceholderText('e.g. alice.martin')).toBeInTheDocument();

    // Description field
    expect(screen.getByPlaceholderText(/describe what changed/i)).toBeInTheDocument();

    // Submit button
    expect(screen.getByRole('button', { name: /register change/i })).toBeInTheDocument();
  });

  it('shows validation errors for empty required fields', async () => {
    const user = userEvent.setup();
    renderAddChange();

    await user.click(screen.getByRole('button', { name: /register change/i }));

    expect(screen.getByText('System is required')).toBeInTheDocument();
    expect(screen.getByText('Change type is required')).toBeInTheDocument();
    expect(screen.getByText('Description is required')).toBeInTheDocument();
  });

  it('selecting change type clears type error', async () => {
    const user = userEvent.setup();
    renderAddChange();

    // Trigger validation
    await user.click(screen.getByRole('button', { name: /register change/i }));
    expect(screen.getByText('Change type is required')).toBeInTheDocument();

    // Select a type
    await user.click(screen.getByText('Deployment'));

    expect(screen.queryByText('Change type is required')).not.toBeInTheDocument();
  });

  it('successful submit shows Change Registered screen', async () => {
    const user = userEvent.setup();
    renderAddChange();

    // Fill form
    await user.click(screen.getByText('Deployment'));
    await user.type(screen.getByPlaceholderText('e.g. api-gateway'), 'frontend');
    await user.type(screen.getByPlaceholderText(/describe what changed/i), 'Deployed v3.0');

    await user.click(screen.getByRole('button', { name: /register change/i }));

    await waitFor(() => {
      expect(screen.getByText('Change Registered')).toBeInTheDocument();
    });
  });

  it('submit sends correct payload', async () => {
    let capturedBody: unknown = null;
    server.use(
      http.post(`${API_URL}/api/changes`, async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({ id: '99', ...capturedBody }, { status: 201 });
      }),
    );

    const user = userEvent.setup();
    renderAddChange();

    await user.click(screen.getByText('Infrastructure'));
    await user.type(screen.getByPlaceholderText('e.g. api-gateway'), 'database-primary');
    await user.type(screen.getByPlaceholderText(/describe what changed/i), 'Added read replica');

    await user.click(screen.getByRole('button', { name: /register change/i }));

    await waitFor(() => {
      expect(capturedBody).toBeTruthy();
      expect(capturedBody.system).toBe('database-primary');
      expect(capturedBody.type).toBe('infrastructure');
      expect(capturedBody.description).toBe('Added read replica');
      // Timestamp should be ISO string
      expect(capturedBody.timestamp).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    });
  });

  it('failed submit shows error', async () => {
    server.use(
      http.post(`${API_URL}/api/changes`, () => {
        return HttpResponse.json({ error: 'Server error' }, { status: 500 });
      }),
    );

    const user = userEvent.setup();
    renderAddChange();

    await user.click(screen.getByText('Deployment'));
    await user.type(screen.getByPlaceholderText('e.g. api-gateway'), 'frontend');
    await user.type(screen.getByPlaceholderText(/describe what changed/i), 'Deploy v3');

    await user.click(screen.getByRole('button', { name: /register change/i }));

    await waitFor(() => {
      expect(screen.getByText('Server error')).toBeInTheDocument();
    });
  });

  it('"Register Another" resets the form', async () => {
    const user = userEvent.setup();
    renderAddChange();

    // Submit successfully
    await user.click(screen.getByText('Deployment'));
    await user.type(screen.getByPlaceholderText('e.g. api-gateway'), 'frontend');
    await user.type(screen.getByPlaceholderText(/describe what changed/i), 'Deploy v3');
    await user.click(screen.getByRole('button', { name: /register change/i }));

    await waitFor(() => {
      expect(screen.getByText('Change Registered')).toBeInTheDocument();
    });

    // Click Register Another
    await user.click(screen.getByText('Register Another'));

    // Form should be back, fields empty
    expect(screen.getByPlaceholderText('e.g. api-gateway')).toHaveValue('');
    expect(screen.getByPlaceholderText(/describe what changed/i)).toHaveValue('');
  });

  it('Cancel navigates to /', async () => {
    const user = userEvent.setup();
    renderAddChange();

    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(mockNavigate).toHaveBeenCalledWith('/');
  });
});
