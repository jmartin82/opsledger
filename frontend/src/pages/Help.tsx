import Layout from '@/components/Layout';
import { BookOpen, Copy, Terminal, Zap } from 'lucide-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { cn } from '@/lib/utils';

const CodeBlock = ({ code, language = '' }: { code: string; language?: string }) => {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <div className="relative code-block rounded-lg overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-secondary/30">
        <span className="text-xs text-muted-foreground font-mono">{language}</span>
        <button onClick={copy} className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
          <Copy className="w-3 h-3" />
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
      <pre className="p-4 overflow-x-auto text-sm leading-relaxed font-mono whitespace-pre">
        <code>{code}</code>
      </pre>
    </div>
  );
};

const Section = ({ title, children }: { title: string; children: React.ReactNode }) => (
  <section className="space-y-4">
    <h2 className="text-base font-semibold text-foreground border-b border-border pb-2">{title}</h2>
    {children}
  </section>
);

const Help = () => {
  const [searchParams] = useSearchParams();
  const [tab, setTab] = useState<'rest' | 'mcp'>(searchParams.get('tab') === 'mcp' ? 'mcp' : 'rest');
  const baseUrl = 'https://opsledger.io';
  const apiKey = 'ol_live_xxxxxxxxxxxxxxxxxxxx';

  return (
    <Layout>
      <div className="max-w-3xl mx-auto space-y-8">
        {/* Header */}
        <div>
          <div className="flex items-center gap-2 mb-1">
            <BookOpen className="w-4 h-4 text-primary" />
            <h1 className="text-lg font-semibold text-foreground">API & MCP Documentation</h1>
          </div>
          <p className="text-sm text-muted-foreground">
            Integrate OpsLedger with your CI/CD pipelines, incident tools, and AI agents.
          </p>
        </div>

        {/* Tab switch */}
        <div className="flex gap-1 p-1 bg-secondary rounded-lg w-fit">
          {(['rest', 'mcp'] as const).map(t => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={cn(
                'px-4 py-1.5 text-sm font-medium rounded transition-colors',
                tab === t ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {t === 'rest' ? (
                <span className="flex items-center gap-1.5"><Terminal className="w-3.5 h-3.5" />REST API</span>
              ) : (
                <span className="flex items-center gap-1.5"><Zap className="w-3.5 h-3.5" />MCP Connector</span>
              )}
            </button>
          ))}
        </div>

        {tab === 'rest' ? (
          <div className="space-y-8">
            {/* Auth */}
            <Section title="Authentication">
              <p className="text-sm text-muted-foreground">
                All API requests require an <code className="font-mono text-xs bg-secondary px-1.5 py-0.5 rounded text-foreground">Authorization</code> header with your API key.
              </p>
              <CodeBlock language="http" code={`Authorization: Bearer ${apiKey}`} />
            </Section>

            {/* POST */}
            <Section title="POST /changes — Register a Change">
              <p className="text-sm text-muted-foreground">
                Register a new infrastructure, deployment, or configuration change.
              </p>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Request</p>
                <CodeBlock language="bash" code={`curl -X POST ${baseUrl}/changes \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "system": "api-gateway",
    "environment": "production",
    "user": "diana.kim",
    "type": "deployment",
    "description": "Deployed v2.14.3 — rate limiting fix for /auth/token",
    "timestamp": "2025-02-19T14:32:00Z"
  }'`} />
              </div>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Fields</p>
                <div className="bg-card border border-border rounded-lg overflow-hidden">
                  {[
                    { field: 'system', type: 'string', req: true, desc: 'Product or service affected' },
                    { field: 'type', type: '"infrastructure" | "deployment" | "configuration"', req: true, desc: 'Category of the change' },
                    { field: 'description', type: 'string', req: true, desc: 'Human-readable description of what changed' },
                    { field: 'environment', type: 'string', req: false, desc: 'e.g. production, staging, preprod' },
                    { field: 'user', type: 'string', req: false, desc: 'Who performed the change' },
                    { field: 'timestamp', type: 'ISO 8601 string', req: false, desc: 'Defaults to server time if omitted' },
                  ].map(({ field, type, req, desc }, i) => (
                    <div key={field} className={cn('flex gap-3 px-4 py-2.5 text-sm', i > 0 && 'border-t border-border')}>
                      <span className="font-mono text-infra w-28 shrink-0">{field}</span>
                      <span className="font-mono text-xs text-muted-foreground w-48 shrink-0 mt-0.5">{type}</span>
                      <span className={cn('text-xs font-medium w-12 shrink-0 mt-0.5', req ? 'text-destructive' : 'text-muted-foreground')}>
                        {req ? 'required' : 'optional'}
                      </span>
                      <span className="text-xs text-muted-foreground">{desc}</span>
                    </div>
                  ))}
                </div>
              </div>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Response (201 Created)</p>
                <CodeBlock language="json" code={`{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "system": "api-gateway",
  "environment": "production",
  "user": "diana.kim",
  "type": "deployment",
  "description": "Deployed v2.14.3 — rate limiting fix for /auth/token",
  "timestamp": "2025-02-19T14:32:00.000Z"
}`} />
              </div>
            </Section>

            {/* PUT */}
            <Section title="PUT /changes/:id — Update a Change">
              <p className="text-sm text-muted-foreground">
                Update an existing change record. Requires <code className="font-mono text-xs bg-secondary px-1.5 py-0.5 rounded text-foreground">changes:write</code> scope or editor/admin role.
              </p>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Request</p>
                <CodeBlock language="bash" code={`curl -X PUT ${baseUrl}/changes/550e8400-e29b-41d4-a716-446655440000 \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "system": "api-gateway",
    "environment": "production",
    "user": "diana.kim",
    "type": "deployment",
    "description": "Deployed v2.14.4 — rollback of rate limiting change",
    "timestamp": "2025-02-19T14:32:00Z"
  }'`} />
              </div>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Fields</p>
                <div className="bg-card border border-border rounded-lg overflow-hidden">
                  {[
                    { field: 'system', type: 'string', req: true, desc: 'Product or service affected' },
                    { field: 'type', type: '"infrastructure" | "deployment" | "configuration"', req: true, desc: 'Category of the change' },
                    { field: 'description', type: 'string', req: true, desc: 'Human-readable description of what changed' },
                    { field: 'environment', type: 'string', req: false, desc: 'e.g. production, staging, preprod' },
                    { field: 'user', type: 'string', req: false, desc: 'Who performed the change' },
                    { field: 'timestamp', type: 'ISO 8601 string', req: false, desc: 'Override the change timestamp' },
                  ].map(({ field, type, req, desc }, i) => (
                    <div key={field} className={cn('flex gap-3 px-4 py-2.5 text-sm', i > 0 && 'border-t border-border')}>
                      <span className="font-mono text-infra w-28 shrink-0">{field}</span>
                      <span className="font-mono text-xs text-muted-foreground w-48 shrink-0 mt-0.5">{type}</span>
                      <span className={cn('text-xs font-medium w-12 shrink-0 mt-0.5', req ? 'text-destructive' : 'text-muted-foreground')}>
                        {req ? 'required' : 'optional'}
                      </span>
                      <span className="text-xs text-muted-foreground">{desc}</span>
                    </div>
                  ))}
                </div>
              </div>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Response (200 OK)</p>
                <CodeBlock language="json" code={`{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "system": "api-gateway",
  "environment": "production",
  "user": "diana.kim",
  "type": "deployment",
  "description": "Deployed v2.14.4 — rollback of rate limiting change",
  "timestamp": "2025-02-19T14:32:00.000Z"
}`} />
              </div>
            </Section>

            {/* DELETE */}
            <Section title="DELETE /changes/:id — Delete a Change">
              <p className="text-sm text-muted-foreground">
                Permanently delete a change record. Requires <code className="font-mono text-xs bg-secondary px-1.5 py-0.5 rounded text-foreground">changes:write</code> scope or editor/admin role.
              </p>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Request</p>
                <CodeBlock language="bash" code={`curl -X DELETE ${baseUrl}/changes/550e8400-e29b-41d4-a716-446655440000 \\
  -H "Authorization: Bearer ${apiKey}"`} />
              </div>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Response (200 OK)</p>
                <CodeBlock language="json" code={`{
  "message": "Change deleted"
}`} />
              </div>
            </Section>

            {/* GET */}
            <Section title="GET /changes — Query the Change Log">
              <p className="text-sm text-muted-foreground">
                Retrieve changes with optional filters. Results sorted by timestamp descending.
              </p>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Example — Last 2 hours on production</p>
                <CodeBlock language="bash" code={`curl "${baseUrl}/changes?environment=production&timeRange=2h" \\
  -H "Authorization: Bearer ${apiKey}"`} />
              </div>
              <div className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Query Parameters</p>
                <div className="bg-card border border-border rounded-lg overflow-hidden">
                  {[
                    { param: 'system', desc: 'Filter by system name (exact match)' },
                    { param: 'environment', desc: 'Filter by environment (production, staging, ...)' },
                    { param: 'user', desc: 'Filter by user who made the change' },
                    { param: 'type', desc: 'infrastructure | deployment | configuration' },
                    { param: 'timeRange', desc: '30m, 1h, 2h, 6h, 24h, 7d — relative to now' },
                    { param: 'from', desc: 'ISO 8601 start timestamp (for custom range)' },
                    { param: 'to', desc: 'ISO 8601 end timestamp (for custom range)' },
                    { param: 'limit', desc: 'Max results to return (default: 100, max: 1000)' },
                  ].map(({ param, desc }, i) => (
                    <div key={param} className={cn('flex gap-4 px-4 py-2.5 text-sm', i > 0 && 'border-t border-border')}>
                      <span className="font-mono text-infra w-24 shrink-0">{param}</span>
                      <span className="text-xs text-muted-foreground">{desc}</span>
                    </div>
                  ))}
                </div>
              </div>
            </Section>
          </div>
        ) : (
          <div className="space-y-8">
            {/* MCP Overview */}
            <Section title="MCP Connector Overview">
              <p className="text-sm text-muted-foreground">
                Connect OpsLedger to AI agents (Claude, GPT, custom LLMs) via the Model Context Protocol. This allows AI systems to query and register changes autonomously during incident response.
              </p>
              <div className="grid grid-cols-2 gap-3">
                {[
                  { title: 'list_changes', desc: 'Query change log with filters' },
                  { title: 'register_change', desc: 'Log a completed infrastructure, deployment, or config change' },
                  { title: 'update_change', desc: 'Update an existing change record' },
                  { title: 'delete_change', desc: 'Delete a change record' },
                ].map(({ title, desc }) => (
                  <div key={title} className="p-3 rounded-lg bg-card border border-border">
                    <p className="font-mono text-xs text-primary mb-1">{title}</p>
                    <p className="text-xs text-muted-foreground">{desc}</p>
                  </div>
                ))}
              </div>
            </Section>

            {/* Connection config */}
            <Section title="Connection Configuration">
              <p className="text-sm text-muted-foreground">
                Add this to your MCP client configuration (Claude Desktop, Continue, etc.):
              </p>
              <CodeBlock language="json" code={`{
  "mcpServers": {
    "opsledger": {
      "url": "${baseUrl}/mcp",
      "transport": "http",
      "apiKey": "${apiKey}"
    }
  }
}`} />
            </Section>

            {/* Example tool call */}
            <Section title="Example Tool Call — list_changes">
              <CodeBlock language="json" code={`// Tool call from AI agent
{
  "tool": "list_changes",
  "parameters": {
    "environment": "production",
    "timeRange": "2h"
  }
}

// Response
{
  "changes": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "system": "api-gateway",
      "environment": "production",
      "type": "deployment",
      "description": "Deployed v2.14.3 — rate limiting fix",
      "timestamp": "2025-02-19T14:32:00.000Z"
    }
  ],
  "total": 1
}`} />
            </Section>

            {/* Example update */}
            <Section title="Example Tool Call — update_change">
              <CodeBlock language="json" code={`// Tool call from AI agent
{
  "tool": "update_change",
  "parameters": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "system": "api-gateway",
    "environment": "production",
    "type": "deployment",
    "description": "Deployed v2.14.4 — rollback of rate limiting change",
    "user": "diana.kim",
    "timestamp": "2025-02-19T14:32:00Z"
  }
}

// Response
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "system": "api-gateway",
  "environment": "production",
  "type": "deployment",
  "description": "Deployed v2.14.4 — rollback of rate limiting change",
  "user": "diana.kim",
  "timestamp": "2025-02-19T14:32:00.000Z"
}`} />
            </Section>

            {/* Example delete */}
            <Section title="Example Tool Call — delete_change">
              <CodeBlock language="json" code={`// Tool call from AI agent
{
  "tool": "delete_change",
  "parameters": {
    "id": "550e8400-e29b-41d4-a716-446655440000"
  }
}

// Response
{
  "message": "Change deleted"
}`} />
            </Section>

            {/* Example register */}
            <Section title="Example Tool Call — register_change">
              <CodeBlock language="json" code={`// Tool call from AI agent (e.g. during automated deployment)
{
  "tool": "register_change",
  "parameters": {
    "system": "billing-service",
    "environment": "production",
    "type": "deployment",
    "description": "Automated deployment of billing-service v3.1.0 via CI pipeline",
    "user": "ci-pipeline",
    "timestamp": "2025-02-19T15:00:00Z"
  }
}

// Response
{
  "id": "661e8400-e29b-41d4-a716-446655440001",
  "system": "billing-service",
  "environment": "production",
  "user": "ci-pipeline",
  "type": "deployment",
  "description": "Automated deployment of billing-service v3.1.0 via CI pipeline",
  "timestamp": "2025-02-19T15:00:00.000Z"
}`} />
            </Section>
          </div>
        )}
      </div>
    </Layout>
  );
};

export default Help;
