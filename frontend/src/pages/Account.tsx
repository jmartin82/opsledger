import Layout from '@/components/Layout';
import { useAuth } from '@/contexts/AuthContext';
import { useState } from 'react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { User, KeyRound, CheckCircle } from 'lucide-react';
import { cn } from '@/lib/utils';

const ROLE_COLORS = {
  admin: 'text-primary border-primary/30 bg-primary/10',
  editor: 'text-infra border-infra-border bg-infra-bg',
  viewer: 'text-deploy border-deploy-border bg-deploy-bg',
};

const Account = () => {
  const { user } = useAuth();

  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const validationError =
    next && confirm && next !== confirm ? 'Passwords do not match' :
    next && next.length < 8 ? 'New password must be at least 8 characters' :
    null;

  const canSubmit = current && next && confirm && !validationError && !submitting;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setSubmitting(true);
    setError(null);
    setSuccess(false);
    try {
      await api.post('/api/auth/change-password', { currentPassword: current, newPassword: next });
      setSuccess(true);
      setCurrent(''); setNext(''); setConfirm('');
      setTimeout(() => setSuccess(false), 4000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update password');
    } finally {
      setSubmitting(false);
    }
  };

  if (!user) return null;

  return (
    <Layout>
      <div className="max-w-lg mx-auto space-y-8">
        {/* Header */}
        <div>
          <div className="flex items-center gap-2 mb-1">
            <User className="w-4 h-4 text-primary" />
            <h1 className="text-lg font-semibold text-foreground">My Account</h1>
          </div>
          <p className="text-sm text-muted-foreground">Your profile and security settings.</p>
        </div>

        {/* Profile */}
        <section className="space-y-4">
          <h2 className="text-base font-semibold text-foreground border-b border-border pb-2">Profile</h2>
          <div className="bg-card border border-border rounded-lg overflow-hidden">
            {[
              { label: 'Full name', value: user.name },
              { label: 'Email', value: user.email },
            ].map(({ label, value }, i) => (
              <div key={label} className={cn('flex items-center gap-4 px-4 py-3 text-sm', i > 0 && 'border-t border-border')}>
                <span className="text-muted-foreground w-24 shrink-0">{label}</span>
                <span className="text-foreground">{value}</span>
              </div>
            ))}
            <div className="flex items-center gap-4 px-4 py-3 text-sm border-t border-border">
              <span className="text-muted-foreground w-24 shrink-0">Role</span>
              <span className={cn('inline-flex items-center text-xs font-medium px-2 py-0.5 rounded-full border', ROLE_COLORS[user.role])}>
                {user.role}
              </span>
            </div>
          </div>
        </section>

        {/* Change password */}
        <section className="space-y-4">
          <h2 className="text-base font-semibold text-foreground border-b border-border pb-2 flex items-center gap-2">
            <KeyRound className="w-3.5 h-3.5" />
            Change Password
          </h2>
          <form onSubmit={handleSubmit} className="space-y-3">
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Current password</Label>
              <Input
                type="password"
                value={current}
                onChange={e => setCurrent(e.target.value)}
                autoComplete="current-password"
                className="bg-background border-border text-sm"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">New password</Label>
              <Input
                type="password"
                value={next}
                onChange={e => setNext(e.target.value)}
                autoComplete="new-password"
                className="bg-background border-border text-sm"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Confirm new password</Label>
              <Input
                type="password"
                value={confirm}
                onChange={e => setConfirm(e.target.value)}
                autoComplete="new-password"
                className={cn('bg-background border-border text-sm', validationError && 'border-destructive')}
              />
            </div>

            {validationError && (
              <p className="text-xs text-destructive">{validationError}</p>
            )}
            {error && (
              <p className="text-xs text-destructive p-2 rounded bg-destructive/10 border border-destructive/20">{error}</p>
            )}
            {success && (
              <p className="text-xs text-deploy flex items-center gap-1.5 p-2 rounded bg-deploy-bg border border-deploy-border">
                <CheckCircle className="w-3.5 h-3.5" />
                Password updated successfully.
              </p>
            )}

            <div className="pt-1">
              <Button type="submit" size="sm" disabled={!canSubmit}>
                {submitting ? 'Updating…' : 'Update Password'}
              </Button>
            </div>
          </form>
        </section>
      </div>
    </Layout>
  );
};

export default Account;
