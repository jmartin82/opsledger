import { useEffect, useState, useCallback } from 'react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';
import { LogIn, Shield, Key, RefreshCw, Ban, CheckCircle, UserPlus, ChevronLeft, ChevronRight } from 'lucide-react';

interface AuditEntry {
  id: string;
  actor: string;
  actorId?: string;
  action: string;
  targetType: string;
  targetId?: string;
  details?: string;
  ipAddress?: string;
  timestamp: string;
}

interface AuditResponse {
  entries: AuditEntry[];
  total: number;
  limit: number;
  offset: number;
}

const ACTION_ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  'user.login': LogIn,
  'user.register': UserPlus,
  'user.create': UserPlus,
  'user.role_change': Shield,
  'user.status_change': Ban,
  'user.password_reset': RefreshCw,
  'apikey.create': Key,
  'apikey.revoke': Ban,
  'apikey.rotate': RefreshCw,
  'change.create': CheckCircle,
};

const ACTION_LABELS: Record<string, string> = {
  'user.login': 'User login',
  'user.register': 'User registered',
  'user.create': 'User created',
  'user.role_change': 'Role changed',
  'user.status_change': 'Status changed',
  'user.password_reset': 'Password reset',
  'apikey.create': 'API key created',
  'apikey.revoke': 'API key revoked',
  'apikey.rotate': 'API key rotated',
  'change.create': 'Change created',
};

const PAGE_SIZE = 50;

const AuditTab = () => {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [actionFilter, setActionFilter] = useState('');

  const fetchAudit = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) });
      if (actionFilter) params.set('action', actionFilter);
      const data = await api.get<AuditResponse>(`/api/admin/audit?${params}`);
      setEntries(data.entries);
      setTotal(data.total);
    } catch {
      setEntries([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [offset, actionFilter]);

  useEffect(() => { fetchAudit(); }, [fetchAudit]);

  const handleFilterChange = (value: string) => {
    setActionFilter(value);
    setOffset(0);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{total} total events</p>
        <select
          className="text-sm border border-border rounded px-2 py-1 bg-background text-foreground"
          value={actionFilter}
          onChange={(e) => handleFilterChange(e.target.value)}
        >
          <option value="">All actions</option>
          {Object.entries(ACTION_LABELS).map(([key, label]) => (
            <option key={key} value={key}>{label}</option>
          ))}
        </select>
      </div>

      <div className="border border-border rounded-lg overflow-hidden">
        {loading ? (
          <div className="text-center py-12 text-muted-foreground">
            <p className="text-sm">Loading...</p>
          </div>
        ) : entries.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground">
            <Shield className="w-8 h-8 mx-auto mb-3 opacity-30" />
            <p className="text-sm">No audit events yet.</p>
          </div>
        ) : (
          entries.map((entry) => {
            const Icon = ACTION_ICONS[entry.action] ?? Shield;
            const label = ACTION_LABELS[entry.action] ?? entry.action;
            return (
              <div key={entry.id} className="flex items-start gap-3 px-4 py-3 border-b border-border last:border-0 hover:bg-accent/10 transition-colors">
                <div className="w-7 h-7 rounded bg-secondary flex items-center justify-center shrink-0 mt-0.5">
                  <Icon className="w-3.5 h-3.5 text-muted-foreground" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="text-sm font-medium text-foreground">{label}</span>
                    <span className="font-mono text-xs text-muted-foreground truncate">{entry.targetType}{entry.targetId ? ` #${entry.targetId}` : ''}</span>
                  </div>
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span>by <span className="text-foreground">{entry.actor}</span></span>
                    {entry.details && <span>· {entry.details}</span>}
                    {entry.ipAddress && <span>· {entry.ipAddress}</span>}
                  </div>
                </div>
                <span className="text-xs text-muted-foreground shrink-0 mt-0.5">
                  {formatDistanceToNow(new Date(entry.timestamp), { addSuffix: true })}
                </span>
              </div>
            );
          })
        )}
      </div>

      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between text-sm">
          <button
            className="flex items-center gap-1 text-muted-foreground hover:text-foreground disabled:opacity-30"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            <ChevronLeft className="w-4 h-4" /> Previous
          </button>
          <span className="text-muted-foreground">
            {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of {total}
          </span>
          <button
            className="flex items-center gap-1 text-muted-foreground hover:text-foreground disabled:opacity-30"
            disabled={offset + PAGE_SIZE >= total}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            Next <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      )}
    </div>
  );
};

export default AuditTab;
