# Phase 6: UI Dashboard (Weeks 21-24)

Build a production-grade web dashboard for managing MCP servers, agents, monitoring, and marketplace browsing. The stack uses Next.js 15 with the App Router, shadcn/ui for components, Connect-ES for gRPC-Web communication, and NextAuth for authentication against Keycloak.

---

## Step 1: Next.js 15 Setup

Bootstrap the dashboard application with all foundational tooling: App Router, shadcn/ui component library, gRPC-Web client generation, and Keycloak-backed authentication.

### Files

```
ui/package.json
ui/next.config.ts
ui/tsconfig.json
ui/tailwind.config.ts
ui/app/layout.tsx
ui/app/page.tsx
ui/app/api/auth/[...nextauth]/route.ts
ui/lib/auth.ts
ui/lib/grpc-client.ts
ui/lib/types.ts
ui/buf.gen.yaml
ui/components/ui/           (shadcn/ui components)
ui/components/layout/
ui/components/layout/sidebar.tsx
ui/components/layout/header.tsx
```

### Key Code

**ui/package.json** (key dependencies)

```json
{
  "name": "mcp-gateway-dashboard",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev --turbopack",
    "build": "next build",
    "start": "next start",
    "lint": "next lint",
    "test": "vitest",
    "test:e2e": "playwright test",
    "generate": "buf generate"
  },
  "dependencies": {
    "next": "^15.0.0",
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "next-auth": "^5.0.0",
    "@connectrpc/connect": "^2.0.0",
    "@connectrpc/connect-web": "^2.0.0",
    "@bufbuild/protobuf": "^2.0.0",
    "recharts": "^2.12.0",
    "@radix-ui/react-dialog": "^1.1.0",
    "@radix-ui/react-dropdown-menu": "^2.1.0",
    "@radix-ui/react-select": "^2.1.0",
    "@radix-ui/react-slider": "^1.2.0",
    "@radix-ui/react-checkbox": "^1.1.0",
    "@radix-ui/react-tabs": "^1.1.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.3.0",
    "lucide-react": "^0.400.0",
    "zod": "^3.23.0",
    "react-hook-form": "^7.52.0",
    "@hookform/resolvers": "^3.9.0",
    "@monaco-editor/react": "^4.6.0"
  },
  "devDependencies": {
    "typescript": "^5.5.0",
    "@types/react": "^19.0.0",
    "tailwindcss": "^4.0.0",
    "@tailwindcss/postcss": "^4.0.0",
    "vitest": "^2.0.0",
    "@testing-library/react": "^16.0.0",
    "@playwright/test": "^1.45.0",
    "@bufbuild/protoc-gen-es": "^2.0.0",
    "@connectrpc/protoc-gen-connect-es": "^2.0.0",
    "eslint-config-next": "^15.0.0"
  }
}
```

**ui/lib/auth.ts** (NextAuth with Keycloak)

```typescript
import NextAuth from "next-auth";
import KeycloakProvider from "next-auth/providers/keycloak";
import type { NextAuthConfig } from "next-auth";

export const authConfig: NextAuthConfig = {
  providers: [
    KeycloakProvider({
      clientId: process.env.KEYCLOAK_CLIENT_ID!,
      clientSecret: process.env.KEYCLOAK_CLIENT_SECRET!,
      issuer: process.env.KEYCLOAK_ISSUER!,
    }),
  ],
  callbacks: {
    async jwt({ token, account }) {
      if (account) {
        token.accessToken = account.access_token;
        token.refreshToken = account.refresh_token;
        token.expiresAt = account.expires_at;
        token.roles = parseRealmRoles(account.access_token);
      }
      return token;
    },
    async session({ session, token }) {
      session.accessToken = token.accessToken as string;
      session.roles = token.roles as string[];
      return session;
    },
  },
  pages: {
    signIn: "/auth/signin",
    error: "/auth/error",
  },
};

function parseRealmRoles(accessToken?: string): string[] {
  if (!accessToken) return [];
  try {
    const payload = JSON.parse(
      Buffer.from(accessToken.split(".")[1], "base64").toString()
    );
    return payload.realm_access?.roles ?? [];
  } catch {
    return [];
  }
}

export const { handlers, auth, signIn, signOut } = NextAuth(authConfig);
```

**ui/lib/grpc-client.ts** (Connect-ES client factory)

```typescript
import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient } from "@connectrpc/connect";
import type { ServiceType } from "@bufbuild/protobuf";

const transport = createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  useBinaryFormat: true,
});

export function createGrpcClient<T extends ServiceType>(service: T) {
  return createClient(service, transport);
}

// Auth-aware transport for server components
export function createAuthTransport(accessToken: string) {
  return createConnectTransport({
    baseUrl: process.env.API_URL ?? "http://mcp-gateway-api:8080",
    useBinaryFormat: true,
    interceptors: [
      (next) => async (req) => {
        req.header.set("Authorization", `Bearer ${accessToken}`);
        return next(req);
      },
    ],
  });
}
```

**ui/buf.gen.yaml**

```yaml
version: v2
plugins:
  - remote: buf.build/bufbuild/es
    out: gen
    opt: target=ts
  - remote: buf.build/connectrpc/es
    out: gen
    opt: target=ts
inputs:
  - directory: ../api/proto
```

**ui/app/layout.tsx**

```tsx
import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { SessionProvider } from "next-auth/react";
import { Sidebar } from "@/components/layout/sidebar";
import { Header } from "@/components/layout/header";
import { auth } from "@/lib/auth";
import "./globals.css";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "MCP Gateway Dashboard",
  description: "Manage MCP servers, agents, and marketplace",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();

  return (
    <html lang="en" className="dark">
      <body className={inter.className}>
        <SessionProvider session={session}>
          <div className="flex h-screen">
            <Sidebar />
            <div className="flex flex-1 flex-col overflow-hidden">
              <Header />
              <main className="flex-1 overflow-y-auto p-6">
                {children}
              </main>
            </div>
          </div>
        </SessionProvider>
      </body>
    </html>
  );
}
```

