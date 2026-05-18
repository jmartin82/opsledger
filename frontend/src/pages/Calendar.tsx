import { useState, useEffect, useCallback } from 'react';
import { DayPicker } from 'react-day-picker';
import { format, startOfMonth, endOfMonth, parseISO, isSameDay } from 'date-fns';
import Layout from '@/components/Layout';
import { api } from '@/lib/api';
import { Change } from '@/types/change';
import { CalendarDays, AlertTriangle, CheckCircle2, ChevronLeft, ChevronRight, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/use-toast';
import { Button } from '@/components/ui/button';

interface ChangesResponse {
  changes: Change[];
  total: number;
}

const CalendarPage = () => {
  const { toast } = useToast();
  const [month, setMonth] = useState(new Date());
  const [scheduled, setScheduled] = useState<Change[]>([]);
  const [overdue, setOverdue] = useState<Change[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedDay, setSelectedDay] = useState<Date | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);

  const fetchScheduled = useCallback(async (m: Date) => {
    setLoading(true);
    try {
      const from = startOfMonth(m).toISOString();
      const to = endOfMonth(m).toISOString();
      const [scheduledRes, overdueRes] = await Promise.all([
        api.get<ChangesResponse>(`/api/changes?status=scheduled&from=${from}&to=${to}&limit=200`),
        api.get<ChangesResponse>(`/api/changes?status=overdue&limit=200`),
      ]);
      setScheduled(scheduledRes.changes ?? []);
      setOverdue(overdueRes.changes ?? []);
    } catch (err) {
      toast({ title: 'Failed to load calendar', description: err instanceof Error ? err.message : undefined, variant: 'destructive' });
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    fetchScheduled(month);
  }, [month, fetchScheduled]);

  const handleConfirm = async (change: Change) => {
    setConfirming(change.id);
    try {
      await api.patch(`/api/changes/${change.id}/confirm`, {});
      toast({ title: 'Change confirmed', description: `${change.system} marked as executed` });
      fetchScheduled(month);
      setSelectedDay(null);
    } catch (err) {
      toast({ title: 'Failed to confirm', description: err instanceof Error ? err.message : undefined, variant: 'destructive' });
    } finally {
      setConfirming(null);
    }
  };

  // Group scheduled changes by date string
  const byDate = scheduled.reduce<Record<string, Change[]>>((acc, c) => {
    const key = format(parseISO(c.timestamp), 'yyyy-MM-dd');
    (acc[key] ??= []).push(c);
    return acc;
  }, {});

  const selectedChanges = selectedDay
    ? scheduled.filter(c => isSameDay(parseISO(c.timestamp), selectedDay))
    : [];

  const daysWithEvents = Object.keys(byDate).map(d => new Date(d + 'T12:00:00'));

  return (
    <Layout>
      <div className="max-w-5xl mx-auto">
        {/* Header */}
        <div className="flex items-center gap-2 mb-6">
          <CalendarDays className="w-4 h-4 text-primary" />
          <h1 className="text-lg font-semibold text-foreground">Upcoming Changes</h1>
        </div>

        {/* Overdue section */}
        {overdue.length > 0 && (
          <div className="mb-6 rounded-lg border border-amber-500/40 bg-amber-500/5 p-4">
            <div className="flex items-center gap-2 mb-3">
              <AlertTriangle className="w-4 h-4 text-amber-500" />
              <span className="text-sm font-medium text-amber-600">
                Overdue ({overdue.length})
              </span>
            </div>
            <div className="flex flex-wrap gap-2">
              {overdue.map(c => (
                <div
                  key={c.id}
                  className="flex items-center gap-2 bg-card border border-amber-500/30 rounded px-2.5 py-1.5 text-xs"
                >
                  <span className="font-mono font-semibold text-foreground">{c.system}</span>
                  <span className="text-muted-foreground">{c.environment && `· ${c.environment}`}</span>
                  <span className="text-muted-foreground">
                    {format(parseISO(c.timestamp), 'MMM d')}
                  </span>
                  <button
                    onClick={() => handleConfirm(c)}
                    disabled={confirming === c.id}
                    className="flex items-center gap-1 text-emerald-600 hover:text-emerald-500 transition-colors disabled:opacity-50"
                  >
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    {confirming === c.id ? 'Confirming...' : 'Done'}
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="flex gap-6 items-start">
          {/* Calendar grid */}
          <div className="rounded-lg border border-border bg-card p-4 shrink-0">
            {loading ? (
              <div className="flex items-center justify-center w-72 h-72">
                <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <DayPicker
                mode="single"
                selected={selectedDay ?? undefined}
                onSelect={(d) => setSelectedDay(d ?? null)}
                month={month}
                onMonthChange={setMonth}
                modifiers={{ hasEvent: daysWithEvents }}
                modifiersClassNames={{ hasEvent: 'has-event' }}
                classNames={{
                  months: 'flex flex-col',
                  month: 'space-y-3',
                  caption: 'flex justify-center relative items-center h-8',
                  caption_label: 'text-sm font-semibold text-foreground',
                  nav: 'flex items-center gap-1',
                  nav_button: 'h-7 w-7 flex items-center justify-center rounded border border-border bg-card hover:bg-accent transition-colors',
                  nav_button_previous: 'absolute left-0',
                  nav_button_next: 'absolute right-0',
                  table: 'w-full border-collapse',
                  head_row: 'flex',
                  head_cell: 'text-muted-foreground w-10 text-center text-xs font-medium pb-2',
                  row: 'flex w-full',
                  cell: 'w-10 h-10 text-center p-0 relative',
                  day: 'w-10 h-10 text-sm font-normal rounded hover:bg-accent transition-colors flex items-center justify-center relative',
                  day_selected: 'bg-primary text-primary-foreground hover:bg-primary',
                  day_today: 'font-semibold text-primary',
                  day_outside: 'text-muted-foreground opacity-40',
                }}
                components={{
                  IconLeft: () => <ChevronLeft className="w-4 h-4" />,
                  IconRight: () => <ChevronRight className="w-4 h-4" />,
                  DayContent: ({ date }) => {
                    const key = format(date, 'yyyy-MM-dd');
                    const count = byDate[key]?.length ?? 0;
                    return (
                      <div className="flex flex-col items-center justify-center w-full h-full">
                        <span>{date.getDate()}</span>
                        {count > 0 && (
                          <span className="absolute bottom-1 left-1/2 -translate-x-1/2 w-1.5 h-1.5 rounded-full bg-blue-500" />
                        )}
                      </div>
                    );
                  },
                }}
              />
            )}
          </div>

          {/* Day detail panel */}
          <div className="flex-1 min-w-0">
            {selectedDay ? (
              <div>
                <h2 className="text-sm font-semibold text-foreground mb-3">
                  {format(selectedDay, 'EEEE, MMMM d')}
                </h2>
                {selectedChanges.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No scheduled changes on this day.</p>
                ) : (
                  <div className="space-y-2">
                    {selectedChanges.map(c => (
                      <DayChangeCard
                        key={c.id}
                        change={c}
                        onConfirm={handleConfirm}
                        confirming={confirming === c.id}
                      />
                    ))}
                  </div>
                )}
              </div>
            ) : (
              <div className="text-sm text-muted-foreground py-4">
                <p>Select a day to see scheduled changes.</p>
                {!loading && scheduled.length === 0 && overdue.length === 0 && (
                  <p className="mt-2">No upcoming changes this month.</p>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </Layout>
  );
};

interface DayChangeCardProps {
  change: Change;
  onConfirm: (c: Change) => void;
  confirming: boolean;
}

const DayChangeCard = ({ change, onConfirm, confirming }: DayChangeCardProps) => {
  const TYPE_BADGE: Record<string, string> = {
    infrastructure: 'badge-infra',
    deployment: 'badge-deploy',
    configuration: 'badge-config',
  };

  return (
    <div className={cn(
      'rounded-lg border border-border bg-card p-3 space-y-1.5',
    )}>
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className={cn('text-xs font-medium px-1.5 py-0.5 rounded', TYPE_BADGE[change.type])}>
            {change.type}
          </span>
          <span className="font-mono text-xs font-semibold text-foreground bg-secondary px-2 py-0.5 rounded">
            {change.system}
          </span>
          {change.environment && (
            <span className="text-xs text-muted-foreground bg-muted border border-border px-1.5 py-0.5 rounded">
              {change.environment}
            </span>
          )}
          <span className="text-xs text-muted-foreground">
            {format(parseISO(change.timestamp), 'HH:mm')}
          </span>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onConfirm(change)}
          disabled={confirming}
          className="gap-1 h-7 text-xs text-emerald-600 border-emerald-500/30 hover:bg-emerald-500/10 shrink-0"
        >
          <CheckCircle2 className="w-3.5 h-3.5" />
          {confirming ? 'Confirming...' : 'Mark Done'}
        </Button>
      </div>
      <p className="text-sm text-foreground leading-snug">{change.description}</p>
      {change.user && (
        <p className="text-xs text-muted-foreground">by {change.user}</p>
      )}
    </div>
  );
};

export default CalendarPage;
