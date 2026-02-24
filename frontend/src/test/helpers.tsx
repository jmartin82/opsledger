import React from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { AuthUser, UserRole, ApiKey, SsoConfig } from '@/contexts/AuthContext';

// Re-export everything from RTL for convenience
export { screen, waitFor, within, act } from '@testing-library/react';
export { default as userEvent } from '@testing-library/user-event';

// ── Factories ──────────────────────────────────

export function mockUser(overrides?: Partial<AuthUser>): AuthUser {
  return {
    id: 1,
    email: 'test@example.com',
    name: 'Test User',
    role: 'editor',
    status: 'active',
    lastLogin: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

export function mockChange(overrides?: Partial<import('@/types/change').Change>): import('@/types/change').Change {
  return {
    id: '1',
    system: 'api-gateway',
    environment: 'production',
    user: 'alice.martin',
    type: 'deployment',
    description: 'Deployed v2.3.1 with bug fixes',
    timestamp: '2026-02-20T14:30:00Z',
    ...overrides,
  };
}

// ── Auth context mock ──────────────────────────

const ROLE_LEVEL: Record<UserRole, number> = { viewer: 1, editor: 2, admin: 3 };

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const AuthContext = React.createContext<any>(null);

// We dynamically import to get the actual context reference
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let _realAuthContext: React.Context<any> | null = null;

async function getRealAuthContext() {
  if (!_realAuthContext) {
    const mod = await import('@/contexts/AuthContext');
    // The context is not exported, but useAuth reads from it.
    // We'll use a provider wrapper approach instead.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    _realAuthContext = (mod as Record<string, unknown>).__AuthContext as React.Context<any>;
  }
  return _realAuthContext;
}

interface TestAuthProviderProps {
  user: AuthUser | null;
  children: React.ReactNode;
}

// Build a `can` function matching the real logic
function buildCan(user: AuthUser | null) {
  return (action: string) => {
    if (!user) return false;
    switch (action) {
      case 'manage_auth': return user.role === 'admin';
      case 'view_admin': return user.role === 'admin';
      case 'register_changes': return user.role === 'admin' || user.role === 'editor';
      case 'edit_changes': return user.role === 'admin' || user.role === 'editor';
      case 'view_changes': return true;
      default: return false;
    }
  };
}

// We need to provide the value on the same context that useAuth() reads from.
// Since AuthContext is not exported, we'll mock the module.
// Instead, we create a wrapper that provides mock values via the real AuthProvider's context.

// Approach: vi.mock the useAuth hook at the module level won't work per-test.
// Better approach: directly mock the context module using a test-specific provider.

// The simplest approach: We re-export a provider that provides values through
// the same context key. Since the context is created in AuthContext.tsx,
// we'll use vi.mock to replace the AuthProvider with our test version.

export function createAuthValue(user: AuthUser | null) {
  const noopAsync = async () => ({ success: true });
  const noopPromise = async () => ({ temporaryPassword: 'temp123' });
  const defaultSso: SsoConfig = {
    enabled: false, provider: 'azure', issuerUrl: '', clientId: '',
    clientSecret: '', scopes: '', roleMappingClaim: '', adminGroups: '',
    editorGroups: '', viewerGroups: '',
  };

  return {
    user,
    loading: false,
    users: [],
    apiKeys: [],
    ssoConfig: defaultSso,
    isAuthenticated: !!user,
    login: vi.fn().mockResolvedValue({ success: true }),
    logout: vi.fn(),
    register: vi.fn().mockResolvedValue({ success: true }),
    updateUserRole: vi.fn(),
    toggleUserStatus: vi.fn(),
    createUser: vi.fn().mockResolvedValue({ temporaryPassword: 'temp' }),
    resetPassword: vi.fn().mockResolvedValue({ temporaryPassword: 'temp' }),
    createApiKey: vi.fn().mockResolvedValue({ key: 'ol_live_test' }),
    revokeApiKey: vi.fn(),
    rotateApiKey: vi.fn().mockResolvedValue({ key: 'ol_live_rotated' }),
    saveSsoConfig: vi.fn(),
    can: buildCan(user),
  };
}

// ── Render wrapper ─────────────────────────────

interface WrapperOptions {
  user?: AuthUser | null;
  initialRoute?: string;
  authOverrides?: Record<string, unknown>;
}

/**
 * Renders a component wrapped in MemoryRouter and a mocked AuthContext provider.
 *
 * IMPORTANT: Tests using this must call `vi.mock('@/contexts/AuthContext')` at the
 * top of the file to intercept useAuth. Use `mockUseAuth()` to set up the mock.
 */
export function renderWithProviders(
  ui: React.ReactElement,
  { user, initialRoute = '/', authOverrides }: WrapperOptions = {},
  renderOptions?: Omit<RenderOptions, 'wrapper'>,
) {
  const authUser = user === undefined ? mockUser() : user;
  const authValue = { ...createAuthValue(authUser), ...authOverrides };

  // We need to provide this through the real context. We'll use the mock approach.
  // Set the mock return value before rendering.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { useAuth } = require('@/contexts/AuthContext') as { useAuth: { _isMockFunction?: boolean; mockReturnValue: (v: unknown) => void } };
  if (typeof useAuth === 'function' && useAuth._isMockFunction) {
    useAuth.mockReturnValue(authValue);
  }

  function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <MemoryRouter initialEntries={[initialRoute]}>
        {children}
      </MemoryRouter>
    );
  }

  return {
    ...render(ui, { wrapper: Wrapper, ...renderOptions }),
    authValue,
  };
}

/**
 * Call this at the top of test files that need auth mocking.
 * Use inside vi.mock callback or call before tests.
 */
export function mockUseAuth(user?: AuthUser | null) {
  const authUser = user === undefined ? mockUser() : user;
  const authValue = createAuthValue(authUser);
  return authValue;
}