### Quality Gate

- `npm run build` succeeds with zero TypeScript errors.
- `buf generate` produces TypeScript types for all protobuf services.
- NextAuth redirects unauthenticated users to Keycloak login.
- Authenticated users see the dashboard layout with sidebar and header.

### Testing Command

```bash
cd ui

# Install dependencies
npm install

# Generate protobuf types
npm run generate

# Type check
npx tsc --noEmit

# Build
npm run build

# Dev server
npm run dev

# Run unit tests
npm run test
```

### Pitfalls

- **NextAuth v5 breaking changes:** NextAuth v5 (Auth.js) has a different API from v4. The `auth()` function replaces `getServerSession()`. Check the v5 migration guide if examples from the internet use v4 patterns.
- **Connect-ES binary format:** The `useBinaryFormat: true` option sends protobuf binary instead of JSON. This is more efficient but requires the server to accept `application/proto`. If the server only speaks JSON, set `useBinaryFormat: false`.
- **Turbopack compatibility:** Some dependencies may not work with Turbopack. If `next dev --turbopack` fails, fall back to webpack by removing the `--turbopack` flag.
- **Keycloak CORS:** The Keycloak issuer URL must allow CORS from the dashboard origin. Configure the Keycloak client's "Web Origins" setting.

### Progress Marker

- [ ] Next.js project builds successfully
- [ ] shadcn/ui components installed and themed
- [ ] Protobuf TypeScript types generated
- [ ] NextAuth Keycloak flow works end-to-end
- [ ] Layout with sidebar and header renders

---

## Step 2: Server Management View

Build the CRUD interface for MCP servers: a sortable list table, a detail page with status and events, and a create/edit form with an embedded YAML editor.

### Files

```
ui/app/servers/page.tsx
ui/app/servers/[name]/page.tsx
ui/app/servers/new/page.tsx
ui/components/servers/server-table.tsx
ui/components/servers/server-detail.tsx
ui/components/servers/server-form.tsx
ui/components/servers/yaml-editor.tsx
ui/components/servers/server-status-badge.tsx
ui/hooks/use-servers.ts
ui/hooks/use-sse.ts
```

### Key Code

**ui/hooks/use-servers.ts** (data fetching with SWR pattern)

```typescript
"use client";

import { useState, useEffect, useCallback } from "react";
import { createGrpcClient } from "@/lib/grpc-client";
import { GatewayService } from "@/gen/gateway/v1/gateway_pb";
import type { MCPServer } from "@/gen/gateway/v1/gateway_pb";

const client = createGrpcClient(GatewayService);

export function useServers() {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchServers = useCallback(async () => {
    try {
      setLoading(true);
      const response = await client.listMCPServers({});
      setServers(response.servers);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchServers();
  }, [fetchServers]);

  return { servers, loading, error, refetch: fetchServers };
}

export function useServer(name: string, namespace: string) {
  const [server, setServer] = useState<MCPServer | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    async function fetch() {
      try {
        const response = await client.getMCPServer({ name, namespace });
        setServer(response.server);
      } catch (err) {
        setError(err instanceof Error ? err : new Error(String(err)));
      } finally {
        setLoading(false);
      }
    }
    fetch();
  }, [name, namespace]);

  return { server, loading, error };
}
```

**ui/hooks/use-sse.ts** (real-time SSE updates)

```typescript
"use client";

import { useEffect, useRef, useCallback } from "react";

interface SSEOptions {
  url: string;
  onMessage: (event: MessageEvent) => void;
  onError?: (event: Event) => void;
  enabled?: boolean;
}

export function useSSE({ url, onMessage, onError, enabled = true }: SSEOptions) {
  const eventSourceRef = useRef<EventSource | null>(null);

  const connect = useCallback(() => {
    if (!enabled) return;

    const eventSource = new EventSource(url);
    eventSourceRef.current = eventSource;

    eventSource.onmessage = onMessage;
    eventSource.onerror = (event) => {
      onError?.(event);
      // Reconnect after 5 seconds
      setTimeout(() => {
        eventSource.close();
        connect();
      }, 5000);
    };

    return () => {
      eventSource.close();
    };
  }, [url, onMessage, onError, enabled]);

  useEffect(() => {
    const cleanup = connect();
    return cleanup;
  }, [connect]);
}
```

**ui/components/servers/server-table.tsx**

