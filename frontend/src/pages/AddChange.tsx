import { useState, useRef } from 'react';
import Layout from '@/components/Layout';
import { ChangeType, KNOWN_ENVIRONMENTS, KNOWN_SYSTEMS } from '@/types/change';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import AutocompleteInput from '@/components/AutocompleteInput';
import { Plus, CheckCircle, CalendarClock, Server, Rocket, Settings, Clock, Calendar } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useNavigate } from 'react-router-dom';
import { format, parseISO } from 'date-fns';

type ChangeMode = 'executed' | 'scheduled';

type ChangeTypeOption = { value: ChangeType; label: string; description: string; icon: React.ComponentType<{ className?: string }>; badge: string };

const TYPES: ChangeTypeOption[] = [
  { value: 'infrastructure', label: 'Infrastructure', description: 'Network, servers, cloud resources', icon: Server, badge: 'badge-infra' },
  { value: 'deployment', label: 'Deployment', description: 'Code releases, service updates', icon: Rocket, badge: 'badge-deploy' },
  { value: 'configuration', label: 'Configuration', description: 'Config files, feature flags, secrets', icon: Settings, badge: 'badge-config' },
];

const toLocalDatetime = (d: Date) => {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const AddChange = () => {
  const navigate = useNavigate();
  const [submitted, setSubmitted] = useState(false);
  const [mode, setMode] = useState<ChangeMode>('executed');

  const [form, setForm] = useState({
    system: '',
    environment: '',
    user: '',
    type: '' as ChangeType | '',
    description: '',
    executedAt: toLocalDatetime(new Date()),
    scheduledAt: '',
  });

  const [errors, setErrors] = useState<{
    system?: string;
    type?: string;
    description?: string;
    timestamp?: string;
  }>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [submittedSystem, setSubmittedSystem] = useState('');
  const [submittedTimestamp, setSubmittedTimestamp] = useState('');

  const envRef = useRef<HTMLInputElement>(null);
  const userRef = useRef<HTMLInputElement>(null);
  const descRef = useRef<HTMLTextAreaElement>(null);

  const set = (k: keyof typeof form, v: string) => {
    setForm(f => ({ ...f, [k]: v }));
    if (k === 'executedAt' || k === 'scheduledAt') {
      setErrors(e => ({ ...e, timestamp: '' }));
    } else if (errors[k as keyof typeof errors]) {
      setErrors(e => ({ ...e, [k]: '' }));
    }
  };

  const validate = () => {
    const e: typeof errors = {};
    if (!form.system.trim()) e.system = 'System is required';
    if (!form.type) e.type = 'Change type is required';
    if (!form.description.trim()) e.description = 'Description is required';
    if (mode === 'scheduled') {
      if (!form.scheduledAt) {
        e.timestamp = 'Scheduled date is required';
      } else if (new Date(form.scheduledAt) <= new Date()) {
        e.timestamp = 'Scheduled date must be in the future';
      }
    }
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const timestamp = mode === 'executed'
        ? (form.executedAt ? new Date(form.executedAt).toISOString() : undefined)
        : new Date(form.scheduledAt).toISOString();

      await api.post('/api/changes', {
        system: form.system.trim(),
        environment: form.environment.trim() || null,
        user: form.user.trim() || null,
        type: form.type,
        description: form.description.trim(),
        status: mode,
        timestamp,
      });
      setSubmittedSystem(form.system.trim());
      setSubmittedTimestamp(timestamp ?? '');
      setSubmitted(true);
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Failed to register change');
    } finally {
      setSubmitting(false);
    }
  };

  const resetForm = () => {
    setForm({ system: '', environment: '', user: '', type: '', description: '', executedAt: toLocalDatetime(new Date()), scheduledAt: '' });
    setMode('executed');
    setSubmitted(false);
    setErrors({});
    setSubmitError(null);
  };

  if (submitted) {
    if (mode === 'scheduled') {
      return (
        <Layout>
          <div className="max-w-lg mx-auto text-center py-20">
            <div className="w-14 h-14 rounded-full bg-blue-500/10 border border-blue-500/20 flex items-center justify-center mx-auto mb-4">
              <CalendarClock className="w-7 h-7 text-blue-500" />
            </div>
            <h2 className="text-lg font-semibold text-foreground mb-2">Change Scheduled</h2>
            <p className="text-sm text-muted-foreground mb-1">
              The planned change for <span className="font-mono text-foreground">{submittedSystem}</span> has been registered.
            </p>
            {submittedTimestamp && (
              <p className="text-sm text-muted-foreground mb-6">
                Scheduled for <span className="font-semibold text-foreground">{format(parseISO(submittedTimestamp), 'PPpp')}</span>
              </p>
            )}
            <div className="flex gap-3 justify-center">
              <Button variant="outline" onClick={resetForm}>Schedule Another</Button>
              <Button onClick={() => navigate('/calendar')}>View Calendar</Button>
            </div>
          </div>
        </Layout>
      );
    }

    return (
      <Layout>
        <div className="max-w-lg mx-auto text-center py-20">
          <div className="w-14 h-14 rounded-full bg-deploy-bg border border-deploy-border flex items-center justify-center mx-auto mb-4">
            <CheckCircle className="w-7 h-7 text-deploy" />
          </div>
          <h2 className="text-lg font-semibold text-foreground mb-2">Change Registered</h2>
          <p className="text-sm text-muted-foreground mb-6">
            The change for <span className="font-mono text-foreground">{submittedSystem}</span> has been logged successfully.
          </p>
          <div className="flex gap-3 justify-center">
            <Button variant="outline" onClick={resetForm}>Register Another</Button>
            <Button onClick={() => navigate('/')}>View Change Log</Button>
          </div>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-2xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <div className="flex items-center gap-2 mb-1">
            <Plus className="w-4 h-4 text-primary" />
            <h1 className="text-lg font-semibold text-foreground">Register Change</h1>
          </div>
          <p className="text-sm text-muted-foreground">
            Log an infrastructure, deployment, or configuration change. Use <kbd className="px-1 py-0.5 rounded bg-secondary text-xs font-mono">Tab</kbd> to navigate between fields.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Change Type — top, critical */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Change Type <span className="text-destructive">*</span>
            </Label>
            <div className="grid grid-cols-3 gap-2">
              {TYPES.map(({ value, label, description, icon: Icon, badge }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => set('type', value)}
                  className={cn(
                    'flex flex-col gap-1 p-3 rounded-lg border text-left transition-all',
                    form.type === value
                      ? 'border-primary bg-primary/5 ring-1 ring-primary'
                      : 'border-border bg-card hover:border-border/60 hover:bg-accent/40'
                  )}
                >
                  <div className="flex items-center gap-1.5">
                    <span className={cn('text-xs font-medium px-1.5 py-0.5 rounded', badge)}>{label}</span>
                  </div>
                  <p className="text-xs text-muted-foreground leading-snug">{description}</p>
                </button>
              ))}
            </div>
            {errors.type && <p className="text-xs text-destructive">{errors.type}</p>}
          </div>

          {/* System + Environment row */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="system" className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
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
              <Label htmlFor="environment" className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                Environment
              </Label>
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
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              User
            </Label>
            <Input
              value={form.user}
              onChange={e => set('user', e.target.value)}
              placeholder="e.g. alice.martin"
              className="bg-card border-border text-sm"
              ref={userRef}
            />
          </div>

          {/* When — mode toggle + conditional datetime */}
          <div className="space-y-2">
            <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              When
            </Label>

            {/* Mode toggle */}
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

            {/* Datetime input */}
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">
                {mode === 'executed' ? 'When did it happen?' : 'Schedule for'}
                {mode === 'scheduled' && <span className="text-destructive ml-1">*</span>}
              </span>
              <Input
                type="datetime-local"
                value={mode === 'executed' ? form.executedAt : form.scheduledAt}
                onChange={e => set(mode === 'executed' ? 'executedAt' : 'scheduledAt', e.target.value)}
                min={mode === 'scheduled' ? toLocalDatetime(new Date()) : undefined}
                className="bg-card border-border text-sm w-64"
              />
              {mode === 'scheduled' && (
                <p className="text-xs text-muted-foreground">Must be a future date and time.</p>
              )}
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
              placeholder="Describe what changed, why, and any impact..."
              className="bg-card border-border text-sm resize-none min-h-[100px] font-sans"
              rows={4}
            />
            {errors.description && <p className="text-xs text-destructive">{errors.description}</p>}
          </div>

          {/* Actions */}
          <div className="flex items-center justify-between pt-2 border-t border-border">
            <div>
              {submitError && <p className="text-xs text-destructive">{submitError}</p>}
            </div>
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => navigate('/')}>
                Cancel
              </Button>
              <Button type="submit" size="sm" className="gap-1.5" disabled={submitting}>
                {mode === 'scheduled'
                  ? <><Calendar className="w-3.5 h-3.5" />{submitting ? 'Scheduling...' : 'Schedule Change'}</>
                  : <><Plus className="w-3.5 h-3.5" />{submitting ? 'Registering...' : 'Register Change'}</>
                }
              </Button>
            </div>
          </div>
        </form>
      </div>
    </Layout>
  );
};

export default AddChange;
