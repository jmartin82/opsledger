import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import Login from '@/pages/Login';

const API_URL = 'http://localhost:8081';

// Mock auth context
const mockLogin = vi.fn();
const mockRegister = vi.fn();
const mockNavigate = vi.fn();

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: vi.fn(() => ({
    login: mockLogin,
    register: mockRegister,
    ssoConfig: { enabled: false },
  })),
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <Login />
    </MemoryRouter>,
  );
}

describe('Login', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLogin.mockResolvedValue({ success: true });
    mockRegister.mockResolvedValue({ success: true });
  });

  it('renders login form by default', () => {
    renderLogin();
    expect(screen.getByText('Sign in to your account')).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
    // Name field should NOT be visible in login mode
    expect(screen.queryByLabelText(/full name/i)).not.toBeInTheDocument();
  });

  it('shows error when email and password are empty', async () => {
    const user = userEvent.setup();
    renderLogin();

    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(screen.getByText('Email and password are required.')).toBeInTheDocument();
    expect(mockLogin).not.toHaveBeenCalled();
  });

  it('successful login navigates to /', async () => {
    const user = userEvent.setup();
    renderLogin();

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(mockLogin).toHaveBeenCalledWith('test@example.com', 'password123');
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/');
    });
  });

  it('failed login shows error message', async () => {
    mockLogin.mockResolvedValue({ success: false, error: 'Invalid credentials' });
    const user = userEvent.setup();
    renderLogin();

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/password/i), 'wrong');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument();
    });
  });

  it('toggle to register mode shows name field', async () => {
    // Ensure registration link is shown
    server.use(
      http.get(`${API_URL}/api/auth/registration-status`, () => {
        return HttpResponse.json({ allowed: true });
      }),
    );

    const user = userEvent.setup();
    renderLogin();

    // Wait for registration status to load and show the "Create one" link
    const createOneBtn = await screen.findByText('Create one');
    await user.click(createOneBtn);

    expect(screen.getByText('Create your account')).toBeInTheDocument();
    expect(screen.getByLabelText(/full name/i)).toBeInTheDocument();
  });

  it('register validates name is required', async () => {
    server.use(
      http.get(`${API_URL}/api/auth/registration-status`, () => {
        return HttpResponse.json({ allowed: true });
      }),
    );

    const user = userEvent.setup();
    renderLogin();

    // Switch to register mode
    const createOneBtn = await screen.findByText('Create one');
    await user.click(createOneBtn);

    await user.type(screen.getByLabelText(/email/i), 'new@example.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(screen.getByText('Full name is required.')).toBeInTheDocument();
    expect(mockRegister).not.toHaveBeenCalled();
  });

  it('successful registration navigates to /', async () => {
    server.use(
      http.get(`${API_URL}/api/auth/registration-status`, () => {
        return HttpResponse.json({ allowed: true });
      }),
    );

    const user = userEvent.setup();
    renderLogin();

    const createOneBtn = await screen.findByText('Create one');
    await user.click(createOneBtn);

    await user.type(screen.getByLabelText(/full name/i), 'New User');
    await user.type(screen.getByLabelText(/email/i), 'new@example.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(mockRegister).toHaveBeenCalledWith('new@example.com', 'password123', 'New User');
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/');
    });
  });

  it('registration link hidden when registration not allowed', async () => {
    server.use(
      http.get(`${API_URL}/api/auth/registration-status`, () => {
        return HttpResponse.json({ allowed: false });
      }),
    );

    renderLogin();

    // Wait for the registration status to resolve
    await waitFor(() => {
      expect(screen.queryByText('Create one')).not.toBeInTheDocument();
    });
  });

  it('shows loading state during submit', async () => {
    // Make login hang
    mockLogin.mockImplementation(() => new Promise(() => {}));
    const user = userEvent.setup();
    renderLogin();

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      const btn = screen.getByRole('button', { name: /please wait/i });
      expect(btn).toBeDisabled();
    });
  });
});