```tsx
"use client";

import { useState } from "react";
import Link from "next/link";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ServerStatusBadge } from "./server-status-badge";
import { useServers } from "@/hooks/use-servers";
import { useSSE } from "@/hooks/use-sse";
import { ArrowUpDown, Plus, Search } from "lucide-react";
import type { MCPServer } from "@/gen/gateway/v1/gateway_pb";

type SortField = "name" | "namespace" | "transport" | "status";
type SortDir = "asc" | "desc";

export function ServerTable() {
  const { servers, loading, refetch } = useServers();
  const [search, setSearch] = useState("");
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  // Real-time updates via SSE
  useSSE({
    url: `${process.env.NEXT_PUBLIC_API_URL}/events/servers`,
    onMessage: () => refetch(),
  });

  const filtered = servers
    .filter(
      (s) =>
        s.name.toLowerCase().includes(search.toLowerCase()) ||
        s.namespace.toLowerCase().includes(search.toLowerCase())
    )
    .sort((a, b) => {
      const aVal = a[sortField] ?? "";
      const bVal = b[sortField] ?? "";
      const cmp = String(aVal).localeCompare(String(bVal));
      return sortDir === "asc" ? cmp : -cmp;
    });

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir(sortDir === "asc" ? "desc" : "asc");
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  };

  if (loading) return <div className="animate-pulse">Loading servers...</div>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="relative w-72">
          <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search servers..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>
        <Link href="/servers/new">
          <Button>
            <Plus className="mr-2 h-4 w-4" />
            New Server
          </Button>
        </Link>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead onClick={() => toggleSort("name")} className="cursor-pointer">
              Name <ArrowUpDown className="ml-1 inline h-3 w-3" />
            </TableHead>
            <TableHead onClick={() => toggleSort("namespace")} className="cursor-pointer">
              Namespace <ArrowUpDown className="ml-1 inline h-3 w-3" />
            </TableHead>
            <TableHead onClick={() => toggleSort("transport")} className="cursor-pointer">
              Transport <ArrowUpDown className="ml-1 inline h-3 w-3" />
            </TableHead>
            <TableHead onClick={() => toggleSort("status")} className="cursor-pointer">
              Status <ArrowUpDown className="ml-1 inline h-3 w-3" />
            </TableHead>
            <TableHead>Tools</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {filtered.map((server) => (
            <TableRow key={`${server.namespace}/${server.name}`}>
              <TableCell>
                <Link
                  href={`/servers/${server.namespace}/${server.name}`}
                  className="font-medium text-primary hover:underline"
                >
                  {server.name}
                </Link>
              </TableCell>
              <TableCell>{server.namespace}</TableCell>
              <TableCell>
                <code className="rounded bg-muted px-2 py-1 text-xs">
                  {server.transport}
                </code>
              </TableCell>
              <TableCell>
                <ServerStatusBadge status={server.status} />
              </TableCell>
              <TableCell>{server.toolCount ?? 0}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
```

**ui/components/servers/yaml-editor.tsx**

```tsx
"use client";

import { useCallback } from "react";
import Editor from "@monaco-editor/react";

interface YAMLEditorProps {
  value: string;
  onChange: (value: string) => void;
  readOnly?: boolean;
  height?: string;
}

export function YAMLEditor({
  value,
  onChange,
  readOnly = false,
  height = "400px",
}: YAMLEditorProps) {
  const handleEditorChange = useCallback(
    (val: string | undefined) => {
      if (val !== undefined) {
        onChange(val);
      }
    },
    [onChange]
  );

  return (
    <div className="rounded-md border">
      <Editor
        height={height}
        language="yaml"
        theme="vs-dark"
        value={value}
        onChange={handleEditorChange}
        options={{
          readOnly,
          minimap: { enabled: false },
          fontSize: 13,
          lineNumbers: "on",
          scrollBeyondLastLine: false,
          wordWrap: "on",
          tabSize: 2,
          formatOnPaste: true,
        }}
      />
    </div>
  );
}
```

### Quality Gate

- Server list loads and displays all MCPServer CRDs in the cluster.
- Search filters the table in real time.
- Sorting works on all columns.
- SSE connection updates the table when a server is created or deleted externally.
- YAML editor validates syntax and highlights errors.

### Testing Command

```bash
cd ui

# Unit tests for hooks and components
npm run test -- --run

# Specific component tests
npx vitest run components/servers/

# Storybook visual test (if configured)
npm run storybook -- --ci
```

### Pitfalls

- **SSE reconnection storms:** If the API server is down, the 5-second reconnect in `useSSE` can create a reconnection storm. Implement exponential backoff with a cap at 60 seconds.
- **Monaco editor bundle size:** The `@monaco-editor/react` package loads the full Monaco editor (~2MB). Use dynamic imports with `next/dynamic` and `ssr: false` to prevent server-side rendering failures and reduce initial bundle size.
- **gRPC-Web CORS:** The API server must return appropriate CORS headers for the Connect protocol. Missing `Connect-Protocol-Version` in CORS `Access-Control-Allow-Headers` causes silent failures.

### Progress Marker

- [ ] Server list table renders with real data
- [ ] Search and sort functional
- [ ] Server detail page shows status and events
- [ ] Create form with YAML editor works
- [ ] SSE real-time updates functional

---

## Step 3: Agent Management View

Build the agent management interface with a permission matrix (checkbox grid mapping agents to servers/tools), rate limit sliders, and quota progress bars.

### Files

```
ui/app/agents/page.tsx
ui/app/agents/[name]/page.tsx
ui/app/agents/new/page.tsx
ui/components/agents/agent-table.tsx
ui/components/agents/permission-matrix.tsx
ui/components/agents/rate-limit-slider.tsx
ui/components/agents/quota-progress.tsx
ui/components/agents/agent-form.tsx
ui/hooks/use-agents.ts
```

### Key Code

**ui/components/agents/permission-matrix.tsx**

