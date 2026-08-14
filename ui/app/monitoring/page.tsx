'use client'

import { useState, useEffect } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import StatCard from '@/components/stat-card'
import { fetchServers, fetchAgents } from '@/lib/api'

export default function MonitoringPage() {
  const [serverCount, setServerCount] = useState(0)
  const [agentCount, setAgentCount] = useState(0)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    Promise.allSettled([
      fetchServers().then((s) => setServerCount(s.length)),
      fetchAgents().then((a) => setAgentCount(a.length)),
    ]).finally(() => setLoaded(true))
  }, [])

  return (
    <div className="p-6">
      <h1 className="mb-6 text-2xl font-bold text-gray-900">Platform Monitoring</h1>

      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Servers"
          value={loaded ? serverCount : '-'}
          change=""
          changeType="neutral"
        />
        <StatCard
          title="Total Agents"
          value={loaded ? agentCount : '-'}
          change=""
          changeType="neutral"
        />
        <StatCard title="Request Rate" value="-" change="" changeType="neutral" />
        <StatCard title="Error Rate" value="-" change="" changeType="neutral" />
      </div>

      <div className="mb-8 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold text-gray-900">Request Rate Over Time</h2>
        <div className="flex h-[300px] items-center justify-center text-gray-400">
          No monitoring data available. Connect a metrics source to display request rate charts.
        </div>
      </div>

      <div className="flex h-48 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-gray-50 text-gray-400">
        Grafana dashboard embeds will appear here
      </div>
    </div>
  )
}
