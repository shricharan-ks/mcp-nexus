'use client'

import { useState, useEffect } from 'react'
import { fetchMarketplace, deployFromCatalog, MarketplaceEntry } from '@/lib/api'

const categories = ['All', 'developer-tools', 'data', 'communication', 'productivity', 'ai-ml', 'security', 'infrastructure', 'custom']

const categoryLabels: Record<string, string> = {
  'All': 'All',
  'developer-tools': 'Developer Tools',
  'data': 'Data',
  'communication': 'Communication',
  'productivity': 'Productivity',
  'ai-ml': 'AI/ML',
  'security': 'Security',
  'infrastructure': 'Infrastructure',
  'custom': 'Custom',
}

export default function MarketplacePage() {
  const [entries, setEntries] = useState<MarketplaceEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [activeCategory, setActiveCategory] = useState('All')
  const [deployTarget, setDeployTarget] = useState<MarketplaceEntry | null>(null)
  const [secrets, setSecrets] = useState({ apiKey: '', apiSecret: '' })
  const [deploying, setDeploying] = useState(false)
  const [deployError, setDeployError] = useState<string | null>(null)

  useEffect(() => {
    fetchMarketplace()
      .then(setEntries)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  const filtered = entries.filter((entry) => {
    const name = entry.spec.displayName || entry.metadata.name
    const desc = entry.spec.description || ''
    const matchesSearch =
      name.toLowerCase().includes(search.toLowerCase()) ||
      desc.toLowerCase().includes(search.toLowerCase())
    const matchesCategory =
      activeCategory === 'All' || entry.spec.category === activeCategory
    return matchesSearch && matchesCategory
  })

  if (loading) return <div className="p-8 text-gray-500">Loading marketplace...</div>
  if (error) return <div className="p-8 text-red-500">Error: {error}</div>

  return (
    <div className="p-6">
      <h1 className="mb-6 text-2xl font-bold text-gray-900">MCP Server Marketplace</h1>

      <div className="mb-4">
        <input
          type="text"
          placeholder="Search servers..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-full rounded-md border border-gray-300 px-4 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
      </div>

      <div className="mb-6 flex flex-wrap gap-2">
        {categories.map((cat) => (
          <button
            key={cat}
            onClick={() => setActiveCategory(cat)}
            className={`rounded-full px-3 py-1 text-sm font-medium transition-colors ${
              activeCategory === cat
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
            }`}
          >
            {categoryLabels[cat] || cat}
          </button>
        ))}
      </div>

      {filtered.length === 0 ? (
        <div className="p-8 text-gray-500">No marketplace entries found.</div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {filtered.map((entry) => {
            const name = entry.spec.displayName || entry.metadata.name
            const toolCount = entry.spec.installTemplate?.mcpServerSpec
              ? (entry.spec.source?.image ? 1 : 0)
              : 0
            return (
              <div
                key={entry.metadata.name}
                className="flex flex-col rounded-lg border border-gray-200 bg-white p-5 shadow-sm"
              >
                <div className="mb-2 flex items-start justify-between">
                  <div>
                    <h3 className="flex items-center gap-1.5 text-lg font-semibold text-gray-900">
                      {name}
                      {entry.spec.verified && (
                        <svg
                          className="h-4 w-4 text-blue-500"
                          fill="currentColor"
                          viewBox="0 0 20 20"
                        >
                          <path
                            fillRule="evenodd"
                            d="M6.267 3.455a3.066 3.066 0 001.745-.723 3.066 3.066 0 013.976 0 3.066 3.066 0 001.745.723 3.066 3.066 0 012.812 2.812c.051.643.304 1.254.723 1.745a3.066 3.066 0 010 3.976 3.066 3.066 0 00-.723 1.745 3.066 3.066 0 01-2.812 2.812 3.066 3.066 0 00-1.745.723 3.066 3.066 0 01-3.976 0 3.066 3.066 0 00-1.745-.723 3.066 3.066 0 01-2.812-2.812 3.066 3.066 0 00-.723-1.745 3.066 3.066 0 010-3.976 3.066 3.066 0 00.723-1.745 3.066 3.066 0 012.812-2.812zm7.44 5.252a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                            clipRule="evenodd"
                          />
                        </svg>
                      )}
                    </h3>
                    <p className="text-xs text-gray-500">{entry.spec.vendor}</p>
                  </div>
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                    {entry.spec.version}
                  </span>
                </div>

                <span className="mb-2 inline-flex w-fit rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
                  {categoryLabels[entry.spec.category] || entry.spec.category}
                </span>

                <p className="mb-4 line-clamp-2 flex-1 text-sm text-gray-600">
                  {entry.spec.description}
                </p>

                <div className="mb-4 flex items-center gap-4 text-xs text-gray-500">
                  {entry.status.installCount !== undefined && (
                    <span>{entry.status.installCount.toLocaleString()} installs</span>
                  )}
                  {entry.spec.security?.scanStatus && (
                    <span>Scan: {entry.spec.security.scanStatus}</span>
                  )}
                </div>

                <button
                  onClick={() => {
                    setDeployTarget(entry)
                    setSecrets({ apiKey: '', apiSecret: '' })
                    setDeployError(null)
                  }}
                  className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
                >
                  Deploy
                </button>
              </div>
            )
          })}
        </div>
      )}

      {deployTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-semibold text-gray-900">
              Deploy {deployTarget.spec.displayName || deployTarget.metadata.name}
            </h2>

            {deployError && (
              <div className="mb-4 rounded-md bg-red-50 p-3 text-sm text-red-700">{deployError}</div>
            )}

            <div className="mb-4 space-y-3">
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">API Key</label>
                <input
                  type="password"
                  value={secrets.apiKey}
                  onChange={(e) => setSecrets((s) => ({ ...s, apiKey: e.target.value }))}
                  placeholder="Enter API key"
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">
                  API Secret
                </label>
                <input
                  type="password"
                  value={secrets.apiSecret}
                  onChange={(e) => setSecrets((s) => ({ ...s, apiSecret: e.target.value }))}
                  placeholder="Enter API secret"
                  className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
              </div>
            </div>

            <div className="flex justify-end gap-3">
              <button
                onClick={() => setDeployTarget(null)}
                disabled={deploying}
                className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={async () => {
                  setDeploying(true)
                  setDeployError(null)
                  try {
                    await deployFromCatalog(
                      deployTarget.metadata.name,
                      deployTarget.metadata.namespace || 'default',
                      secrets
                    )
                    setDeployTarget(null)
                    // Refresh marketplace data
                    fetchMarketplace().then(setEntries).catch(() => {})
                  } catch (e: unknown) {
                    setDeployError(e instanceof Error ? e.message : 'Deploy failed')
                  } finally {
                    setDeploying(false)
                  }
                }}
                disabled={deploying}
                className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {deploying ? 'Deploying...' : 'Deploy'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