```tsx
"use client";

import { useState, useCallback } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { AgentPermission, MCPServer } from "@/gen/gateway/v1/gateway_pb";

interface PermissionMatrixProps {
  servers: MCPServer[];
  permissions: AgentPermission[];
  onChange: (permissions: AgentPermission[]) => void;
  readOnly?: boolean;
}

export function PermissionMatrix({
  servers,
  permissions,
  onChange,
  readOnly = false,
}: PermissionMatrixProps) {
  // Build a map: serverName -> Set<toolName>
  const permissionMap = new Map<string, Set<string>>();
  for (const perm of permissions) {
    if (!permissionMap.has(perm.serverName)) {
      permissionMap.set(perm.serverName, new Set());
    }
    for (const tool of perm.allowedTools) {
      permissionMap.get(perm.serverName)!.add(tool);
    }
  }

  // Collect all unique tools across all servers
  const allTools = new Map<string, string[]>();
  for (const server of servers) {
    allTools.set(server.name, server.tools?.map((t) => t.name) ?? []);
  }

  const togglePermission = useCallback(
    (serverName: string, toolName: string) => {
      if (readOnly) return;
      const newPerms = [...permissions];
      const serverPerm = newPerms.find((p) => p.serverName === serverName);

      if (serverPerm) {
        const toolIdx = serverPerm.allowedTools.indexOf(toolName);
        if (toolIdx >= 0) {
          serverPerm.allowedTools.splice(toolIdx, 1);
        } else {
          serverPerm.allowedTools.push(toolName);
        }
      } else {
        newPerms.push({
          serverName,
          allowedTools: [toolName],
        } as AgentPermission);
      }
      onChange(newPerms);
    },
    [permissions, onChange, readOnly]
  );

  const toggleAllForServer = useCallback(
    (serverName: string) => {
      if (readOnly) return;
      const tools = allTools.get(serverName) ?? [];
      const currentPerms = permissionMap.get(serverName);
      const allChecked = currentPerms?.size === tools.length;

      const newPerms = permissions.filter((p) => p.serverName !== serverName);
      if (!allChecked) {
        newPerms.push({
          serverName,
          allowedTools: [...tools],
        } as AgentPermission);
      }
      onChange(newPerms);
    },
    [permissions, onChange, permissionMap, allTools, readOnly]
  );

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="sticky left-0 bg-background">
              Server / Tool
            </TableHead>
            {servers.map((server) => (
              <TableHead key={server.name} className="text-center">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      onClick={() => toggleAllForServer(server.name)}
                      className="font-medium hover:text-primary"
                      disabled={readOnly}
                    >
                      {server.name}
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>Click to toggle all tools</TooltipContent>
                </Tooltip>
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {/* Flatten all tools across servers into rows */}
          {Array.from(allTools.entries()).flatMap(([serverName, tools]) =>
            tools.map((tool) => (
              <TableRow key={`${serverName}-${tool}`}>
                <TableCell className="sticky left-0 bg-background font-mono text-sm">
                  {serverName}/{tool}
                </TableCell>
                {servers.map((server) => (
                  <TableCell key={server.name} className="text-center">
                    {server.name === serverName ? (
                      <Checkbox
                        checked={
                          permissionMap.get(serverName)?.has(tool) ?? false
                        }
                        onCheckedChange={() =>
                          togglePermission(serverName, tool)
                        }
                        disabled={readOnly}
                      />
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
```

**ui/components/agents/rate-limit-slider.tsx**

```tsx
"use client";

import { Slider } from "@/components/ui/slider";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";

interface RateLimitSliderProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  unit: string;
  onChange: (value: number) => void;
  disabled?: boolean;
}

export function RateLimitSlider({
  label,
  value,
  min,
  max,
  step,
  unit,
  onChange,
  disabled = false,
}: RateLimitSliderProps) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <Label>{label}</Label>
        <div className="flex items-center gap-2">
          <Input
            type="number"
            value={value}
            onChange={(e) => onChange(Number(e.target.value))}
            className="w-20 text-right"
            min={min}
            max={max}
            step={step}
            disabled={disabled}
          />
          <span className="text-sm text-muted-foreground">{unit}</span>
        </div>
      </div>
      <Slider
        value={[value]}
        onValueChange={([v]) => onChange(v)}
        min={min}
        max={max}
        step={step}
        disabled={disabled}
      />
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>
          {min} {unit}
        </span>
        <span>
          {max} {unit}
        </span>
      </div>
    </div>
  );
}
```

**ui/components/agents/quota-progress.tsx**

```tsx
"use client";

import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";

interface QuotaProgressProps {
  label: string;
  used: number;
  total: number;
  unit: string;
}

export function QuotaProgress({ label, used, total, unit }: QuotaProgressProps) {
  const percentage = total > 0 ? (used / total) * 100 : 0;
  const remaining = total - used;

  const colorClass =
    percentage > 90
      ? "bg-destructive"
      : percentage > 70
        ? "bg-yellow-500"
        : "bg-primary";

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="font-medium">{label}</span>
        <span className="text-muted-foreground">
          {used.toLocaleString()} / {total.toLocaleString()} {unit}
        </span>
      </div>
      <Progress
        value={percentage}
        className={cn("h-2", colorClass)}
      />
      <p className="text-xs text-muted-foreground">
        {remaining.toLocaleString()} {unit} remaining ({(100 - percentage).toFixed(1)}%)
      </p>
    </div>
  );
}
```

### Quality Gate

- Permission matrix correctly reflects current agent-to-server-tool mappings.
- Toggling a checkbox immediately updates the permission list.
- "Toggle all" on a server header selects/deselects all tools for that server.
- Rate limit sliders and numeric inputs stay synchronized.
- Quota progress bars show correct colors at thresholds (green < 70%, yellow 70-90%, red > 90%).

### Testing Command

```bash
cd ui

# Component unit tests
npx vitest run components/agents/

# Test permission matrix interactions
npx vitest run components/agents/permission-matrix.test.tsx

# Visual regression (if Storybook configured)
npx chromatic --project-token=$CHROMATIC_TOKEN
```

### Pitfalls

- **Permission matrix performance with many servers/tools:** A 50-server x 20-tool matrix renders 1000 checkboxes. Use `React.memo` on individual cells and virtualize the table with `@tanstack/react-virtual` if the grid exceeds 500 cells.
- **Slider precision:** Floating-point arithmetic in slider values can produce values like `10.000000001`. Round values to the step precision before sending to the API.
- **Optimistic updates vs. server state:** The permission matrix should use optimistic updates for responsiveness but reconcile with the server response. Show a toast notification on save failure and revert to the previous state.

### Progress Marker

