import { useState, useEffect, useCallback } from 'react';
import { Plug, Plus, Copy, CheckCircle, Pencil, Trash2, ToggleLeft, ToggleRight } from 'lucide-react';
import Layout from '@/components/Layout';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8081';

interface Connector {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  jira_url?: string;
  mapping?: {
    type_map: Record<string, string>;
    environment_label_prefix: string;
  };
  created_by: number;
  created_at: string;
  updated_at: string;
}

const CHANGE_TYPES = ['infrastructure', 'deployment', 'configuration'];

const DEFAULT_MAPPING = {
  type_map: { Change: 'configuration', Deployment: 'deployment', Infrastructure: 'infrastructure' },
  environment_label_prefix: 'env:',
};

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
          {copied ? <CheckCircle className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
        </Button>
      </div>
    </div>
  );
};

interface ConnectorDialogProps {
  open: boolean;
  editing: Connector | null;
  onClose: () => void;
  onSaved: () => void;
}

const ConnectorDialog = ({ open, editing, onClose, onSaved }: ConnectorDialogProps) => {
  const [name, setName] = useState('');
  const [jiraURL, setJiraURL] = useState('');
  const [apiToken, setApiToken] = useState('');
  const [envPrefix, setEnvPrefix] = useState('env:');
  const [typeMap, setTypeMap] = useState<Record<string, string>>(DEFAULT_MAPPING.type_map);
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      if (editing) {
        setName(editing.name);
        setJiraURL(editing.jira_url ?? '');
        setApiToken('');
        setEnvPrefix(editing.mapping?.environment_label_prefix ?? 'env:');
        setTypeMap(editing.mapping?.type_map ?? DEFAULT_MAPPING.type_map);
        setEnabled(editing.enabled);
      } else {
        setName('');
        setJiraURL('');
        setApiToken('');
        setEnvPrefix('env:');
        setTypeMap({ ...DEFAULT_MAPPING.type_map });
        setEnabled(true);
      }
      setError(null);
    }
  }, [open, editing]);

  const handleSave = async () => {
    if (!name || !jiraURL) { setError('Name and Jira URL are required'); return; }
    if (!editing && !apiToken) { setError('API Token is required'); return; }

    setSaving(true);
    setError(null);
    try {
      const body = {
        name,
        jira_url: jiraURL,
        api_token: apiToken,
        mapping: { type_map: typeMap, environment_label_prefix: envPrefix },
        enabled,
      };
      if (editing) {
        await api.put(`/api/admin/connectors/${editing.id}`, body);
      } else {
        await api.post('/api/admin/connectors', body);
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save connector');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={v => { if (!v) onClose(); }}>
      <DialogContent className="bg-card border-border max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Connector' : 'Add Jira Connector'}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="conn-name" className="text-sm">Name</Label>
            <Input id="conn-name" value={name} onChange={e => setName(e.target.value)} placeholder="My Jira Connector" className="bg-background" />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="conn-url" className="text-sm">Jira Base URL</Label>
            <Input id="conn-url" value={jiraURL} onChange={e => setJiraURL(e.target.value)} placeholder="https://yourcompany.atlassian.net" className="bg-background" />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="conn-token" className="text-sm">
              API Token{editing && <span className="text-muted-foreground text-xs ml-1">(leave blank to keep existing)</span>}
            </Label>
            <Input id="conn-token" type="password" value={apiToken} onChange={e => setApiToken(e.target.value)} placeholder={editing ? '••••••••' : 'Jira API token'} className="bg-background" />
          </div>

          <div className="space-y-1.5">
            <Label className="text-sm">Issue Type → Change Type Mapping</Label>
            <div className="space-y-2 rounded border border-border p-3 bg-background">
              {Object.entries(typeMap).map(([issueType, changeType]) => (
                <div key={issueType} className="flex items-center gap-2">
                  <Input
                    value={issueType}
                    onChange={e => {
                      const next: Record<string, string> = {};
                      Object.entries(typeMap).forEach(([k, v]) => { next[k === issueType ? e.target.value : k] = v; });
                      setTypeMap(next);
                    }}
                    placeholder="Jira issue type"
                    className="bg-card text-xs h-8"
                  />
                  <span className="text-muted-foreground text-xs shrink-0">→</span>
                  <select
                    value={changeType}
                    onChange={e => setTypeMap({ ...typeMap, [issueType]: e.target.value })}
                    className="flex-1 text-xs h-8 rounded border border-border bg-card px-2 text-foreground"
                  >
                    {CHANGE_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
                  </select>
                  <Button size="icon" variant="ghost" className="h-8 w-8 text-muted-foreground hover:text-destructive"
                    onClick={() => { const next = { ...typeMap }; delete next[issueType]; setTypeMap(next); }}>
                    <Trash2 className="w-3.5 h-3.5" />
                  </Button>
                </div>
              ))}
              <Button size="sm" variant="outline" className="w-full text-xs h-8 mt-1"
                onClick={() => setTypeMap({ ...typeMap, '': 'configuration' })}>
                <Plus className="w-3 h-3 mr-1" /> Add mapping
              </Button>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="conn-env-prefix" className="text-sm">Environment label prefix</Label>
            <Input id="conn-env-prefix" value={envPrefix} onChange={e => setEnvPrefix(e.target.value)} placeholder="env:" className="bg-background" />
            <p className="text-xs text-muted-foreground">
              Labels on the Jira issue that start with this prefix are used to set the environment.
              For example, prefix <code className="font-mono bg-secondary px-1 rounded">env:</code> extracts{' '}
              <code className="font-mono bg-secondary px-1 rounded">production</code> from the label{' '}
              <code className="font-mono bg-secondary px-1 rounded">env:production</code>.
              Leave blank to not map an environment.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Switch id="conn-enabled" checked={enabled} onCheckedChange={setEnabled} />
            <Label htmlFor="conn-enabled" className="text-sm">Enabled</Label>
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={saving}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? 'Saving…' : (editing ? 'Save changes' : 'Create connector')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

const Connectors = () => {
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Connector | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await api.get<Connector[]>('/api/admin/connectors');
      setConnectors(data ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load connectors');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this connector? Jira will no longer send events to this webhook URL.')) return;
    setDeleting(id);
    try {
      await api.delete(`/api/admin/connectors/${id}`);
      setConnectors(cs => cs.filter(c => c.id !== id));
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete connector');
    } finally {
      setDeleting(null);
    }
  };

  return (
    <Layout>
      <div>
        <div className="mb-6 flex items-start justify-between">
          <div>
            <div className="flex items-center gap-2 mb-1">
              <Plug className="w-4 h-4 text-primary" />
              <h1 className="text-lg font-semibold text-foreground">Connectors</h1>
            </div>
            <p className="text-sm text-muted-foreground">
              Connect external sources to automatically register changes.
            </p>
          </div>
          <Button size="sm" onClick={() => { setEditing(null); setDialogOpen(true); }}>
            <Plus className="w-3.5 h-3.5 mr-1.5" />
            Add Connector
          </Button>
        </div>

        {loading && (
          <div className="text-sm text-muted-foreground py-12 text-center">Loading connectors…</div>
        )}

        {!loading && error && (
          <div className="text-sm text-destructive py-12 text-center">{error}</div>
        )}

        {!loading && !error && connectors.length === 0 && (
          <div className="border border-dashed border-border rounded-lg py-16 text-center">
            <Plug className="w-8 h-8 text-muted-foreground/40 mx-auto mb-3" />
            <p className="text-sm font-medium text-foreground">No connectors configured</p>
            <p className="text-xs text-muted-foreground mt-1 mb-4">
              Add a connector to automatically import changes from external tools like Jira.
            </p>
            <Button size="sm" variant="outline" onClick={() => { setEditing(null); setDialogOpen(true); }}>
              <Plus className="w-3.5 h-3.5 mr-1.5" />
              Add Connector
            </Button>
          </div>
        )}

        {!loading && !error && connectors.length > 0 && (
          <div className="space-y-4">
            <div className="space-y-2">
              {connectors.map(c => (
                <div key={c.id} className={cn(
                  'border border-border rounded-lg p-4 bg-card transition-opacity',
                  !c.enabled && 'opacity-60'
                )}>
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className={cn(
                        'w-2 h-2 rounded-full shrink-0',
                        c.enabled ? 'bg-green-500' : 'bg-muted-foreground/40'
                      )} />
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-foreground">{c.name}</span>
                          <span className="text-xs px-1.5 py-0.5 rounded bg-secondary text-muted-foreground font-mono">{c.type}</span>
                          {!c.enabled && <span className="text-xs text-muted-foreground">disabled</span>}
                        </div>
                        {c.jira_url && (
                          <p className="text-xs text-muted-foreground mt-0.5 truncate">{c.jira_url}</p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      <Button size="icon" variant="ghost" className="h-8 w-8"
                        onClick={() => { setEditing(c); setDialogOpen(true); }}>
                        <Pencil className="w-3.5 h-3.5" />
                      </Button>
                      <Button size="icon" variant="ghost" className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        disabled={deleting === c.id}
                        onClick={() => handleDelete(c.id)}>
                        <Trash2 className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </div>

                  <div className="mt-3 pt-3 border-t border-border">
                    <CopyField
                      label="Webhook URL"
                      value={`${API_URL}/api/connectors/${c.id}/webhook`}
                    />
                  </div>
                </div>
              ))}
            </div>

            <div className="rounded-md border border-border bg-card p-4 space-y-3">
              <p className="text-xs font-medium text-foreground">
                Jira webhook setup{' '}
                <span className="font-normal text-muted-foreground">— Settings → System → Webhooks → Create webhook</span>
              </p>
              <div className="space-y-2">
                {[
                  {
                    step: '1',
                    label: 'URL',
                    body: 'Paste the webhook URL from the connector above.',
                  },
                  {
                    step: '2',
                    label: 'Events',
                    body: <><em>Issue Created</em> and <em>Issue Updated</em>.</>,
                  },
                  {
                    step: '3',
                    label: 'JQL filter',
                    body: (
                      <>
                        Limit which issues trigger this webhook. Example:{' '}
                        <code className="font-mono bg-secondary px-1 rounded text-foreground">
                          project = OPS AND issuetype in (Change, Deployment)
                        </code>
                      </>
                    ),
                  },
                  {
                    step: '4',
                    label: 'Secret header',
                    body: (
                      <>
                        Add a custom request header{' '}
                        <code className="font-mono bg-secondary px-1 rounded text-foreground">X-Connector-Secret</code>{' '}
                        with the secret generated when the connector was created.
                      </>
                    ),
                  },
                ].map(({ step, label, body }) => (
                  <div key={step} className="flex gap-2.5 text-xs text-muted-foreground">
                    <span className="shrink-0 w-4 h-4 rounded-full bg-secondary text-foreground font-medium text-[10px] flex items-center justify-center mt-0.5">
                      {step}
                    </span>
                    <span>
                      <strong className="text-foreground">{label} — </strong>
                      {body}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>

      <ConnectorDialog
        open={dialogOpen}
        editing={editing}
        onClose={() => setDialogOpen(false)}
        onSaved={load}
      />
    </Layout>
  );
};

export default Connectors;
