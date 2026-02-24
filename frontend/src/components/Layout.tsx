import { NavLink, Link, useLocation, useNavigate } from 'react-router-dom';
import { Activity, Plus, BookOpen, Zap, Shield, LogOut, ChevronDown, User } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAuth } from '@/contexts/AuthContext';
import { useLive } from '@/contexts/LiveContext';
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';

const Layout = ({ children }: { children: React.ReactNode }) => {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, logout, can } = useAuth();
  const { connected } = useLive();

  const navItems = [
    { to: '/', label: 'Change Log', icon: Activity, exact: true, show: true },
    { to: '/add', label: 'Register Change', icon: Plus, show: can('register_changes') },
    { to: '/help', label: 'API & MCP', icon: BookOpen, show: true },
    { to: '/admin', label: 'Access & Security', icon: Shield, show: can('view_admin') },
  ].filter(i => i.show);

  const ROLE_COLORS = { admin: 'text-primary', editor: 'text-infra', viewer: 'text-muted-foreground' };
  const roleColor = user ? ROLE_COLORS[user.role] : '';

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-background flex flex-col">
      {/* Top nav */}
      <header className="border-b border-border bg-card sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between gap-6">
          {/* Brand */}
          <div className="flex items-center gap-2.5 shrink-0">
            <div className="w-7 h-7 rounded bg-primary flex items-center justify-center">
              <Zap className="w-4 h-4 text-primary-foreground" strokeWidth={2.5} />
            </div>
            <span className="font-semibold text-foreground tracking-tight text-sm">OpsLedger</span>
            <span className="text-muted-foreground text-xs ml-1 hidden sm:block">Change. Tracked.</span>
          </div>

          {/* Nav links */}
          <nav className="flex items-center gap-1 flex-1 justify-center">
            {navItems.map(({ to, label, icon: Icon, exact }) => {
              const isActive = exact ? location.pathname === to : location.pathname.startsWith(to);
              return (
                <NavLink
                  key={to}
                  to={to}
                  className={cn(
                    'flex items-center gap-1.5 px-3 py-1.5 rounded text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                  )}
                >
                  <Icon className="w-3.5 h-3.5" />
                  <span className="hidden sm:block">{label}</span>
                </NavLink>
              );
            })}
          </nav>

          {/* Right side: Live + User */}
          <div className="flex items-center gap-3 shrink-0">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <span className={cn(
                'w-1.5 h-1.5 rounded-full transition-colors',
                connected ? 'bg-deploy dot-live' : 'bg-amber-500/60'
              )} />
              <span className={cn('hidden sm:block', !connected && 'opacity-50')}>
                {connected ? 'Live' : 'Reconnecting'}
              </span>
            </div>

            {user && (
              <DropdownMenu>
                <DropdownMenuTrigger className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-accent transition-colors outline-none">
                  <div className="w-6 h-6 rounded-full bg-secondary flex items-center justify-center text-xs font-semibold text-foreground">
                    {user.name.split(' ').map(n => n[0]).join('').slice(0, 2).toUpperCase()}
                  </div>
                  <div className="hidden sm:block text-left">
                    <p className="text-xs font-medium text-foreground leading-none">{user.name.split(' ')[0]}</p>
                    <p className={cn('text-xs leading-none mt-0.5', roleColor)}>{user.role}</p>
                  </div>
                  <ChevronDown className="w-3 h-3 text-muted-foreground hidden sm:block" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="bg-card border-border w-48">
                  <div className="px-3 py-2">
                    <p className="text-sm font-medium text-foreground">{user.name}</p>
                    <p className="text-xs text-muted-foreground">{user.email}</p>
                    <p className={cn('text-xs font-medium mt-1', roleColor)}>{user.role}</p>
                  </div>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => navigate('/account')} className="text-sm gap-2">
                    <User className="w-3.5 h-3.5" />
                    My Account
                  </DropdownMenuItem>
                  {can('view_admin') && (
                    <DropdownMenuItem onClick={() => navigate('/admin')} className="text-sm gap-2">
                      <Shield className="w-3.5 h-3.5" />
                      Access & Security
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuItem onClick={handleLogout} className="text-sm gap-2 text-destructive focus:text-destructive">
                    <LogOut className="w-3.5 h-3.5" />
                    Sign out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="flex-1 max-w-7xl mx-auto w-full px-4 py-6">
        {children}
      </main>

      {/* Footer */}
      <footer className="border-t border-border py-3 px-4">
        <div className="max-w-7xl mx-auto flex items-center justify-between text-xs text-muted-foreground">
          <span>OpsLedger v1.0 — Change Management</span>
          <span className="font-mono">
            <Link to="/help?tab=rest" className="hover:text-foreground transition-colors">REST API</Link>
            {' · '}
            <Link to="/help?tab=mcp" className="hover:text-foreground transition-colors">MCP Connector</Link>
          </span>
        </div>
      </footer>
    </div>
  );
};

export default Layout;