- [ ] Agent list table with search and sort
- [ ] Permission matrix renders with real server/tool data
- [ ] Checkbox toggling updates permissions correctly
- [ ] Rate limit sliders functional with numeric input sync
- [ ] Quota progress bars render with correct color thresholds

---

## Step 4: Monitoring View

Embed Grafana dashboards for deep-dive monitoring alongside lightweight Recharts stat cards for at-a-glance metrics.

### Files

```
ui/app/monitoring/page.tsx
ui/components/monitoring/grafana-embed.tsx
ui/components/monitoring/stat-card.tsx
ui/components/monitoring/metric-chart.tsx
ui/hooks/use-metrics.ts
```

### Key Code

**ui/components/monitoring/grafana-embed.tsx**

```tsx
"use client";

import { useState } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const DASHBOARDS = [
  { id: "mcp-platform-overview", label: "Platform Overview" },
  { id: "mcp-per-server", label: "Per Server" },
  { id: "mcp-per-agent", label: "Per Agent" },
] as const;

interface GrafanaEmbedProps {
  baseUrl: string;
  orgId?: number;
  theme?: "light" | "dark";
}

export function GrafanaEmbed({
  baseUrl,
  orgId = 1,
  theme = "dark",
}: GrafanaEmbedProps) {
  const [selectedDashboard, setSelectedDashboard] = useState(DASHBOARDS[0].id);
  const [timeRange, setTimeRange] = useState("1h");

  const iframeUrl = `${baseUrl}/d/${selectedDashboard}?orgId=${orgId}&theme=${theme}&kiosk&from=now-${timeRange}&to=now&refresh=30s`;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <Select value={selectedDashboard} onValueChange={setSelectedDashboard}>
          <SelectTrigger className="w-48">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {DASHBOARDS.map((d) => (
              <SelectItem key={d.id} value={d.id}>
                {d.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={timeRange} onValueChange={setTimeRange}>
          <SelectTrigger className="w-32">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="15m">Last 15m</SelectItem>
            <SelectItem value="1h">Last 1h</SelectItem>
            <SelectItem value="6h">Last 6h</SelectItem>
            <SelectItem value="24h">Last 24h</SelectItem>
            <SelectItem value="7d">Last 7d</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="aspect-video w-full overflow-hidden rounded-lg border">
        <iframe
          src={iframeUrl}
          className="h-full w-full"
          title={`Grafana - ${DASHBOARDS.find((d) => d.id === selectedDashboard)?.label}`}
          sandbox="allow-scripts allow-same-origin"
        />
      </div>
    </div>
  );
}
```

**ui/components/monitoring/stat-card.tsx**

```tsx
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { TrendingDown, TrendingUp, Minus } from "lucide-react";

interface StatCardProps {
  title: string;
  value: string | number;
  unit?: string;
  trend?: {
    direction: "up" | "down" | "flat";
    value: string;
    label: string;
  };
  status?: "healthy" | "warning" | "critical";
}

export function StatCard({ title, value, unit, trend, status }: StatCardProps) {
  const statusColors = {
    healthy: "border-l-green-500",
    warning: "border-l-yellow-500",
    critical: "border-l-red-500",
  };

  const TrendIcon =
    trend?.direction === "up"
      ? TrendingUp
      : trend?.direction === "down"
        ? TrendingDown
        : Minus;

  return (
    <Card className={cn("border-l-4", status && statusColors[status])}>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-baseline gap-2">
          <span className="text-3xl font-bold">{value}</span>
          {unit && (
            <span className="text-sm text-muted-foreground">{unit}</span>
          )}
        </div>
        {trend && (
          <div className="mt-2 flex items-center gap-1 text-xs text-muted-foreground">
            <TrendIcon className="h-3 w-3" />
            <span>{trend.value}</span>
            <span>{trend.label}</span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
```

**ui/hooks/use-metrics.ts**

```typescript
"use client";

import { useState, useEffect } from "react";

interface MetricPoint {
  timestamp: number;
  value: number;
}

interface DashboardMetrics {
  activeServers: number;
  totalRequests: number;
  errorRate: number;
  p99Latency: number;
  requestsTimeline: MetricPoint[];
  errorsTimeline: MetricPoint[];
}

export function useMetrics(refreshInterval = 30000) {
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchMetrics() {
      try {
        const promUrl = process.env.NEXT_PUBLIC_PROMETHEUS_URL;
        if (!promUrl) return;

        const [serversRes, requestsRes, errorRateRes, latencyRes] =
          await Promise.all([
            queryPrometheus(promUrl, "mcpgateway_servers_active"),
            queryPrometheus(promUrl, "sum(increase(envoy_http_downstream_rq_total[1h]))"),
            queryPrometheus(promUrl, "sum(rate(envoy_http_downstream_rq_xx{envoy_response_code_class='5'}[5m])) / sum(rate(envoy_http_downstream_rq_total[5m]))"),
            queryPrometheus(promUrl, "histogram_quantile(0.99, sum(rate(envoy_http_downstream_rq_time_bucket[5m])) by (le))"),
          ]);

        setMetrics({
          activeServers: parseScalar(serversRes),
          totalRequests: parseScalar(requestsRes),
          errorRate: parseScalar(errorRateRes),
          p99Latency: parseScalar(latencyRes),
          requestsTimeline: [],
          errorsTimeline: [],
        });
      } catch (err) {
        console.error("Failed to fetch metrics:", err);
      } finally {
        setLoading(false);
      }
    }

    fetchMetrics();
    const interval = setInterval(fetchMetrics, refreshInterval);
    return () => clearInterval(interval);
  }, [refreshInterval]);

  return { metrics, loading };
}

async function queryPrometheus(baseUrl: string, query: string) {
  const resp = await fetch(
    `${baseUrl}/api/v1/query?query=${encodeURIComponent(query)}`
  );
  return resp.json();
}

function parseScalar(response: any): number {
  const result = response?.data?.result?.[0]?.value?.[1];
  return result ? parseFloat(result) : 0;
}
```

