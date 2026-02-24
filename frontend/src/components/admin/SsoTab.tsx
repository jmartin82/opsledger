import { useState } from 'react';
import { useAuth, SsoConfig } from '@/contexts/AuthContext';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Copy, CheckCircle, AlertCircle, Loader2, ChevronRight, Construction } from 'lucide-react';
import { cn } from '@/lib/utils';

const REDIRECT_URL = `${window.location.origin}/auth/callback`;

const PROVIDERS = [
  { value: 'azure', label: 'Azure AD / Entra ID', issuerHint: 'https://login.microsoftonline.com/{tenant}/v2.0' },
  { value: 'okta', label: 'Okta', issuerHint: 'https://{your-domain}.okta.com' },
  { value: 'google', label: 'Google Workspace', issuerHint: 'https://accounts.google.com' },
  { value: 'custom', label: 'Custom OIDC Provider', issuerHint: 'https://your-idp.com' },
];

const Field = ({ label, children, hint }: { label: string; hint?: string; children: React.ReactNode }) => (
  <div className="space-y-1.5">
    <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{label}</Label>
    {children}
    {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
  </div>
);

const CopyButton = ({ value }: { value: string }) => {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <button onClick={copy} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors ml-2">
      <Copy className="w-3 h-3" />
      {copied ? 'Copied!' : 'Copy'}
    </button>
  );
};

