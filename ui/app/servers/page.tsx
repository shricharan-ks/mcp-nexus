import Link from 'next/link'
import PhaseBadge from '@/components/phase-badge'

const mockServers = [
  {
    name: 'github-mcp',
    phase: 'Running',
    readyReplicas: '2/2',
    image: 'ghcr.io/modelcontextprotocol/github:v1.2.0',
    transport: 'SSE',
    age: '3d 4h',
  },
  {
    name: 'slack-mcp',
    phase: 'Deploying',
    readyReplicas: '0/1',
    image: 'ghcr.io/modelcontextprotocol/slack:v0.9.1',
    transport: 'StreamableHTTP',
    age: '2m',
  },
  {
    name: 'echo-server',
    phase: 'Failed',
    readyReplicas: '0/1',
    image: 'ghcr.io/mcp-gateway/echo:latest',
    transport: 'SSE',
    age: '1d 12h',
  },
]

export default function ServersPage() {
  return (
    <div className="p-6">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">MCP Servers</h1>
        <Link
          href="/marketplace"
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700"
        >
          Deploy Server
        </Link>
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
                Ready Replicas
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                Image
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                Transport
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                Age
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {mockServers.map((server) => (
              <tr key={server.name} className="hover:bg-gray-50">
                <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">
                  {server.name}
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-sm">
                  <PhaseBadge phase={server.phase} />
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                  {server.readyReplicas}
                </td>
                <td className="whitespace-nowrap px-6 py-4 font-mono text-xs text-gray-500">
                  {server.image}
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                  {server.transport}
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                  {server.age}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