### Quality Gate

- Grafana iframe loads the selected dashboard without authentication issues.
- Dashboard selector switches between the three pre-built dashboards.
- Time range selector updates the Grafana URL parameters.
- Stat cards show live metrics from Prometheus.
- Stat cards display correct status colors (green/yellow/red).

### Testing Command

```bash
cd ui

# Unit tests
npx vitest run components/monitoring/
npx vitest run hooks/use-metrics.test.ts

# Visual test of stat cards
npx vitest run components/monitoring/stat-card.test.tsx
```

### Pitfalls

- **Grafana iframe authentication:** Grafana must be configured to allow embedding (`allow_embedding = true` in `grafana.ini`) and either use anonymous access for the dashboards or pass an auth token via URL parameter. The `kiosk` parameter hides the Grafana chrome.
- **CORS for Prometheus queries:** Direct browser-to-Prometheus queries require CORS. Consider routing through the Next.js API route (`/api/metrics`) to keep Prometheus internal.
- **Stat card refresh rate:** Polling Prometheus every 30 seconds with four queries is fine for a single user. For multiple concurrent dashboard users, add a server-side caching layer.

### Progress Marker

- [ ] Grafana embed renders with dashboard selector
- [ ] Time range selector works
- [ ] Stat cards display live metrics
- [ ] Status color thresholds correct
- [ ] Metrics hook handles errors gracefully

---

## Step 5: Marketplace Browser

Build the marketplace browsing and deployment UI: a card grid with category filters, a deploy dialog that collects required secrets, and a progress indicator for the deployment flow.

### Files

```
ui/app/marketplace/page.tsx
ui/components/marketplace/entry-card.tsx
ui/components/marketplace/entry-grid.tsx
ui/components/marketplace/category-filter.tsx
ui/components/marketplace/deploy-dialog.tsx
ui/components/marketplace/deploy-progress.tsx
ui/components/marketplace/search-bar.tsx
ui/hooks/use-marketplace.ts
```

### Key Code

**ui/components/marketplace/entry-card.tsx**

```tsx
"use client";

import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Shield, Download, ExternalLink } from "lucide-react";
import type { CatalogEntry } from "@/gen/marketplace/v1/marketplace_pb";

interface EntryCardProps {
  entry: CatalogEntry;
  onDeploy: (entry: CatalogEntry) => void;
}

const categoryColors: Record<string, string> = {
  "ai-ml": "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  data: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  "developer-tools": "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  communication: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  security: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  monitoring: "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
  infrastructure: "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200",
  custom: "bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200",
};

export function EntryCard({ entry, onDeploy }: EntryCardProps) {
  return (
    <Card className="flex flex-col transition-shadow hover:shadow-lg">
      <CardHeader className="space-y-2">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            {entry.icon ? (
              <img
                src={entry.icon}
                alt=""
                className="h-10 w-10 rounded"
              />
            ) : (
              <div className="flex h-10 w-10 items-center justify-center rounded bg-muted text-lg font-bold">
                {entry.displayName[0]}
              </div>
            )}
            <div>
              <h3 className="font-semibold leading-tight">{entry.displayName}</h3>
              <p className="text-xs text-muted-foreground">{entry.vendor}</p>
            </div>
          </div>
          {entry.verified && (
            <Shield className="h-5 w-5 text-green-500" aria-label="Verified" />
          )}
        </div>
        <Badge variant="secondary" className={categoryColors[entry.category] ?? ""}>
          {entry.category}
        </Badge>
      </CardHeader>

      <CardContent className="flex-1">
        <p className="line-clamp-3 text-sm text-muted-foreground">
          {entry.description}
        </p>
        {entry.tags.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-1">
            {entry.tags.map((tag) => (
              <Badge key={tag} variant="outline" className="text-xs">
                {tag}
              </Badge>
            ))}
          </div>
        )}
      </CardContent>

      <CardFooter className="flex items-center justify-between border-t pt-4">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Download className="h-3 w-3" />
          <span>{entry.installCount.toLocaleString()} installs</span>
        </div>
        <div className="flex gap-2">
          {entry.documentation && (
            <Button variant="ghost" size="sm" asChild>
              <a href={entry.documentation} target="_blank" rel="noopener noreferrer">
                <ExternalLink className="h-4 w-4" />
              </a>
            </Button>
          )}
          <Button size="sm" onClick={() => onDeploy(entry)}>
            Deploy
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
}
```

**ui/components/marketplace/deploy-dialog.tsx**

```tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DeployProgress } from "./deploy-progress";
import { createGrpcClient } from "@/lib/grpc-client";
import { MarketplaceService } from "@/gen/marketplace/v1/marketplace_pb";
import type { CatalogEntry } from "@/gen/marketplace/v1/marketplace_pb";

const client = createGrpcClient(MarketplaceService);

interface DeployDialogProps {
  entry: CatalogEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  namespaces: string[];
}

type DeployState = "form" | "deploying" | "success" | "error";

export function DeployDialog({
  entry,
  open,
  onOpenChange,
  namespaces,
}: DeployDialogProps) {
  const [state, setState] = useState<DeployState>("form");
  const [deployError, setDeployError] = useState<string>("");
  const [progress, setProgress] = useState(0);

  // Dynamically build schema based on required secrets
  const secretKeys = entry?.source ? [] : []; // populated from installTemplate
  const schema = z.object({
    instanceName: z
      .string()
      .min(3)
      .max(63)
      .regex(/^[a-z0-9][a-z0-9-]*[a-z0-9]$/),
    namespace: z.string().min(1),
    ...Object.fromEntries(
      (secretKeys ?? []).map((key: string) => [key, z.string().min(1)])
    ),
  });

  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      instanceName: entry?.name ?? "",
      namespace: "default",
    },
  });

  async function onSubmit(values: Record<string, string>) {
    if (!entry) return;
    setState("deploying");
    setProgress(25);

    try {
      setProgress(50);
      const secretValues: Record<string, string> = {};
      for (const key of secretKeys ?? []) {
        secretValues[key] = values[key];
      }

      const response = await client.deployCatalogEntry({
        entryName: entry.name,
        targetNamespace: values.namespace,
        instanceName: values.instanceName,
        secretValues,
      });

      setProgress(100);
      setState("success");
    } catch (err) {
      setState("error");
      setDeployError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Deploy {entry?.displayName}</DialogTitle>
          <DialogDescription>
            Configure and deploy this MCP server to your cluster.
          </DialogDescription>
        </DialogHeader>

        {state === "form" && (
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="instanceName">Instance Name</Label>
              <Input
                id="instanceName"
                placeholder="my-server-instance"
                {...form.register("instanceName")}
              />
              {form.formState.errors.instanceName && (
                <p className="text-xs text-destructive">
                  {form.formState.errors.instanceName.message as string}
                </p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="namespace">Namespace</Label>
              <Select
                onValueChange={(v) => form.setValue("namespace", v)}
                defaultValue="default"
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {namespaces.map((ns) => (
                    <SelectItem key={ns} value={ns}>
                      {ns}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Dynamic secret fields would be rendered here */}

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit">Deploy</Button>
            </DialogFooter>
          </form>
        )}

        {(state === "deploying" || state === "success" || state === "error") && (
          <DeployProgress
            progress={progress}
            state={state}
            error={deployError}
            onClose={() => {
              setState("form");
              setProgress(0);
              onOpenChange(false);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
```

**ui/components/marketplace/deploy-progress.tsx**

```tsx
"use client";

import { Progress } from "@/components/ui/progress";
import { Button } from "@/components/ui/button";
import { CheckCircle2, XCircle, Loader2 } from "lucide-react";

interface DeployProgressProps {
  progress: number;
  state: "deploying" | "success" | "error";
  error?: string;
  onClose: () => void;
}

export function DeployProgress({
  progress,
  state,
  error,
  onClose,
}: DeployProgressProps) {
  return (
    <div className="space-y-6 py-4">
      <Progress value={progress} className="h-2" />

      <div className="flex flex-col items-center gap-3 text-center">
        {state === "deploying" && (
          <>
            <Loader2 className="h-10 w-10 animate-spin text-primary" />
            <p className="text-sm text-muted-foreground">
              Creating resources...
            </p>
          </>
        )}

        {state === "success" && (
          <>
            <CheckCircle2 className="h-10 w-10 text-green-500" />
            <p className="font-medium">Deployment successful</p>
            <p className="text-sm text-muted-foreground">
              Your MCP server is being started. Check the Servers page for status.
            </p>
            <Button onClick={onClose}>Done</Button>
          </>
        )}

        {state === "error" && (
          <>
            <XCircle className="h-10 w-10 text-destructive" />
            <p className="font-medium">Deployment failed</p>
            <p className="text-sm text-destructive">{error}</p>
            <Button variant="outline" onClick={onClose}>
              Close
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
```

### Quality Gate

- Marketplace page loads and displays all catalog entries as cards.
- Category filter narrows the displayed cards.
- Search filters by name, vendor, and tags.
- Deploy dialog collects instance name, namespace, and required secret values.
- Deploy progress indicator shows creating/success/error states.
- Verified badge appears only on verified entries.

### Testing Command

```bash
cd ui

# Component tests
npx vitest run components/marketplace/

# Deploy dialog integration test
npx vitest run components/marketplace/deploy-dialog.test.tsx

# E2E deploy flow
npx playwright test marketplace.spec.ts
```

### Pitfalls

- **Secret values in browser memory:** The deploy dialog temporarily holds secret values in React state. Clear them immediately after the gRPC call completes. Never log or persist secret values client-side.
- **Dynamic form validation:** The Zod schema must be rebuilt when the selected entry changes (different entries require different secrets). Use `useMemo` keyed on `entry.name` to avoid stale validation rules.
- **Card grid responsive layout:** Use CSS Grid with `grid-template-columns: repeat(auto-fill, minmax(300px, 1fr))` for responsive card sizing. Fixed column counts break on narrow viewports.

### Progress Marker

- [ ] Entry cards render with all metadata
- [ ] Category filter works
- [ ] Search filters across name/vendor/tags
- [ ] Deploy dialog opens and collects inputs
- [ ] Progress indicator shows all three states
- [ ] Verified badge displays correctly

---

## Step 6: Playwright E2E Tests

End-to-end tests covering authentication, marketplace deploy flow, and agent creation using Playwright.

### Files

```
ui/e2e/auth.setup.ts
ui/e2e/marketplace.spec.ts
ui/e2e/agents.spec.ts
ui/e2e/servers.spec.ts
ui/playwright.config.ts
```

### Key Code

**ui/playwright.config.ts**

```typescript
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? "github" : "html",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "setup",
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["setup"],
    },
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
      dependencies: ["setup"],
    },
  ],
  webServer: process.env.CI
    ? undefined
    : {
        command: "npm run dev",
        url: "http://localhost:3000",
        reuseExistingServer: true,
      },
});
```

**ui/e2e/auth.setup.ts** (persistent auth state)

