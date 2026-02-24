import { useState } from 'react';
import { useAuth, UserRole, AuthUser } from '@/contexts/AuthContext';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { formatDistanceToNow } from 'date-fns';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog';
import { UserPlus, MoreHorizontal, RefreshCw, Ban, CheckCircle, Shield } from 'lucide-react';
import { cn } from '@/lib/utils';
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';

const ROLE_COLORS: Record<UserRole, string> = {
  admin: 'text-primary border-primary/30 bg-primary/10',
  editor: 'text-infra border-infra-border bg-infra-bg',
  viewer: 'text-deploy border-deploy-border bg-deploy-bg',
};

const RoleBadge = ({ role, ssoManaged }: { role: UserRole; ssoManaged?: boolean }) => (
  <span className={cn('inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full border', ROLE_COLORS[role])}>
    {role}
    {ssoManaged && <span className="opacity-60">· SSO</span>}
  </span>
);

const StatusBadge = ({ status }: { status: 'active' | 'disabled' }) => (
  <span className={cn(
    'inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full border',
    status === 'active' ? 'text-deploy border-deploy-border bg-deploy-bg' : 'text-muted-foreground border-border bg-muted/30'
  )}>
    <span className={cn('w-1.5 h-1.5 rounded-full', status === 'active' ? 'bg-deploy' : 'bg-muted-foreground')} />
    {status}
  </span>
);

