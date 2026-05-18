import { Change, CHANGE_TYPE_COLORS, CHANGE_TYPE_LABELS, ChangeType } from '@/types/change';
import { cn } from '@/lib/utils';
import { Server, Rocket, Settings, User, Globe, Clock, Pencil, Trash2, CalendarClock, CheckCircle2, AlertTriangle } from 'lucide-react';
import { formatDistanceToNow, format, parseISO } from 'date-fns';
import { useAuth } from '@/contexts/AuthContext';
import { api } from '@/lib/api';
import { useToast } from '@/hooks/use-toast';

const TYPE_ICONS: Record<ChangeType, React.ComponentType<{ className?: string }>> = {
  infrastructure: Server,
  deployment: Rocket,
  configuration: Settings,
};

interface ChangeItemProps {
  change: Change;
  onEdit?: (change: Change) => void;
  onDelete?: (change: Change) => void;
  onConfirm?: (change: Change) => void;
}

const ChangeItem = ({ change, onEdit, onDelete, onConfirm }: ChangeItemProps) => {
  const { can } = useAuth();
  const { toast } = useToast();
  const Icon = TYPE_ICONS[change.type];
  const badgeClass = CHANGE_TYPE_COLORS[change.type];
  const label = CHANGE_TYPE_LABELS[change.type];
  const parsedDate = parseISO(change.timestamp);
  const showActions = can('edit_changes') && (onEdit || onDelete || onConfirm);

  const isScheduled = change.status === 'scheduled';
  const isOverdue = isScheduled && parsedDate < new Date();

  const handleConfirm = async () => {
    try {
      const confirmed = await api.patch<Change>(`/api/changes/${change.id}/confirm`, {});
      toast({ title: 'Change confirmed', description: `${change.system} marked as executed` });
      onConfirm?.(confirmed);
    } catch {
      toast({ title: 'Failed to confirm change', variant: 'destructive' });
    }
  };

  return (
    <div className={cn(
      'flex gap-4 p-4 rounded-lg border border-border bg-card hover:border-border/80',
      'hover:bg-accent/30 transition-colors group animate-fade-in relative',
      isOverdue && 'border-amber-500/40 bg-amber-500/5',
    )}>
      {/* Action buttons — top-right, visible on hover */}
      {showActions && (
        <div className="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {isScheduled && onConfirm && (
            <button
              onClick={handleConfirm}
              className="p-1.5 rounded hover:bg-emerald-500/10 text-muted-foreground hover:text-emerald-500 transition-colors"
              title="Mark as done"
            >
              <CheckCircle2 className="w-3.5 h-3.5" />
            </button>
          )}
          {onEdit && (
            <button
              onClick={() => onEdit(change)}
              className="p-1.5 rounded hover:bg-secondary text-muted-foreground hover:text-foreground transition-colors"
              title="Edit change"
            >
              <Pencil className="w-3.5 h-3.5" />
            </button>
          )}
          {onDelete && (
            <button
              onClick={() => onDelete(change)}
              className="p-1.5 rounded hover:bg-destructive/10 text-muted-foreground hover:text-destructive transition-colors"
              title="Delete change"
            >
              <Trash2 className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      )}

      {/* Left — icon */}
      <div className="shrink-0 mt-0.5">
        <div className={cn(
          'w-8 h-8 rounded flex items-center justify-center',
          change.type === 'infrastructure' && 'bg-infra-bg',
          change.type === 'deployment' && 'bg-deploy-bg',
          change.type === 'configuration' && 'bg-config-bg',
        )}>
          <Icon className={cn(
            'w-4 h-4',
            change.type === 'infrastructure' && 'text-infra',
            change.type === 'deployment' && 'text-deploy',
            change.type === 'configuration' && 'text-config',
          )} />
        </div>
      </div>

      {/* Center — content */}
      <div className="flex-1 min-w-0">
        <div className="flex flex-wrap items-center gap-2 mb-1.5">
          {/* Type badge */}
          <span className={cn('text-xs font-medium px-1.5 py-0.5 rounded', badgeClass)}>
            {label}
          </span>

          {/* Status badge — only for non-executed */}
          {isOverdue ? (
            <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-600 border border-amber-500/30 flex items-center gap-1">
              <AlertTriangle className="w-3 h-3" />
              Overdue
            </span>
          ) : isScheduled ? (
            <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-500 border border-blue-500/20 flex items-center gap-1">
              <CalendarClock className="w-3 h-3" />
              Scheduled
            </span>
          ) : null}

          {/* System */}
          <span className="font-mono text-xs font-semibold text-foreground bg-secondary px-2 py-0.5 rounded">
            {change.system}
          </span>

          {/* Environment */}
          {change.environment && (
            <span className={cn(
              'text-xs px-1.5 py-0.5 rounded flex items-center gap-1',
              change.environment === 'production'
                ? 'text-destructive bg-destructive/10 border border-destructive/20'
                : 'text-muted-foreground bg-muted border border-border'
            )}>
              <Globe className="w-3 h-3" />
              {change.environment}
            </span>
          )}
        </div>

        {/* Description */}
        <p className="text-sm text-foreground leading-snug">
          {change.description}
        </p>

        {/* Footer meta */}
        <div className="flex items-center gap-3 mt-2 text-xs text-muted-foreground">
          {change.user && (
            <span className="flex items-center gap-1">
              <User className="w-3 h-3" />
              {change.user}
            </span>
          )}
          <span className="flex items-center gap-1" title={format(parsedDate, 'PPpp')}>
            <Clock className="w-3 h-3" />
            {isScheduled
              ? `Scheduled ${formatDistanceToNow(parsedDate, { addSuffix: true })}`
              : formatDistanceToNow(parsedDate, { addSuffix: true })}
          </span>
          <span className="font-mono text-muted-foreground/60 hidden sm:block">
            {format(parsedDate, 'yyyy-MM-dd HH:mm:ss')} UTC
          </span>
        </div>
      </div>
    </div>
  );
};

export default ChangeItem;
