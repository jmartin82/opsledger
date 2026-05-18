import { useState, useEffect, useCallback, useRef } from 'react';
import { Change, ChangeFilters } from '@/types/change';
import { api } from '@/lib/api';
import ChangeItem from '@/components/ChangeItem';
import FilterBar from '@/components/FilterBar';
import EditChangeDialog from '@/components/EditChangeDialog';
import DeleteChangeDialog from '@/components/DeleteChangeDialog';
import Layout from '@/components/Layout';
import { Activity, Plus, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Link } from 'react-router-dom';
import { useLive } from '@/contexts/LiveContext';

interface ChangesResponse {
  changes: Change[];
  total: number;
  limit: number;
  offset: number;
}

function buildQueryString(filters: ChangeFilters): string {
  const params = new URLSearchParams();

  if (filters.system) params.set('system', filters.system);
  if (filters.environment) params.set('environment', filters.environment);
  if (filters.user) params.set('user', filters.user);
  if (filters.type) params.set('type', filters.type);
  if (filters.status) params.set('status', filters.status);
  if (filters.search) params.set('search', filters.search);

  if (filters.timeRange === 'custom') {
    if (filters.customFrom) params.set('from', new Date(filters.customFrom).toISOString());
    if (filters.customTo) params.set('to', new Date(filters.customTo).toISOString());
  } else if (filters.timeRange) {
    params.set('timeRange', filters.timeRange);
  }

  const qs = params.toString();
  return qs ? `?${qs}` : '';
}

/** Returns true if a change passes the current equality filters. */
function passesFilters(change: Change, filters: ChangeFilters): boolean {
  if (filters.system && change.system !== filters.system) return false;
  if (filters.environment && change.environment !== filters.environment) return false;
  if (filters.type && change.type !== filters.type) return false;
  if (filters.status) {
    const isOverdue = change.status === 'scheduled' && new Date(change.timestamp) < new Date();
    if (filters.status === 'overdue' && !isOverdue) return false;
    if (filters.status === 'executed' && change.status !== 'executed') return false;
    if (filters.status === 'scheduled' && change.status !== 'scheduled') return false;
  }
  // Time-range always passes for new arrivals — they were just created
  return true;
}

const Index = () => {
  const [filters, setFilters] = useState<ChangeFilters>({});
  const [changes, setChanges] = useState<Change[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingChange, setEditingChange] = useState<Change | null>(null);
  const [deletingChange, setDeletingChange] = useState<Change | null>(null);

  const { connected, subscribe } = useLive();

  // Keep a ref to current filters so SSE callbacks see fresh state
  // without needing to re-register on every filter change.
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  const fetchChanges = useCallback(async (f: ChangeFilters) => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<ChangesResponse>(`/api/changes${buildQueryString(f)}`);
      setChanges(data.changes);
      setTotal(data.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load changes');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchChanges(filters);
  }, [filters, fetchChanges]);

  // SSE subscriptions
  useEffect(() => {
    const unsubCreate = subscribe('change.created', (data) => {
      const change = data as Change;
      if (!passesFilters(change, filtersRef.current)) return;
      setChanges(prev => [change, ...prev]);
      setTotal(prev => prev + 1);
    });

    const unsubUpdate = subscribe('change.updated', (data) => {
      const updated = data as Change;
      setChanges(prev => prev.map(c => c.id === updated.id ? updated : c));
    });

    const unsubDelete = subscribe('change.deleted', (data) => {
      const { id } = data as { id: string };
      setChanges(prev => {
        const exists = prev.some(c => c.id === id);
        if (exists) setTotal(t => t - 1);
        return prev.filter(c => c.id !== id);
      });
    });

    return () => {
      unsubCreate();
      unsubUpdate();
      unsubDelete();
    };
  }, [subscribe]);

  return (
    <Layout>
      {/* Page header */}
      <div className="flex items-start justify-between mb-6 gap-4">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Activity className="w-4 h-4 text-primary" />
            <h1 className="text-lg font-semibold text-foreground">Change Log</h1>
          </div>
          <p className="text-sm text-muted-foreground">
            Real-time view of infrastructure, deployment, and configuration changes across all systems.
          </p>
        </div>
        <Link to="/add">
          <Button size="sm" className="gap-1.5 shrink-0">
            <Plus className="w-3.5 h-3.5" />
            Register Change
          </Button>
        </Link>
      </div>

      {/* Filters */}
      <div className="mb-4">
        <FilterBar
          filters={filters}
          onChange={setFilters}
          totalCount={total}
          filteredCount={changes.length}
        />
      </div>

      {/* Change list */}
      <div className="space-y-2">
        {loading ? (
          <div className="text-center py-16 text-muted-foreground">
            <Loader2 className="w-6 h-6 mx-auto mb-3 animate-spin opacity-50" />
            <p className="text-sm">Loading changes...</p>
          </div>
        ) : error ? (
          <div className="text-center py-16 text-muted-foreground">
            <Activity className="w-8 h-8 mx-auto mb-3 opacity-30" />
            <p className="text-sm text-destructive">{error}</p>
            <button
              onClick={() => fetchChanges(filters)}
              className="text-xs text-primary hover:underline mt-1"
            >
              Retry
            </button>
          </div>
        ) : changes.length === 0 ? (
          <div className="text-center py-16 text-muted-foreground">
            <Activity className="w-8 h-8 mx-auto mb-3 opacity-30" />
            <p className="text-sm">No changes match the current filters.</p>
            <button
              onClick={() => setFilters({})}
              className="text-xs text-primary hover:underline mt-1"
            >
              Clear filters
            </button>
          </div>
        ) : (
          changes.map(change => (
            <ChangeItem
              key={change.id}
              change={change}
              onEdit={setEditingChange}
              onDelete={setDeletingChange}
              onConfirm={(confirmed) => setChanges(prev => prev.map(c => c.id === confirmed.id ? confirmed : c))}
            />
          ))
        )}
      </div>
      <EditChangeDialog
        change={editingChange}
        open={editingChange !== null}
        onOpenChange={(open) => { if (!open) setEditingChange(null); }}
        onSaved={() => {
          setEditingChange(null);
          // Only refetch when SSE is offline — SSE handles it when connected
          if (!connected) fetchChanges(filters);
        }}
      />

      <DeleteChangeDialog
        change={deletingChange}
        open={deletingChange !== null}
        onOpenChange={(open) => { if (!open) setDeletingChange(null); }}
        onDeleted={() => {
          setDeletingChange(null);
          // Only refetch when SSE is offline — SSE handles it when connected
          if (!connected) fetchChanges(filters);
        }}
      />
    </Layout>
  );
};

export default Index;
