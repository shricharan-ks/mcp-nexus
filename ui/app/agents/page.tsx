import PhaseBadge from '@/components/phase-badge'

const mockAgents = [
  {
    name: 'coding-assistant',
    phase: 'Active',
    clientId: 'agent-ca-7f3a',
    monthlyCalls: 4521,
    quota: 100000,
    activeConnections: 3,
  },
  {
    name: 'data-pipeline',
    phase: 'Active',
    clientId: 'agent-dp-9b1c',
    monthlyCalls: 89000,
    quota: 100000,
    activeConnections: 1,
  },
]

function quotaColor(pct: number): string {
  if (pct > 90) return 'bg-red-500'
  if (pct >= 75) return 'bg-yellow-500'
  return 'bg-green-500'
}

export default function AgentsPage() {
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
            {mockAgents.map((agent) => {
              const pct = Math.round((agent.monthlyCalls / agent.quota) * 100)
              return (
                <tr key={agent.name} className="hover:bg-gray-50">
                  <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">
                    {agent.name}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm">
                    <PhaseBadge phase={agent.phase} />
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 font-mono text-xs text-gray-500">
                    {agent.clientId}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                    {agent.monthlyCalls.toLocaleString()} / {agent.quota.toLocaleString()}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm">
                    <div className="flex items-center gap-2">
                      <div className="h-2 w-24 overflow-hidden rounded-full bg-gray-200">
                        <div
                          className={`h-full rounded-full ${quotaColor(pct)}`}
                          style={{ width: `${Math.min(pct, 100)}%` }}
                        />
                      </div>
                      <span className="text-xs text-gray-500">{pct}%</span>
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                    {agent.activeConnections}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