const SsoTab = () => {
  const { ssoConfig, saveSsoConfig } = useAuth();
  const [form, setForm] = useState<SsoConfig>(ssoConfig);
  const [saved, setSaved] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null);

  const set = <K extends keyof SsoConfig>(k: K, v: SsoConfig[K]) => {
    setForm(f => ({ ...f, [k]: v }));
    setSaved(false);
    setTestResult(null);
  };

  const selectedProvider = PROVIDERS.find(p => p.value === form.provider);

  const handleSave = () => {
    saveSsoConfig(form);
    setSaved(true);
    setTimeout(() => setSaved(false), 3000);
  };

  const handleTest = async () => {
    if (!form.issuerUrl || !form.clientId) {
      setTestResult({ ok: false, message: 'Issuer URL and Client ID are required to test the connection.' });
      return;
    }
    setTesting(true);
    setTestResult(null);
    await new Promise(r => setTimeout(r, 1500));
    // Prototype: simulate discovery check
    const isValid = form.issuerUrl.startsWith('https://');
    setTesting(false);
    setTestResult(
      isValid
        ? { ok: true, message: 'Discovery metadata resolved successfully. OIDC configuration appears valid.' }
        : { ok: false, message: 'Could not reach discovery endpoint. Ensure the Issuer URL is correct and accessible.' }
    );
  };

  return (
    <div className="relative">
      {/* Coming Soon Overlay */}
      <div className="absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-background/60 backdrop-blur-[2px]">
        <div className="flex flex-col items-center gap-2 text-center">
          <Construction className="w-8 h-8 text-muted-foreground" />
          <p className="text-sm font-medium text-foreground">Coming Soon</p>
          <p className="text-xs text-muted-foreground max-w-[260px]">
            SSO authentication is currently under development and will be available in a future release.
          </p>
        </div>
      </div>

      <div className="space-y-6 pointer-events-none select-none opacity-50">
      {/* SSO Toggle */}
      <div className="flex items-center justify-between p-4 rounded-lg border border-border bg-card">
        <div>
          <p className="text-sm font-medium text-foreground">Enable SSO Authentication</p>
          <p className="text-xs text-muted-foreground mt-0.5">
            When enabled, local login is disabled. Only the configured IdP is accepted.
          </p>
        </div>
        <Switch
          checked={form.enabled}
          onCheckedChange={v => set('enabled', v)}
        />
      </div>

      {form.enabled && (
        <div className="p-3 rounded-lg bg-config-bg border border-config-border">
          <div className="flex items-start gap-2">
            <AlertCircle className="w-4 h-4 text-config shrink-0 mt-0.5" />
            <p className="text-xs text-muted-foreground">
              <span className="text-foreground font-medium">Local login will be disabled</span> for all users once you save. 
              Ensure your OIDC configuration is tested and working before saving.
            </p>
          </div>
        </div>
      )}

      {/* Provider */}
      <Field label="Identity Provider">
        <Select value={form.provider} onValueChange={v => set('provider', v as SsoConfig['provider'])}>
          <SelectTrigger className="bg-background border-border text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PROVIDERS.map(p => (
              <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      {/* OIDC Inputs */}
      <Field label="Issuer URL" hint={`e.g. ${selectedProvider?.issuerHint ?? 'https://your-idp.com'}`}>
        <Input
          value={form.issuerUrl}
          onChange={e => set('issuerUrl', e.target.value)}
          placeholder={selectedProvider?.issuerHint}
          className="bg-background border-border text-sm font-mono"
        />
      </Field>

      <div className="grid grid-cols-2 gap-3">
        <Field label="Client ID">
          <Input
            value={form.clientId}
            onChange={e => set('clientId', e.target.value)}
            placeholder="your-client-id"
            className="bg-background border-border text-sm font-mono"
          />
        </Field>
        <Field label="Client Secret">
          <Input
            type="password"
            value={form.clientSecret}
            onChange={e => set('clientSecret', e.target.value)}
            placeholder="••••••••••••"
            className="bg-background border-border text-sm font-mono"
          />
        </Field>
      </div>

      <Field label="Redirect URL (read-only)" hint="Add this URL to your IdP's list of allowed redirect URIs">
        <div className="flex items-center px-3 py-2 rounded-md border border-border bg-muted/30">
          <span className="font-mono text-xs text-foreground flex-1">{REDIRECT_URL}</span>
          <CopyButton value={REDIRECT_URL} />
        </div>
      </Field>

      <Field label="Scopes">
        <Input
          value={form.scopes}
          onChange={e => set('scopes', e.target.value)}
          className="bg-background border-border text-sm font-mono"
        />
      </Field>

      {/* Role Mapping */}
      <div className="border border-border rounded-lg overflow-hidden">
        <div className="px-4 py-3 border-b border-border bg-secondary/20">
          <p className="text-sm font-medium text-foreground">Role Mapping</p>
          <p className="text-xs text-muted-foreground mt-0.5">
            Map IdP group claims to OpsLedger roles. The claim <code className="font-mono bg-secondary px-1 rounded">{form.roleMappingClaim}</code> will be checked.
          </p>
        </div>
        <div className="p-4 space-y-3">
          <Field label="Group claim name">
            <Input
              value={form.roleMappingClaim}
              onChange={e => set('roleMappingClaim', e.target.value)}
              placeholder="groups"
              className="bg-background border-border text-sm font-mono"
            />
          </Field>
          {[
            { label: 'Admin groups', key: 'adminGroups' as const, color: 'text-primary' },
            { label: 'Editor groups', key: 'editorGroups' as const, color: 'text-infra' },
            { label: 'Viewer groups', key: 'viewerGroups' as const, color: 'text-deploy' },
          ].map(({ label, key, color }) => (
            <div key={key} className="grid grid-cols-3 gap-3 items-center">
              <Label className={cn('text-xs font-medium', color)}>{label}</Label>
              <div className="col-span-2">
                <Input
                  value={form[key]}
                  onChange={e => set(key, e.target.value)}
                  placeholder="comma-separated group names"
                  className="bg-background border-border text-sm font-mono"
                />
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Test result */}
      {testResult && (
        <div className={cn(
          'flex items-start gap-2 p-3 rounded-lg border text-xs',
          testResult.ok
            ? 'bg-deploy-bg border-deploy-border text-deploy'
            : 'bg-destructive/10 border-destructive/20 text-destructive'
        )}>
          {testResult.ok ? <CheckCircle className="w-4 h-4 shrink-0 mt-0.5" /> : <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />}
          <p>{testResult.message}</p>
        </div>
      )}

      {/* Actions */}
      <div className="flex items-center gap-3 pt-2 border-t border-border">
        <Button variant="outline" size="sm" onClick={handleTest} disabled={testing} className="gap-1.5">
          {testing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <ChevronRight className="w-3.5 h-3.5" />}
          Test Connection
        </Button>
        <Button size="sm" onClick={handleSave} className="gap-1.5">
          {saved ? <CheckCircle className="w-3.5 h-3.5" /> : null}
          {saved ? 'Saved!' : 'Save Configuration'}
        </Button>
      </div>
      </div>
    </div>
  );
};

export default SsoTab;
