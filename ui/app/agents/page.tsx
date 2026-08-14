'use client'

import { useState, useEffect } from 'react'
import PhaseBadge from '@/components/phase-badge'
import { fetchAgents, MCPAgent } from '@/lib/api'

function quotaColor(pct: number): string {
  if (pct > 90) return 'bg-red-500'
  if (pct >= 75) return 'bg-yellow-500'
  return 'bg-green-500'
}

export default function AgentsPage() {
  const [agents, setAgents] = useState<MCPAgent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchAgents()
      .then(setAgents)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="p-8 text-gray-500">Loading agents...</div>
  if (error) return <div className="p-8 text-red-500">Error: {error}</div>

  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">MCP Agents</h1>
        <a
          href="#"
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700"
        >
          Create Agent
        </a>
      </div>

      {agents.length === 0 ? (
        <div className="p-8 text-gray-500">No MCP agents found.</div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  Phase
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  Client ID
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  Monthly Calls
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  Quota %
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  Active Connections
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {agents.map((agent) => {
                const monthlyCalls = agent.status.currentMonthToolCalls
                const quota = agent.spec.quota?.maxMonthlyToolCalls || 0
                const pct = quota > 0 ? Math.round((monthlyCalls / quota) * 100) : 0
                return (
                  <tr key={agent.metadata.name} className="hover:bg-gray-50">
                    <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">
                      {agent.metadata.name}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm">
                      <PhaseBadge phase={agent.status.phase} />
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 font-mono text-xs text-gray-500">
                      {agent.spec.identity?.oidcClientId || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {monthlyCalls.toLocaleString()}{quota > 0 ? ` / ${quota.toLocaleString()}` : ''}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm">
                      {quota > 0 ? (
                        <div className="flex items-center gap-2">
                          <div className="h-2 w-24 overflow-hidden rounded-full bg-gray-200">
                            <div
                              className={`h-full rounded-full ${quotaColor(pct)}`}
                              style={{ width: `${Math.min(pct, 100)}%` }}
                            />
                          </div>
                          <span className="text-xs text-gray-500">{pct}%</span>
                        </div>
                      ) : (
                        <span className="text-xs text-gray-400">No quota</span>
                      )}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {agent.status.activeConnections}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
