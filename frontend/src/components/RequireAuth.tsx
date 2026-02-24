import { useAuth } from '@/contexts/AuthContext';
import { Navigate, useLocation } from 'react-router-dom';
import type { UserRole } from '@/contexts/AuthContext';

interface RequireAuthProps {
  children: React.ReactNode;
  minRole?: UserRole;
}

const ROLE_LEVEL: Record<UserRole, number> = { viewer: 1, editor: 2, admin: 3 };

const RequireAuth = ({ children, minRole = 'viewer' }: RequireAuthProps) => {
  const { user, isAuthenticated, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return null;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (user && ROLE_LEVEL[user.role] < ROLE_LEVEL[minRole]) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
};

export default RequireAuth;
