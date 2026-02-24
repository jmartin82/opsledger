import { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useNavigate, Link } from 'react-router-dom';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Zap, Eye, EyeOff, AlertCircle } from 'lucide-react';

type Mode = 'login' | 'register';

const Login = () => {
  const { login, register, ssoConfig } = useAuth();
  const navigate = useNavigate();

  const [mode, setMode] = useState<Mode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [showPwd, setShowPwd] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [registrationAllowed, setRegistrationAllowed] = useState(false);

  useEffect(() => {
    api.get('/api/auth/registration-status')
      .then(data => setRegistrationAllowed(data.allowed))
      .catch(() => setRegistrationAllowed(false));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    if (!email || !password) { setError('Email and password are required.'); return; }
    if (mode === 'register' && !name.trim()) { setError('Full name is required.'); return; }

    setLoading(true);
    const result = mode === 'login'
      ? await login(email, password)
      : await register(email, password, name);
    setLoading(false);

    if (result.success) {
      navigate('/');
    } else {
      setError(result.error ?? 'Something went wrong.');
    }
  };

  return (
    <div className="min-h-screen bg-background flex items-center justify-center px-4">
      {/* Subtle grid background */}
      <div className="absolute inset-0 grid-bg opacity-40 pointer-events-none" />

      <div className="relative w-full max-w-sm">
        {/* Logo */}
        <div className="flex flex-col items-center mb-8 gap-3">
          <div className="w-10 h-10 rounded-lg bg-primary flex items-center justify-center glow-amber">
            <Zap className="w-5 h-5 text-primary-foreground" strokeWidth={2.5} />
          </div>
          <div className="text-center">
            <h1 className="text-lg font-semibold text-foreground tracking-tight">OpsLedger</h1>
            <p className="text-xs text-muted-foreground">Change Management</p>
          </div>
        </div>

        {/* Card */}
        <div className="bg-card border border-border rounded-xl p-6 shadow-md">
          {/* SSO banner */}
          {ssoConfig.enabled && (
            <div className="mb-5">
              <Button className="w-full gap-2" type="button">
                <Zap className="w-4 h-4" />
                Sign in with SSO
              </Button>
              <div className="relative my-4">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-border" />
                </div>
                <div className="relative flex justify-center">
                  <span className="bg-card px-2 text-xs text-muted-foreground">break-glass admin only</span>
                </div>
              </div>
            </div>
          )}

          <h2 className="text-sm font-semibold text-foreground mb-4">
            {mode === 'login' ? 'Sign in to your account' : 'Create your account'}
          </h2>

          <form onSubmit={handleSubmit} className="space-y-4">
            {mode === 'register' && (
              <div className="space-y-1.5">
                <Label htmlFor="name" className="text-xs text-muted-foreground">Full name</Label>
                <Input
                  id="name"
                  value={name}
                  onChange={e => setName(e.target.value)}
                  placeholder="Alice Martin"
                  className="bg-background border-border text-sm"
                  autoComplete="name"
                />
              </div>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="email" className="text-xs text-muted-foreground">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder="you@company.com"
                className="bg-background border-border text-sm"
                autoComplete="email"
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="password" className="text-xs text-muted-foreground">Password</Label>
              <div className="relative">
                <Input
                  id="password"
                  type={showPwd ? 'text' : 'password'}
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  placeholder="••••••••"
                  className="bg-background border-border text-sm pr-10"
                  autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                />
                <button
                  type="button"
                  onClick={() => setShowPwd(v => !v)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                >
                  {showPwd ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            {error && (
              <div className="flex items-start gap-2 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
                <AlertCircle className="w-4 h-4 text-destructive shrink-0 mt-0.5" />
                <p className="text-xs text-destructive">{error}</p>
              </div>
            )}

            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
            </Button>
          </form>

          <p className="text-xs text-muted-foreground text-center mt-4">
            {mode === 'login' ? (
              registrationAllowed ? (
                <>No account?{' '}
                  <button onClick={() => { setMode('register'); setError(''); }} className="text-primary hover:underline">Create one</button>
                </>
              ) : null
            ) : (
              <>Already have an account?{' '}
                <button onClick={() => { setMode('login'); setError(''); }} className="text-primary hover:underline">Sign in</button>
              </>
            )}
          </p>

          {mode === 'register' && registrationAllowed && (
            <p className="text-xs text-muted-foreground text-center mt-2 p-2 rounded bg-muted/40 border border-border">
              The first user to register is automatically assigned the <span className="text-primary font-medium">Admin</span> role.
            </p>
          )}
        </div>

        <p className="text-center text-xs text-muted-foreground mt-6">
          OpsLedger v1.0
        </p>
      </div>
    </div>
  );
};

export default Login;
