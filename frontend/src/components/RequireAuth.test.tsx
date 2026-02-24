import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import RequireAuth from '@/components/RequireAuth';
import { mockUser } from '@/test/helpers';
import type { AuthUser } from '@/contexts/AuthContext';

// Mock the auth context module
vi.mock('@/contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}));

import { useAuth } from '@/contexts/AuthContext';
const mockUseAuth = vi.mocked(useAuth);

function renderWithAuth(
  minRole: 'viewer' | 'editor' | 'admin' | undefined,
  user: AuthUser | null,
  loading = false,
) {
  const authValue = {
    user,
    isAuthenticated: !!user,
    loading,
    can: () => true,
    login: vi.fn(),
    logout: vi.fn(),
    register: vi.fn(),
  };
  mockUseAuth.mockReturnValue(authValue as any);

  return render(
    <MemoryRouter initialEntries={['/protected']}>
      <Routes>
        <Route
          path="/protected"
          element={
            <RequireAuth minRole={minRole}>
              <div>Protected Content</div>
            </RequireAuth>
          }
        />
        <Route path="/login" element={<div>Login Page</div>} />
        <Route path="/" element={<div>Home Page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('RequireAuth', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('redirects to /login when unauthenticated', () => {
    renderWithAuth(undefined, null);
    expect(screen.getByText('Login Page')).toBeInTheDocument();
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });

  it('renders children when authenticated with any role', () => {
    renderWithAuth(undefined, mockUser({ role: 'viewer' }));
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('minRole=editor blocks viewer and redirects to /', () => {
    renderWithAuth('editor', mockUser({ role: 'viewer' }));
    expect(screen.getByText('Home Page')).toBeInTheDocument();
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });

  it('minRole=editor allows editor', () => {
    renderWithAuth('editor', mockUser({ role: 'editor' }));
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('minRole=admin blocks editor and redirects to /', () => {
    renderWithAuth('admin', mockUser({ role: 'editor' }));
    expect(screen.getByText('Home Page')).toBeInTheDocument();
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });

  it('renders nothing while loading', () => {
    renderWithAuth(undefined, null, true);
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.queryByText('Login Page')).not.toBeInTheDocument();
  });
});