const CreateUserDialog = ({ open, onClose }: { open: boolean; onClose: () => void }) => {
  const { createUser, ssoConfig } = useAuth();
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [role, setRole] = useState<UserRole>('viewer');
  const [tempPassword, setTempPassword] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleCreate = async () => {
    if (!email || !name) return;
    setError(null);
    setSubmitting(true);
    try {
      const result = await createUser(email, name, role);
      setTempPassword(result.temporaryPassword);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create user');
    } finally {
      setSubmitting(false);
    }
  };

  const handleClose = () => {
    setEmail(''); setName(''); setRole('viewer');
    setTempPassword(null); setError(null);
    onClose();
  };

  if (ssoConfig.enabled) return null;

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="bg-card border-border max-w-sm">
        <DialogHeader>
          <DialogTitle className="text-base">{tempPassword ? 'User Created' : 'Create Local User'}</DialogTitle>
        </DialogHeader>
        {tempPassword ? (
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              Share this temporary password with the user. It will not be shown again.
            </p>
            <div className="flex items-center gap-2">
              <Input value={tempPassword} readOnly className="bg-background border-border text-sm font-mono" />
              <Button size="sm" variant="outline" onClick={() => navigator.clipboard.writeText(tempPassword)}>Copy</Button>
            </div>
            <DialogFooter>
              <Button size="sm" onClick={handleClose}>Done</Button>
            </DialogFooter>
          </div>
        ) : (
          <>
            <div className="space-y-3 py-2">
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Full name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} placeholder="Alice Martin" className="bg-background border-border text-sm" />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Email</Label>
                <Input value={email} onChange={e => setEmail(e.target.value)} placeholder="alice@company.com" className="bg-background border-border text-sm" />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Role</Label>
                <Select value={role} onValueChange={v => setRole(v as UserRole)}>
                  <SelectTrigger className="bg-background border-border text-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="admin">Admin</SelectItem>
                    <SelectItem value="editor">Editor</SelectItem>
                    <SelectItem value="viewer">Viewer</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {error && (
                <p className="text-xs text-destructive p-2 rounded bg-destructive/10 border border-destructive/20">{error}</p>
              )}
              <p className="text-xs text-muted-foreground p-2 rounded bg-muted/30 border border-border">
                A temporary password will be generated. The user will be prompted to reset it on first login.
              </p>
            </div>
            <DialogFooter>
              <Button variant="outline" size="sm" onClick={handleClose}>Cancel</Button>
              <Button size="sm" onClick={handleCreate} disabled={!email || !name || submitting}>
                {submitting ? 'Creating...' : 'Create User'}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
};

const UserRow = ({ u, currentUserId }: { u: AuthUser; currentUserId: number }) => {
  const { updateUserRole, toggleUserStatus, resetPassword, ssoConfig } = useAuth();
  const isCurrentUser = u.id === currentUserId;
  const [tempPassword, setTempPassword] = useState<string | null>(null);

  const handleResetPassword = async () => {
    try {
      const result = await resetPassword(String(u.id));
      setTempPassword(result.temporaryPassword);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to reset password');
    }
  };

  const handleRoleChange = async (role: UserRole) => {
    try {
      await updateUserRole(String(u.id), role);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to update role');
    }
  };

  const handleToggleStatus = async () => {
    try {
      await toggleUserStatus(String(u.id));
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to update status');
    }
  };

  return (
    <>
      <div className="flex items-center gap-4 px-4 py-3 border-b border-border last:border-0 hover:bg-accent/20 transition-colors">
        {/* Avatar */}
        <div className="w-8 h-8 rounded-full bg-secondary flex items-center justify-center shrink-0 text-xs font-semibold text-foreground">
          {u.name.split(' ').map(n => n[0]).join('').slice(0, 2).toUpperCase()}
        </div>

        {/* Identity */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-foreground truncate">{u.name}</span>
            {isCurrentUser && <span className="text-xs text-muted-foreground">(you)</span>}
          </div>
          <p className="text-xs text-muted-foreground truncate">{u.email}</p>
        </div>

        {/* Role */}
        <RoleBadge role={u.role} ssoManaged={u.ssoManaged} />

        {/* Status */}
        <StatusBadge status={u.status} />

        {/* Last login */}
        <span className="text-xs text-muted-foreground w-28 shrink-0 text-right">
          {u.lastLogin ? formatDistanceToNow(new Date(u.lastLogin), { addSuffix: true }) : 'Never'}
        </span>

        {/* Actions */}
        {isCurrentUser ? (
          <div className="w-7 shrink-0" />
        ) : (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7">
                <MoreHorizontal className="w-4 h-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="bg-card border-border w-48">
              <div className="px-2 py-1.5">
                <p className="text-xs text-muted-foreground mb-1">Change role</p>
                {(['admin', 'editor', 'viewer'] as UserRole[]).filter(r => r !== u.role).map(r => (
                  <DropdownMenuItem key={r} onClick={() => handleRoleChange(r)} className="text-sm gap-2">
                    <Shield className="w-3.5 h-3.5" />
                    Set as {r}
                  </DropdownMenuItem>
                ))}
              </div>
              <DropdownMenuSeparator />
              {!ssoConfig.enabled && (
                <DropdownMenuItem onClick={handleResetPassword} className="text-sm gap-2">
                  <RefreshCw className="w-3.5 h-3.5" />
                  Force password reset
                </DropdownMenuItem>
              )}
              <DropdownMenuItem
                onClick={handleToggleStatus}
                className={cn('text-sm gap-2', u.status === 'active' ? 'text-destructive focus:text-destructive' : '')}
              >
                {u.status === 'active' ? <Ban className="w-3.5 h-3.5" /> : <CheckCircle className="w-3.5 h-3.5" />}
                {u.status === 'active' ? 'Disable user' : 'Enable user'}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {/* Temp password dialog after reset */}
      <Dialog open={!!tempPassword} onOpenChange={() => setTempPassword(null)}>
        <DialogContent className="bg-card border-border max-w-sm">
          <DialogHeader>
            <DialogTitle className="text-base">Password Reset</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              New temporary password for <strong>{u.name}</strong>. It will not be shown again.
            </p>
            <div className="flex items-center gap-2">
              <Input value={tempPassword ?? ''} readOnly className="bg-background border-border text-sm font-mono" />
              <Button size="sm" variant="outline" onClick={() => navigator.clipboard.writeText(tempPassword ?? '')}>Copy</Button>
            </div>
          </div>
          <DialogFooter>
            <Button size="sm" onClick={() => setTempPassword(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
};

const UsersTab = () => {
  const { users, user: currentUser, ssoConfig } = useAuth();
  const [showCreate, setShowCreate] = useState(false);

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-muted-foreground">
            {users.length} user{users.length !== 1 ? 's' : ''}
            {ssoConfig.enabled && <span className="ml-2 text-xs text-config">· SSO mode — new users are provisioned on first login</span>}
          </p>
        </div>
        {!ssoConfig.enabled && (
          <Button size="sm" onClick={() => setShowCreate(true)} className="gap-1.5">
            <UserPlus className="w-3.5 h-3.5" />
            Create User
          </Button>
        )}
      </div>

      {/* Table */}
      <div className="border border-border rounded-lg overflow-hidden">
        {/* Header row */}
        <div className="flex items-center gap-4 px-4 py-2 bg-secondary/20 border-b border-border">
          <div className="w-8 shrink-0" />
          <p className="flex-1 text-xs font-medium text-muted-foreground uppercase tracking-wide">User</p>
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide w-20">Role</p>
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide w-20">Status</p>
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide w-28 text-right">Last login</p>
          <div className="w-7 shrink-0" />
        </div>
        {users.map(u => (
          <UserRow key={u.id} u={u} currentUserId={currentUser!.id} />
        ))}
      </div>

      <CreateUserDialog open={showCreate} onClose={() => setShowCreate(false)} />
    </div>
  );
};

export default UsersTab;
