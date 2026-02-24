import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { api, setToken, getToken, clearToken } from '@/lib/api';

export type UserRole = 'admin' | 'editor' | 'viewer';

export interface AuthUser {
  id: number;
  email: string;
  name: string;
  role: UserRole;
  status: 'active' | 'disabled';
  lastLogin: string | null;
  ssoManaged?: boolean;
}

export interface ApiKey {
  id: number;
  name: string;
  scopes: ('changes:read' | 'changes:write')[];
  createdAt: string;
  lastUsed?: string;
  expiresAt?: string;
  status: 'active' | 'revoked';
  prefix: string;
  createdBy: number;
}

export interface SsoConfig {
  enabled: boolean;
  provider: 'azure' | 'okta' | 'google' | 'custom';
  issuerUrl: string;
  clientId: string;
  clientSecret: string;
  scopes: string;
  roleMappingClaim: string;
  adminGroups: string;
  editorGroups: string;
  viewerGroups: string;
}

const SSO_KEY = 'opsledger_sso';

const DEFAULT_SSO: SsoConfig = {
  enabled: false,
  provider: 'azure',
  issuerUrl: '',
  clientId: '',
  clientSecret: '',
  scopes: 'openid profile email',
  roleMappingClaim: 'groups',
  adminGroups: 'opsledger-admins',
  editorGroups: 'opsledger-editors',
  viewerGroups: 'opsledger-viewers',
};

interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  users: AuthUser[];
  apiKeys: ApiKey[];
  ssoConfig: SsoConfig;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<{ success: boolean; error?: string }>;
  logout: () => void;
  register: (email: string, password: string, name: string) => Promise<{ success: boolean; error?: string }>;
  updateUserRole: (userId: string, role: UserRole) => Promise<void>;
  toggleUserStatus: (userId: string) => Promise<void>;
  createUser: (email: string, name: string, role: UserRole) => Promise<{ temporaryPassword: string }>;
  resetPassword: (userId: string) => Promise<{ temporaryPassword: string }>;
  createApiKey: (name: string, scopes: ApiKey['scopes'], expiresAt?: string) => Promise<{ key: string }>;
  revokeApiKey: (keyId: number) => Promise<void>;
  rotateApiKey: (keyId: number) => Promise<{ key: string }>;
  saveSsoConfig: (config: SsoConfig) => void;
  can: (action: 'manage_auth' | 'register_changes' | 'edit_changes' | 'view_changes' | 'view_admin') => boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

