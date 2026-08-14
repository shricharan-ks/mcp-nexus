'use client'

import { useState } from 'react'

interface MarketplaceEntry {
  name: string
  vendor: string
  version: string
  category: string
  description: string
  tools: number
  installs: number
  verified: boolean
}

const mockEntries: MarketplaceEntry[] = [
  {
    name: 'GitHub',
    vendor: 'GitHub Inc.',
    version: 'v1.2.0',
    category: 'Developer Tools',
    description:
      'Access GitHub repositories, issues, pull requests, and actions. Enables AI agents to interact with your codebase and development workflow.',
    tools: 12,
    installs: 8420,
    verified: true,
  },
  {
    name: 'Slack',
    vendor: 'Salesforce',
    version: 'v0.9.1',
    category: 'Communication',
    description:
      'Send and receive Slack messages, manage channels, and automate workspace interactions through the MCP protocol.',
    tools: 8,
    installs: 5230,
    verified: true,
  },
  {
    name: 'PostgreSQL',
    vendor: 'Community',
    version: 'v2.0.3',
    category: 'Data',
    description:
      'Query and manage PostgreSQL databases. Supports read/write operations, schema introspection, and query optimization suggestions.',
    tools: 15,
    installs: 6100,
    verified: true,
  },
  {
    name: 'Brave Search',
    vendor: 'Brave Software',
    version: 'v1.0.0',
    category: 'AI/ML',
    description:
      'Web and local search powered by Brave. Provides real-time search results, summarization, and structured data extraction.',
    tools: 3,
    installs: 3800,
    verified: true,
  },
  {
    name: 'Filesystem',
    vendor: 'MCP Core',
    version: 'v1.1.0',
    category: 'Productivity',
    description:
      'Secure filesystem access with configurable sandboxing. Read, write, and manage files within allowed directory boundaries.',
    tools: 10,
    installs: 9100,
    verified: true,
  },
  {
    name: 'HuggingFace',
    vendor: 'Hugging Face',
    version: 'v0.5.2',
    category: 'AI/ML',
    description:
      'Browse and query models, datasets, and spaces on the Hugging Face Hub. Run inference and manage ML artifacts from AI agents.',
    tools: 9,
    installs: 2750,
    verified: false,
  },
]

const categories = ['All', 'Developer Tools', 'Data', 'Communication', 'Productivity', 'AI/ML']

export default function MarketplacePage() {
  const [search, setSearch] = useState('')
  const [activeCategory, setActiveCategory] = useState('All')
  const [deployTarget, setDeployTarget] = useState<MarketplaceEntry | null>(null)
  const [secrets, setSecrets] = useState({ apiKey: '', apiSecret: '' })

  const filtered = mockEntries.filter((entry) => {
    const matchesSearch =
      entry.name.toLowerCase().includes(search.toLowerCase()) ||
      entry.description.toLowerCase().includes(search.toLowerCase())
    const matchesCategory = activeCategory === 'All' || entry.category === activeCategory
    return matchesSearch && matchesCategory
  })

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
            {cat}
          </button>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {filtered.map((entry) => (
          <div
            key={entry.name}
            className="flex flex-col rounded-lg border border-gray-200 bg-white p-5 shadow-sm"
          >
            <div className="mb-2 flex items-start justify-between">
              <div>
                <h3 className="flex items-center gap-1.5 text-lg font-semibold text-gray-900">
                  {entry.name}
                  {entry.verified && (
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
                <p className="text-xs text-gray-500">{entry.vendor}</p>
              </div>
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                {entry.version}
              </span>
            </div>

            <span className="mb-2 inline-flex w-fit rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
              {entry.category}
            </span>

            <p className="mb-4 line-clamp-2 flex-1 text-sm text-gray-600">{entry.description}</p>

            <div className="mb-4 flex items-center gap-4 text-xs text-gray-500">
              <span>{entry.tools} tools</span>
              <span>{entry.installs.toLocaleString()} installs</span>
            </div>

            <button
              onClick={() => {
                setDeployTarget(entry)
                setSecrets({ apiKey: '', apiSecret: '' })
              }}
              className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              Deploy
            </button>
          </div>
        ))}
      </div>

      {deployTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-semibold text-gray-900">
              Deploy {deployTarget.name}
            </h2>

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
                className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  setDeployTarget(null)
                }}
                className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
              >
                Deploy
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