```typescript
import { test as setup, expect } from "@playwright/test";
import path from "path";

const authFile = path.join(__dirname, "../.auth/user.json");

setup("authenticate", async ({ page }) => {
  // Navigate to app, which redirects to Keycloak
  await page.goto("/");

  // Wait for Keycloak login page
  await page.waitForURL(/.*keycloak.*/);

  // Fill in credentials
  await page.fill("#username", process.env.E2E_USERNAME ?? "admin");
  await page.fill("#password", process.env.E2E_PASSWORD ?? "admin");
  await page.click("#kc-login");

  // Wait for redirect back to app
  await page.waitForURL(/.*localhost:3000.*/);

  // Verify authenticated
  await expect(page.locator("[data-testid=user-menu]")).toBeVisible();

  // Save auth state
  await page.context().storageState({ path: authFile });
});
```

**ui/e2e/marketplace.spec.ts**

```typescript
import { test, expect } from "@playwright/test";

test.use({ storageState: ".auth/user.json" });

test.describe("Marketplace", () => {
  test("browse and filter catalog entries", async ({ page }) => {
    await page.goto("/marketplace");

    // Verify cards are displayed
    const cards = page.locator("[data-testid=entry-card]");
    await expect(cards.first()).toBeVisible();
    const initialCount = await cards.count();
    expect(initialCount).toBeGreaterThan(0);

    // Filter by category
    await page.click("[data-testid=category-filter-developer-tools]");
    const filteredCount = await cards.count();
    expect(filteredCount).toBeLessThanOrEqual(initialCount);

    // Verify all visible cards have the correct category
    for (let i = 0; i < filteredCount; i++) {
      const badge = cards.nth(i).locator("[data-testid=category-badge]");
      await expect(badge).toHaveText("developer-tools");
    }
  });

  test("search catalog entries", async ({ page }) => {
    await page.goto("/marketplace");

    await page.fill("[data-testid=marketplace-search]", "github");
    await page.waitForTimeout(300); // debounce

    const cards = page.locator("[data-testid=entry-card]");
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText("GitHub");
  });

  test("deploy marketplace entry", async ({ page }) => {
    await page.goto("/marketplace");

    // Find the Prometheus entry (no secrets required)
    await page.fill("[data-testid=marketplace-search]", "prometheus");
    await page.waitForTimeout(300);

    // Click Deploy
    await page.click("[data-testid=entry-card] >> button:has-text('Deploy')");

    // Fill deploy form
    await page.fill("[data-testid=instance-name-input]", "e2e-test-prometheus");
    await page.selectOption("[data-testid=namespace-select]", "default");

    // Submit
    await page.click("button:has-text('Deploy')");

    // Wait for success
    await expect(page.locator("text=Deployment successful")).toBeVisible({
      timeout: 30000,
    });

    // Close dialog
    await page.click("button:has-text('Done')");

    // Verify server appears on servers page
    await page.goto("/servers");
    await expect(page.locator("text=e2e-test-prometheus")).toBeVisible();
  });
});
```

**ui/e2e/agents.spec.ts**

```typescript
import { test, expect } from "@playwright/test";

test.use({ storageState: ".auth/user.json" });

test.describe("Agent Management", () => {
  test("create a new agent with permissions", async ({ page }) => {
    await page.goto("/agents/new");

    // Fill agent name
    await page.fill("[data-testid=agent-name-input]", "e2e-test-agent");

    // Set rate limits
    const rateLimitSlider = page.locator(
      "[data-testid=rate-limit-requests-per-minute]"
    );
    await rateLimitSlider.locator("input[type=number]").fill("100");

    // Toggle permissions in the matrix
    const firstCheckbox = page.locator(
      "[data-testid=permission-matrix] input[type=checkbox]"
    ).first();
    await firstCheckbox.check();

    // Submit
    await page.click("button:has-text('Create Agent')");

    // Verify redirect to agent list
    await page.waitForURL("/agents");
    await expect(page.locator("text=e2e-test-agent")).toBeVisible();
  });

  test("view agent quota progress", async ({ page }) => {
    await page.goto("/agents");

    // Click on an existing agent
    await page.click("text=e2e-test-agent");

    // Verify quota progress bars are visible
    const progressBars = page.locator("[data-testid=quota-progress]");
    await expect(progressBars.first()).toBeVisible();
  });
});
```

### Quality Gate

- All E2E tests pass against a running Next.js dev server with a real API backend.
- Auth state persists across tests (no re-login between tests).
- Marketplace deploy flow creates an actual MCPServer CR.
- Tests produce screenshots on failure for debugging.
- CI reports are generated in GitHub Actions format.

### Testing Command

```bash
cd ui

# Install Playwright browsers
npx playwright install

# Run all E2E tests
npx playwright test

# Run specific test file
npx playwright test marketplace.spec.ts

# Run with headed browser (debugging)
npx playwright test --headed

# Run with trace viewer
npx playwright test --trace on
npx playwright show-trace test-results/*/trace.zip
```

### Pitfalls

- **Keycloak test instance:** The auth setup requires a running Keycloak instance with a test user. Use a Docker Compose file (`docker-compose.e2e.yaml`) to start Keycloak with a pre-configured realm, client, and user.
- **Auth state file path:** The `.auth/user.json` file contains session tokens. Add it to `.gitignore` and clean it up in CI between runs to prevent stale auth issues.
- **Flaky marketplace deploy test:** The deploy test depends on the marketplace indexer and operator being operational. Use Playwright's `expect(...).toBeVisible({ timeout: 30000 })` with generous timeouts for async operations.
- **Parallel test isolation:** Tests that create resources (agents, servers) must use unique names (e.g., timestamp suffixes) to avoid conflicts when running in parallel.

### Progress Marker

- [ ] Auth setup authenticates against Keycloak
- [ ] Auth state persists across test files
- [ ] Marketplace browse/filter/search tests pass
- [ ] Marketplace deploy E2E test creates real resources
- [ ] Agent creation E2E test passes
- [ ] CI produces test reports with failure screenshots
