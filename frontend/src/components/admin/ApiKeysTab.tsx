import { useState } from 'react';
import { useAuth, ApiKey } from '@/contexts/AuthContext';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Copy, Eye, EyeOff, Plus, RotateCcw, XCircle, CheckCircle, Key } from 'lucide-react';
import { formatDistanceToNow, format } from 'date-fns';
import { cn } from '@/lib/utils';

const CopyField = ({ label, value }: { label: string; value: string }) => {
  const [copied, setCopied] = useState(false);
  return (
    <div className="space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <div className="flex items-center gap-2">
        <code className="flex-1 text-xs font-mono bg-background border border-border rounded px-3 py-2 text-foreground break-all min-w-0">
          {value}
        </code>

        <Button
          size="icon"
          variant="outline"
          className="shrink-0 h-8 w-8"
          onClick={() => { navigator.clipboard.writeText(value); setCopied(true); setTimeout(() => setCopied(false), 2000); }}
        >
          {copied ? <CheckCircle className="w-3.5 h-3.5 text-deploy" /> : <Copy className="w-3.5 h-3.5" />}
        </Button>
      </div>
    </div>
  );
};

const CreateKeyDialog = ({ open, onClose }: { open: boolean; onClose: () => void }) => {
  const { createApiKey } = useAuth();
  const [name, setName] = useState('');
  const [readScope, setReadScope] = useState(true);
  const [writeScope, setWriteScope] = useState(false);
  const [hasExpiry, setHasExpiry] = useState(false);
  const [expiresAt, setExpiresAt] = useState('');
  const [created, setCreated] = useState<{ key: string } | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const scopes: ApiKey['scopes'] = [
    ...(readScope ? ['changes:read' as const] : []),
    ...(writeScope ? ['changes:write' as const] : []),
  ];

  const handleCreate = async () => {
    if (!name || scopes.length === 0) return;
    setCreating(true);
    setError(null);
    try {
      const result = await createApiKey(name, scopes, hasExpiry ? expiresAt : undefined);
      setCreated(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create API key');
    } finally {
      setCreating(false);
    }
  };

  const handleClose = () => {
    setName(''); setReadScope(true); setWriteScope(false);
    setHasExpiry(false); setExpiresAt(''); setCreated(null);
    setError(null);
    onClose();
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="bg-card border-border max-w-md">
        <DialogHeader>
          <DialogTitle className="text-base">
            {created ? 'API Key Created' : 'Create API Key'}
          </DialogTitle>
        </DialogHeader>

        {created ? (
          <div className="space-y-4 py-2">
            <div className="p-3 rounded-lg bg-deploy-bg border border-deploy-border">
              <div className="flex items-start gap-2 mb-3">
                <CheckCircle className="w-4 h-4 text-deploy shrink-0 mt-0.5" />
                <p className="text-xs text-muted-foreground">
                  <span className="text-foreground font-medium">Copy this key now.</span>{' '}
                  It will not be shown again. Store it securely.
                </p>
              </div>
              <CopyField label="API Key (secret)" value={created.key} />
            </div>
          </div>
        ) : (
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Key name</Label>
              <Input
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="e.g. CI/CD Pipeline"
                className="bg-background border-border text-sm"
              />
            </div>

            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Scopes</Label>
              {[
                { label: 'changes:read', desc: 'Query the change log', checked: readScope, set: setReadScope },
                { label: 'changes:write', desc: 'Register new changes', checked: writeScope, set: setWriteScope },
              ].map(({ label, desc, checked, set }) => (
                <div key={label} className="flex items-center justify-between p-3 rounded-lg border border-border bg-background">
                  <div>
                    <p className="text-xs font-mono text-foreground">{label}</p>
                    <p className="text-xs text-muted-foreground">{desc}</p>
                  </div>
                  <Switch checked={checked} onCheckedChange={set} />
                </div>
              ))}
              {scopes.length === 0 && <p className="text-xs text-destructive">At least one scope is required.</p>}
              {error && <p className="text-xs text-destructive">{error}</p>}
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label className="text-xs text-muted-foreground">Expiry date (optional)</Label>
                <Switch checked={hasExpiry} onCheckedChange={setHasExpiry} />
              </div>
              {hasExpiry && (
                <Input
                  type="date"
                  value={expiresAt}
                  onChange={e => setExpiresAt(e.target.value)}
                  className="bg-background border-border text-sm"
                  min={new Date().toISOString().split('T')[0]}
                />
              )}
            </div>
          </div>
        )}

        <DialogFooter>
          {created ? (
            <Button size="sm" onClick={handleClose}>Done</Button>
          ) : (
            <>
              <Button variant="outline" size="sm" onClick={handleClose} disabled={creating}>Cancel</Button>
              <Button size="sm" onClick={handleCreate} disabled={!name || scopes.length === 0 || creating}>
                {creating ? 'Creating…' : 'Create Key'}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

const KeyRow = ({ k }: { k: ApiKey }) => {
  const { revokeApiKey, rotateApiKey } = useAuth();
  const [rotated, setRotated] = useState<{ key: string } | null>(null);
  const [showKey, setShowKey] = useState(false);
  const [busy, setBusy] = useState(false);

  const handleRotate = async () => {
    setBusy(true);
    try {
      const result = await rotateApiKey(k.id);
      setRotated(result);
    } catch {
      // Key list will refresh via fetchApiKeys; error is visible if key disappears
    } finally {
      setBusy(false);
    }
  };

  const handleRevoke = async () => {
    setBusy(true);
    try {
      await revokeApiKey(k.id);
    } catch {
      // Silent — list refresh will reflect actual state
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="px-4 py-3 border-b border-border last:border-0">
      <div className="flex items-start gap-4">
        {/* Icon */}
        <div className={cn(
          'w-7 h-7 rounded flex items-center justify-center shrink-0 mt-0.5',
          k.status === 'active' ? 'bg-primary/10 text-primary' : 'bg-muted/30 text-muted-foreground'
        )}>
          <Key className="w-3.5 h-3.5" />
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="text-sm font-medium text-foreground">{k.name}</span>
            <span className={cn(
              'text-xs font-medium px-2 py-0.5 rounded-full border',
              k.status === 'active' ? 'text-deploy border-deploy-border bg-deploy-bg' : 'text-muted-foreground border-border bg-muted/30'
            )}>{k.status}</span>
          </div>

          <div className="flex flex-wrap items-center gap-2 mb-1">
            <span className="font-mono text-xs text-muted-foreground">{k.prefix}</span>
            {k.scopes.map(s => (
              <span key={s} className="text-xs font-mono px-1.5 py-0.5 rounded bg-secondary text-muted-foreground">{s}</span>
            ))}
          </div>

          <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
            <span>Created {formatDistanceToNow(new Date(k.createdAt), { addSuffix: true })}</span>
            {k.lastUsed && <span>Last used {formatDistanceToNow(new Date(k.lastUsed), { addSuffix: true })}</span>}
            {k.expiresAt && <span>Expires {format(new Date(k.expiresAt), 'MMM d, yyyy')}</span>}
          </div>

          {rotated && (
            <div className="mt-2 p-2 rounded-lg bg-deploy-bg border border-deploy-border">
              <p className="text-xs text-muted-foreground mb-1 flex items-center gap-1">
                <CheckCircle className="w-3 h-3 text-deploy" />
                New key generated — copy now, will not be shown again.
              </p>
              <div className="flex items-center gap-2">
                <code className="flex-1 text-xs font-mono text-foreground overflow-hidden">
                  {showKey ? rotated.key : rotated.key.slice(0, 16) + '••••••••••••••••'}
                </code>
                <button onClick={() => setShowKey(v => !v)} className="text-muted-foreground hover:text-foreground">
                  {showKey ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                </button>
                <button onClick={() => navigator.clipboard.writeText(rotated.key)} className="text-muted-foreground hover:text-foreground">
                  <Copy className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Actions */}
        {k.status === 'active' && (
          <div className="flex gap-2 shrink-0">
            <Button variant="outline" size="sm" className="gap-1.5 h-7 text-xs" onClick={handleRotate} disabled={busy}>
              <RotateCcw className={cn('w-3 h-3', busy && 'animate-spin')} />
              Rotate
            </Button>
            <Button variant="outline" size="sm" className="gap-1.5 h-7 text-xs text-destructive hover:text-destructive" onClick={handleRevoke} disabled={busy}>
              <XCircle className="w-3 h-3" />
              Revoke
            </Button>
          </div>
        )}
      </div>
    </div>
  );
};

const ApiKeysTab = () => {
  const { apiKeys } = useAuth();
  const [showCreate, setShowCreate] = useState(false);
  const active = apiKeys.filter(k => k.status === 'active');
  const revoked = apiKeys.filter(k => k.status === 'revoked');

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {active.length} active key{active.length !== 1 ? 's' : ''}
          <span className="ml-2 text-xs text-muted-foreground/60">· Keys are hashed server-side and cannot be recovered</span>
        </p>
        <Button size="sm" onClick={() => setShowCreate(true)} className="gap-1.5">
          <Plus className="w-3.5 h-3.5" />
          Create Key
        </Button>
      </div>

      {/* Active keys */}
      {active.length > 0 && (
        <div className="border border-border rounded-lg overflow-hidden">
          {active.map(k => <KeyRow key={k.id} k={k} />)}
        </div>
      )}

      {/* Revoked keys */}
      {revoked.length > 0 && (
        <div>
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">Revoked</p>
          <div className="border border-border rounded-lg overflow-hidden opacity-60">
            {revoked.map(k => <KeyRow key={k.id} k={k} />)}
          </div>
        </div>
      )}

      {active.length === 0 && revoked.length === 0 && (
        <div className="text-center py-12 text-muted-foreground border border-border rounded-lg">
          <Key className="w-8 h-8 mx-auto mb-3 opacity-30" />
          <p className="text-sm">No API keys yet.</p>
        </div>
      )}

      <CreateKeyDialog open={showCreate} onClose={() => setShowCreate(false)} />
    </div>
  );
};

export default ApiKeysTab;