interface AuthApiResponse {
  token: string;
  user: AuthUser;
}

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);

  const [users, setUsers] = useState<AuthUser[]>([]);

  const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);

  const [ssoConfig, setSsoConfig] = useState<SsoConfig>(() => {
    const stored = localStorage.getItem(SSO_KEY);
    return stored ? JSON.parse(stored) : DEFAULT_SSO;
  });

  useEffect(() => { localStorage.setItem(SSO_KEY, JSON.stringify(ssoConfig)); }, [ssoConfig]);

  const fetchApiKeys = useCallback(async () => {
    try {
      const keys = await api.get<ApiKey[]>('/api/admin/api-keys');
      setApiKeys(keys);
    } catch {
      // Non-admin users will get 403 — silently ignore
    }
  }, []);

  const fetchUsers = useCallback(async () => {
    try {
      const data = await api.get<AuthUser[]>('/api/admin/users');
      setUsers(data);
    } catch {
      // Non-admin users will get 403 — silently ignore
    }
  }, []);

  // Restore session from JWT on mount
  useEffect(() => {
    const token = getToken();
    if (!token) {
      setLoading(false);
      return;
    }
    api.get<AuthUser>('/api/auth/me')
      .then((u) => {
        setUser(u);
        if (u.role === 'admin') {
          fetchApiKeys();
          fetchUsers();
        }
      })
      .catch(() => clearToken())
      .finally(() => setLoading(false));
  }, [fetchApiKeys, fetchUsers]);

  const login = async (email: string, password: string): Promise<{ success: boolean; error?: string }> => {
    try {
      const data = await api.post<AuthApiResponse>('/api/auth/login', { email, password });
      setToken(data.token);
      setUser(data.user);
      if (data.user.role === 'admin') { fetchApiKeys(); fetchUsers(); }
      return { success: true };
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Login failed' };
    }
  };

  const register = async (email: string, password: string, name: string): Promise<{ success: boolean; error?: string }> => {
    try {
      const data = await api.post<AuthApiResponse>('/api/auth/register', { email, password, name });
      setToken(data.token);
      setUser(data.user);
      if (data.user.role === 'admin') { fetchApiKeys(); fetchUsers(); }
      return { success: true };
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Registration failed' };
    }
  };

  const logout = () => {
    // Fire-and-forget
    api.post('/api/auth/logout').catch(() => {});
    clearToken();
    setUser(null);
  };

  const updateUserRole = async (userId: string, role: UserRole): Promise<void> => {
    await api.put(`/api/admin/users/${userId}/role`, { role });
    await fetchUsers();
  };

  const toggleUserStatus = async (userId: string): Promise<void> => {
    const target = users.find(u => String(u.id) === userId);
    if (!target) return;
    const newStatus = target.status === 'active' ? 'disabled' : 'active';
    await api.put(`/api/admin/users/${userId}/status`, { status: newStatus });
    await fetchUsers();
  };

  const createUser = async (email: string, name: string, role: UserRole): Promise<{ temporaryPassword: string }> => {
    const data = await api.post<{ user: AuthUser; temporaryPassword: string }>('/api/admin/users', { email, name, role });
    await fetchUsers();
    return { temporaryPassword: data.temporaryPassword };
  };

  const resetPassword = async (userId: string): Promise<{ temporaryPassword: string }> => {
    const data = await api.post<{ temporaryPassword: string }>(`/api/admin/users/${userId}/reset-password`);
    return { temporaryPassword: data.temporaryPassword };
  };

  const createApiKey = async (name: string, scopes: ApiKey['scopes'], expiresAt?: string): Promise<{ key: string }> => {
    const body: { name: string; scopes: string[]; expiresAt?: string } = { name, scopes };
    if (expiresAt) {
      body.expiresAt = new Date(expiresAt).toISOString();
    }
    const data = await api.post<{ key: string; apiKey: ApiKey }>('/api/admin/api-keys', body);
    await fetchApiKeys();
    return { key: data.key };
  };

  const revokeApiKey = async (keyId: number): Promise<void> => {
    await api.post(`/api/admin/api-keys/${keyId}/revoke`);
    await fetchApiKeys();
  };

  const rotateApiKey = async (keyId: number): Promise<{ key: string }> => {
    const data = await api.post<{ key: string; apiKey: ApiKey }>(`/api/admin/api-keys/${keyId}/rotate`);
    await fetchApiKeys();
    return { key: data.key };
  };

  const saveSsoConfig = (config: SsoConfig) => {
    setSsoConfig(config);
  };

  const can = (action: 'manage_auth' | 'register_changes' | 'edit_changes' | 'view_changes' | 'view_admin'): boolean => {
    if (!user) return false;
    switch (action) {
      case 'manage_auth': return user.role === 'admin';
      case 'view_admin': return user.role === 'admin';
      case 'register_changes': return user.role === 'admin' || user.role === 'editor';
      case 'edit_changes': return user.role === 'admin' || user.role === 'editor';
      case 'view_changes': return true;
    }
  };

  return (
    <AuthContext.Provider value={{
      user, loading, users, apiKeys, ssoConfig, isAuthenticated: !!user,
      login, logout, register,
      updateUserRole, toggleUserStatus, createUser, resetPassword,
      createApiKey, revokeApiKey, rotateApiKey,
      saveSsoConfig, can,
    }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
};
