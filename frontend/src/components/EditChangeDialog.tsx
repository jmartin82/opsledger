import { useState, useRef, useEffect } from 'react';
import { Change, ChangeType, KNOWN_ENVIRONMENTS, KNOWN_SYSTEMS } from '@/types/change';
import { api } from '@/lib/api';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import AutocompleteInput from '@/components/AutocompleteInput';
import { Server, Rocket, Settings, Clock, Calendar, CheckCircle2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/use-toast';

type ChangeTypeOption = { value: ChangeType; label: string; description: string; icon: React.ComponentType<{ className?: string }>; badge: string };
type ChangeMode = 'executed' | 'scheduled';

const TYPES: ChangeTypeOption[] = [
  { value: 'infrastructure', label: 'Infrastructure', description: 'Network, servers, cloud resources', icon: Server, badge: 'badge-infra' },
  { value: 'deployment', label: 'Deployment', description: 'Code releases, service updates', icon: Rocket, badge: 'badge-deploy' },
  { value: 'configuration', label: 'Configuration', description: 'Config files, feature flags, secrets', icon: Settings, badge: 'badge-config' },
];

const toLocalDatetime = (isoStr: string) => {
  if (!isoStr) return '';
  return new Date(isoStr).toISOString().slice(0, 16);
};

interface EditChangeDialogProps {
  change: Change | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void;
}

const EditChangeDialog = ({ change, open, onOpenChange, onSaved }: EditChangeDialogProps) => {
  const { toast } = useToast();
  const [mode, setMode] = useState<ChangeMode>('executed');
  const [form, setForm] = useState({
    system: '',
    environment: '',
    user: '',
    type: '' as ChangeType | '',
    description: '',
    timestamp: '',
  });
  const [errors, setErrors] = useState<{ system?: string; type?: string; description?: string; timestamp?: string }>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const envRef = useRef<HTMLInputElement>(null);
  const userRef = useRef<HTMLInputElement>(null);
  const descRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (change) {
      setMode((change.status as ChangeMode) || 'executed');
      setForm({
        system: change.system,
        environment: change.environment || '',
        user: change.user || '',
        type: change.type,
        description: change.description,
        timestamp: toLocalDatetime(change.timestamp),
      });
      setErrors({});
      setSubmitError(null);
    }
  }, [change]);

  const set = (k: keyof typeof form, v: string) => {
    setForm(f => ({ ...f, [k]: v }));
    if (errors[k as keyof typeof errors]) setErrors(e => ({ ...e, [k]: '' }));
  };

  const validate = () => {
    const e: typeof errors = {};
    if (!form.system.trim()) e.system = 'System is required';
    if (!form.type) e.type = 'Change type is required';
    if (!form.description.trim()) e.description = 'Description is required';
    if (mode === 'scheduled' && !form.timestamp) e.timestamp = 'Scheduled date is required';
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate() || !change) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      await api.put(`/api/changes/${change.id}`, {
        system: form.system.trim(),
        environment: form.environment.trim() || null,
        user: form.user.trim() || null,
        type: form.type,
        description: form.description.trim(),
        status: mode,
        timestamp: form.timestamp ? new Date(form.timestamp).toISOString() : undefined,
      });
      onOpenChange(false);
      onSaved();
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Failed to update change');
    } finally {
      setSubmitting(false);
    }
  };

  const handleConfirm = async () => {
    if (!change) return;
    setConfirming(true);
    try {
      await api.patch(`/api/changes/${change.id}/confirm`, {});
      toast({ title: 'Change confirmed', description: `${change.system} marked as executed` });
      onOpenChange(false);
      onSaved();
    } catch (err) {
      toast({ title: 'Failed to confirm change', description: err instanceof Error ? err.message : undefined, variant: 'destructive' });
    } finally {
      setConfirming(false);
    }
  };

  const isScheduled = change?.status === 'scheduled';
  const isOverdue = isScheduled && change?.timestamp ? new Date(change.timestamp) < new Date() : false;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>Edit Change</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Change Type */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Change Type <span className="text-destructive">*</span>
            </Label>
            <div className="grid grid-cols-3 gap-2">
              {TYPES.map(({ value, label, description, badge }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => set('type', value)}
                  className={cn(
                    'flex flex-col gap-1 p-2.5 rounded-lg border text-left transition-all',
                    form.type === value
                      ? 'border-primary bg-primary/5 ring-1 ring-primary'
                      : 'border-border bg-card hover:border-border/60 hover:bg-accent/40'
                  )}
                >
                  <span className={cn('text-xs font-medium px-1.5 py-0.5 rounded', badge)}>{label}</span>
                  <p className="text-xs text-muted-foreground leading-snug">{description}</p>
                </button>
              ))}
            </div>
            {errors.type && <p className="text-xs text-destructive">{errors.type}</p>}
          </div>

          {/* System + Environment */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                System <span className="text-destructive">*</span>
              </Label>
              <AutocompleteInput
                value={form.system}
                onChange={v => set('system', v)}
                suggestions={KNOWN_SYSTEMS}
                placeholder="e.g. api-gateway"
                onNext={() => envRef.current?.focus()}
                mono
              />
              {errors.system && <p className="text-xs text-destructive">{errors.system}</p>}
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Environment</Label>
              <AutocompleteInput
                value={form.environment}
                onChange={v => set('environment', v)}
                suggestions={KNOWN_ENVIRONMENTS}
                placeholder="e.g. production"
                onNext={() => userRef.current?.focus()}
                inputRef={envRef}
              />
            </div>
          </div>

          {/* User */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">User</Label>
            <Input
              value={form.user}
              onChange={e => set('user', e.target.value)}
              placeholder="e.g. alice.martin"
              className="bg-card border-border text-sm"
              ref={userRef}
            />
          </div>

          {/* When — mode toggle */}
          <div className="space-y-2">
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">When</Label>
            <div className="flex rounded-lg border border-border bg-card p-0.5 w-fit gap-0.5">
              <button
                type="button"
                onClick={() => setMode('executed')}
                className={cn(
                  'flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium transition-all',
                  mode === 'executed'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                <Clock className="w-3.5 h-3.5" />
                Already happened
              </button>
              <button
                type="button"
                onClick={() => setMode('scheduled')}
                className={cn(
                  'flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium transition-all',
                  mode === 'scheduled'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                <Calendar className="w-3.5 h-3.5" />
                Schedule for later
              </button>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">
                {mode === 'executed' ? 'When did it happen?' : 'Scheduled for'}
              </span>
              <Input
                type="datetime-local"
                value={form.timestamp}
                onChange={e => set('timestamp', e.target.value)}
                className="bg-card border-border text-sm w-64"
              />
              {errors.timestamp && <p className="text-xs text-destructive">{errors.timestamp}</p>}
            </div>
          </div>

          {/* Description */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Description <span className="text-destructive">*</span>
            </Label>
            <Textarea
              ref={descRef}
              value={form.description}
              onChange={e => set('description', e.target.value)}
              placeholder="Describe what changed..."
              className="bg-card border-border text-sm resize-none min-h-[80px]"
              rows={3}
            />
            {errors.description && <p className="text-xs text-destructive">{errors.description}</p>}
          </div>

          {submitError && <p className="text-xs text-destructive">{submitError}</p>}

          <div className="flex justify-between gap-2 pt-2">
            {/* Confirm button — only for scheduled changes */}
            <div>
              {isScheduled && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={handleConfirm}
                  disabled={confirming}
                  className={cn('gap-1.5', isOverdue && 'border-amber-500/50 text-amber-600 hover:bg-amber-500/10')}
                >
                  <CheckCircle2 className="w-3.5 h-3.5" />
                  {confirming ? 'Confirming...' : 'Mark as Done'}
                </Button>
              )}
            </div>
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" size="sm" disabled={submitting}>
                {submitting ? 'Saving...' : 'Save Changes'}
              </Button>
            </div>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
};

export default EditChangeDialog;
