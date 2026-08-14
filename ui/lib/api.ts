// ---------------------------------------------------------------------------
// TypeScript interfaces matching the Go CRD types in api/v1alpha1/
// ---------------------------------------------------------------------------

// --- Shared / Kubernetes meta ---

export interface ObjectMeta {
  name: string
  namespace?: string
  creationTimestamp?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

export interface Condition {
  type: string
  status: string
  lastTransitionTime: string
  reason: string
  message: string
}

// --- MCPServer ---

export type MCPServerPhase =
  | 'Pending'
  | 'Deploying'
  | 'Running'
  | 'Updating'
  | 'Scaling'
  | 'Failed'
  | 'Terminating'

export type TransportType = 'streamable-http' | 'stdio'

export interface MCPServerHealthCheck {
  path: string
  periodSeconds: number
}

export interface MCPServerSource {
  image: string
  port: number
  healthCheck?: MCPServerHealthCheck
}

export interface MCPServerProtocol {
  transport: TransportType
  endpoint: string
}

export interface ScaleToZeroConfig {
  enabled: boolean
  idleTimeoutSeconds?: number
}

export interface MCPServerScaling {
  minReplicas?: number
  maxReplicas: number
  scaleToZero?: ScaleToZeroConfig
}

export interface SecretKeyRef {
  name: string
  key: string
}

export interface MCPServerSecret {
  envVar: string
  secretRef: SecretKeyRef
}

export interface DiscoveredCapabilities {
  tools?: string[]
  resources?: string[]
  prompts?: string[]
  lastDiscoveredAt?: string
  cacheTTLMs?: number
}

export interface MCPServerSpec {
  source: MCPServerSource
  protocol: MCPServerProtocol
  scaling?: MCPServerScaling
  secrets?: MCPServerSecret[]
  env?: { name: string; value: string }[]
  serviceAnnotations?: Record<string, string>
  podAnnotations?: Record<string, string>
}

export interface MCPServerStatus {
  phase: MCPServerPhase
  replicas: number
  readyReplicas: number
  discoveredCapabilities?: DiscoveredCapabilities
  conditions?: Condition[]
  deploymentName?: string
  serviceName?: string
  observedGeneration?: number
  message?: string
}

export interface MCPServer {
  metadata: ObjectMeta
  spec: MCPServerSpec
  status: MCPServerStatus
}

// --- MCPAgent ---

export type MCPAgentPhase =
  | 'Pending'
  | 'Registering'
  | 'Active'
  | 'Suspended'
  | 'Failed'

export interface MCPAgentIdentity {
  oidcClientId?: string
}

export interface ServerAccessEntry {
  serverRef: { name: string }
  policyRef?: { name: string }
}

export interface RateLimitEntry {
  requestsPerMinute: number
}

export interface ToolRateLimitEntry {
  tool: string
  requestsPerMinute: number
}

export interface AgentRateLimits {
  global?: RateLimitEntry
  perTool?: ToolRateLimitEntry[]
}

export interface AgentQuota {
  maxConcurrentConnections?: number
  maxMonthlyToolCalls?: number
}

export interface MCPAgentSpec {
  identity?: MCPAgentIdentity
  serverAccess: ServerAccessEntry[]
  rateLimits?: AgentRateLimits
  quota?: AgentQuota
}

export interface MCPAgentStatus {
  phase: MCPAgentPhase
  registeredAt?: string
  clientSecretRef?: { name: string }
  currentMonthToolCalls: number
  activeConnections: number
  conditions?: Condition[]
}

export interface MCPAgent {
  metadata: ObjectMeta
  spec: MCPAgentSpec
  status: MCPAgentStatus
}

// --- MCPPolicy ---

export type PolicyEffect = 'ALLOW' | 'DENY'
export type MCPPolicyPhase = 'Pending' | 'Synced' | 'Failed'

export interface PolicyPrincipals {
  roles?: string[]
  agentRefs?: { name: string }[]
}

export interface PolicyResources {
  serverRef?: { name: string }
  tools?: string[]
}

export interface PolicyRule {
  effect: PolicyEffect
  principals?: PolicyPrincipals
  actions: string[]
  resources?: PolicyResources
}

export interface MCPPolicySpec {
  rules: PolicyRule[]
}

export interface MCPPolicyStatus {
  cerbosPolicyId?: string
  syncedAt?: string
  phase: MCPPolicyPhase
  conditions?: Condition[]
}

export interface MCPPolicy {
  metadata: ObjectMeta
  spec: MCPPolicySpec
  status: MCPPolicyStatus
}

// --- MCPMarketplaceEntry ---

export type MarketplaceCategory =
  | 'developer-tools'
  | 'data'
  | 'communication'
  | 'productivity'
  | 'security'
  | 'infrastructure'
  | 'ai-ml'
  | 'custom'

export type ScanStatus = 'passed' | 'failed' | 'warning' | 'pending' | 'not-scanned'
export type MarketplaceEntryPhase = 'Active' | 'Deprecated' | 'Blocked' | 'PendingScan'

export interface MarketplaceSource {
  image: string
  signatureRef?: string
  digest?: string
}

export interface RequiredSecret {
  name: string
  description?: string
}

export interface DefaultPolicy {
  allowedTools?: string[]
  deniedTools?: string[]
}

export interface InstallTemplate {
  mcpServerSpec: MCPServerSpec
  requiredSecrets?: RequiredSecret[]
  defaultPolicy?: DefaultPolicy
}

export interface SecurityInfo {
  scanStatus: ScanStatus
  lastScannedAt?: string
  cveCount?: number
  criticalCveCount?: number
  sbomRef?: string
}

export interface MarketplaceEntrySpec {
  displayName: string
  vendor: string
  version: string
  description?: string
  category: MarketplaceCategory
  tags?: string[]
  homepage?: string
  documentationUrl?: string
  source: MarketplaceSource
  installTemplate: InstallTemplate
  security?: SecurityInfo
  verified?: boolean
  deprecated?: boolean
}

export interface MarketplaceEntryStatus {
  phase: MarketplaceEntryPhase
  installCount?: number
  lastInstalledAt?: string
  conditions?: Condition[]
}

export interface MarketplaceEntry {
  metadata: ObjectMeta
  spec: MarketplaceEntrySpec
  status: MarketplaceEntryStatus
}

// ---------------------------------------------------------------------------
// API client
// ---------------------------------------------------------------------------

const API_URL = process.env.NEXT_PUBLIC_API_URL || ''

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })

  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new Error(`API ${res.status}: ${body || res.statusText}`)
  }

  return res.json() as Promise<T>
}

// --- Servers ---

export function fetchServers(): Promise<MCPServer[]> {
  return apiFetch<MCPServer[]>('/api/v1/servers')
}

export function fetchServer(namespace: string, name: string): Promise<MCPServer> {
  return apiFetch<MCPServer>(`/api/v1/servers/${namespace}/${name}`)
}

// --- Agents ---

export function fetchAgents(): Promise<MCPAgent[]> {
  return apiFetch<MCPAgent[]>('/api/v1/agents')
}

export function fetchAgent(namespace: string, name: string): Promise<MCPAgent> {
  return apiFetch<MCPAgent>(`/api/v1/agents/${namespace}/${name}`)
}

// --- Marketplace ---

export function fetchMarketplace(): Promise<MarketplaceEntry[]> {
  return apiFetch<MarketplaceEntry[]>('/api/v1/marketplace')
}

export function deployFromCatalog(
  name: string,
  namespace: string,
  secrets: Record<string, string>
): Promise<{ serverName: string }> {
  return apiFetch<{ serverName: string }>('/api/v1/marketplace/deploy', {
    method: 'POST',
    body: JSON.stringify({ name, namespace, secrets }),
  })
}
