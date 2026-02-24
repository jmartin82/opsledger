import { useState, useRef, useEffect } from 'react';
import { Change, ChangeType, KNOWN_ENVIRONMENTS, KNOWN_SYSTEMS } from '@/types/change';
import { api } from '@/lib/api';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import AutocompleteInput from '@/components/AutocompleteInput';
import { Server, Rocket, Settings } from 'lucide-react';
import { cn } from '@/lib/utils';

type ChangeTypeOption = { value: ChangeType; label: string; description: string; icon: React.ComponentType<{ className?: string }>; badge: string };

const TYPES: ChangeTypeOption[] = [
  { value: 'infrastructure', label: 'Infrastructure', description: 'Network, servers, cloud resources', icon: Server, badge: 'badge-infra' },
  { value: 'deployment', label: 'Deployment', description: 'Code releases, service updates', icon: Rocket, badge: 'badge-deploy' },
  { value: 'configuration', label: 'Configuration', description: 'Config files, feature flags, secrets', icon: Settings, badge: 'badge-config' },
];

interface EditChangeDialogProps {
  change: Change | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void;
}

const EditChangeDialog = ({ change, open, onOpenChange, onSaved }: EditChangeDialogProps) => {
  const [form, setForm] = useState({
    system: '',
    environment: '',
    user: '',
    type: '' as ChangeType | '',
    description: '',
    timestamp: '',
  });
  const [errors, setErrors] = useState<{ system?: string; type?: string; description?: string }>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const envRef = useRef<HTMLInputElement>(null);
  const userRef = useRef<HTMLInputElement>(null);
  const descRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (change) {
      // Convert ISO timestamp to datetime-local format (YYYY-MM-DDTHH:MM)
      const tsLocal = change.timestamp
        ? new Date(change.timestamp).toISOString().slice(0, 16)
        : '';
      setForm({
        system: change.system,
        environment: change.environment || '',
        user: change.user || '',
        type: change.type,
        description: change.description,
        timestamp: tsLocal,
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
    const e: { system?: string; type?: string; description?: string } = {};
    if (!form.system.trim()) e.system = 'System is required';
    if (!form.type) e.type = 'Change type is required';
    if (!form.description.trim()) e.description = 'Description is required';
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

          {/* Timestamp */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Timestamp</Label>
            <Input
              type="datetime-local"
              value={form.timestamp}
              onChange={e => set('timestamp', e.target.value)}
              className="bg-card border-border text-sm w-64"
            />
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

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" size="sm" disabled={submitting}>
              {submitting ? 'Saving...' : 'Save Changes'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
};

export default EditChangeDialog;
