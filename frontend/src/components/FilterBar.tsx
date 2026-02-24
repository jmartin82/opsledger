import { ChangeFilters, ChangeType, CHANGE_TYPE_LABELS, KNOWN_ENVIRONMENTS, KNOWN_SYSTEMS } from '@/types/change';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Search, X, CalendarRange } from 'lucide-react';
import { cn } from '@/lib/utils';

interface FilterBarProps {
  filters: ChangeFilters;
  onChange: (filters: ChangeFilters) => void;
  totalCount: number;
  filteredCount: number;
}

const TIME_RANGES = [
  { value: '30m', label: 'Last 30 min' },
  { value: '1h', label: 'Last 1 hour' },
  { value: '2h', label: 'Last 2 hours' },
  { value: '6h', label: 'Last 6 hours' },
  { value: '24h', label: 'Last 24 hours' },
  { value: '7d', label: 'Last 7 days' },
  { value: 'custom', label: 'Custom range…' },
];

// Format datetime-local value to a readable label
const formatCustomLabel = (from?: string, to?: string) => {
  const fmt = (s: string) =>
    new Date(s).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  if (from && to) return `${fmt(from)} → ${fmt(to)}`;
  if (from) return `From ${fmt(from)}`;
  if (to) return `Until ${fmt(to)}`;
  return 'Custom range';
};

const FilterBar = ({ filters, onChange, totalCount, filteredCount }: FilterBarProps) => {
  const isCustom = filters.timeRange === 'custom';
  const hasActiveFilters = Object.entries(filters).some(([k, v]) => v && v !== '' && k !== 'customFrom' && k !== 'customTo') ||
    (isCustom && (filters.customFrom || filters.customTo));

  const update = (key: keyof ChangeFilters, value: string) => {
    if (key === 'timeRange' && value !== 'custom') {
      // Clear custom range when switching away
      onChange({ ...filters, timeRange: value === 'all' ? '' : value as ChangeFilters['timeRange'], customFrom: '', customTo: '' });
    } else {
      onChange({ ...filters, [key]: value === 'all' ? '' : value });
    }
  };

  const clear = () => {
    onChange({ system: '', environment: '', user: '', type: '', timeRange: '', search: '', customFrom: '', customTo: '' });
  };

  return (
    <div className="space-y-3">
      {/* Search bar */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
        <Input
          placeholder="Search changes..."
          value={filters.search || ''}
          onChange={(e) => update('search', e.target.value)}
          className="pl-9 bg-card border-border font-mono text-sm placeholder:font-sans placeholder:text-muted-foreground"
        />
        {filters.search && (
          <button
            onClick={() => update('search', '')}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        )}
      </div>

      {/* Filter row */}
      <div className="flex flex-wrap gap-2 items-center">
        {/* Time range select */}
        <Select value={filters.timeRange || 'all'} onValueChange={(v) => update('timeRange', v)}>
          <SelectTrigger className={cn(
            'h-8 text-xs bg-card border-border',
            isCustom ? 'w-44' : 'w-36',
            isCustom && 'border-primary/50 text-primary'
          )}>
            {isCustom ? (
              <span className="flex items-center gap-1.5 truncate">
                <CalendarRange className="w-3 h-3 shrink-0" />
                {formatCustomLabel(filters.customFrom, filters.customTo)}
              </span>
            ) : (
              <SelectValue placeholder="Time range" />
            )}
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All time</SelectItem>
            {TIME_RANGES.map(r => (
              <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Custom range inputs — shown inline when custom is selected */}
        {isCustom && (
          <div className="flex items-center gap-1.5 bg-card border border-primary/30 rounded px-2.5 py-1 animate-slide-in">
            <CalendarRange className="w-3.5 h-3.5 text-primary shrink-0" />
            <input
              type="datetime-local"
              value={filters.customFrom || ''}
              onChange={e => update('customFrom', e.target.value)}
              className="bg-transparent text-xs font-mono text-foreground outline-none w-36 [color-scheme:dark]"
              placeholder="From"
            />
            <span className="text-muted-foreground text-xs">→</span>
            <input
              type="datetime-local"
              value={filters.customTo || ''}
              onChange={e => update('customTo', e.target.value)}
              className="bg-transparent text-xs font-mono text-foreground outline-none w-36 [color-scheme:dark]"
              placeholder="To"
            />
          </div>
        )}

        <Select value={filters.type || 'all'} onValueChange={(v) => update('type', v)}>
          <SelectTrigger className="w-36 h-8 text-xs bg-card border-border">
            <SelectValue placeholder="Type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All types</SelectItem>
            {(Object.keys(CHANGE_TYPE_LABELS) as ChangeType[]).map(t => (
              <SelectItem key={t} value={t}>{CHANGE_TYPE_LABELS[t]}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={filters.environment || 'all'} onValueChange={(v) => update('environment', v)}>
          <SelectTrigger className="w-36 h-8 text-xs bg-card border-border">
            <SelectValue placeholder="Environment" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All environments</SelectItem>
            {KNOWN_ENVIRONMENTS.map(e => (
              <SelectItem key={e} value={e}>{e}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={filters.system || 'all'} onValueChange={(v) => update('system', v)}>
          <SelectTrigger className="w-40 h-8 text-xs bg-card border-border">
            <SelectValue placeholder="System" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All systems</SelectItem>
            {KNOWN_SYSTEMS.map(s => (
              <SelectItem key={s} value={s} className="font-mono">{s}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        {hasActiveFilters && (
          <Button
            variant="ghost"
            size="sm"
            onClick={clear}
            className="h-8 text-xs text-muted-foreground hover:text-foreground gap-1 px-2"
          >
            <X className="w-3 h-3" />
            Clear
          </Button>
        )}

        {/* Count */}
        <div className="ml-auto text-xs text-muted-foreground font-mono">
          {hasActiveFilters ? (
            <span>
              <span className="text-foreground">{filteredCount}</span> / {totalCount} changes
            </span>
          ) : (
            <span><span className="text-foreground">{totalCount}</span> changes</span>
          )}
        </div>
      </div>
    </div>
  );
};

export default FilterBar;
