'use client'

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

const requestRateData = [
  { time: '00:00', rate: 120 },
  { time: '02:00', rate: 98 },
  { time: '04:00', rate: 75 },
  { time: '06:00', rate: 110 },
  { time: '08:00', rate: 145 },
  { time: '10:00', rate: 162 },
  { time: '12:00', rate: 158 },
  { time: '14:00', rate: 170 },
  { time: '16:00', rate: 155 },
  { time: '18:00', rate: 142 },
  { time: '20:00', rate: 130 },
  { time: '22:00', rate: 115 },
]

export default function MonitoringPage() {
  return (
    <div className="p-6">
      <h1 className="mb-6 text-2xl font-bold text-gray-900">Platform Monitoring</h1>

      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Total Servers" value={3} change="+1 this week" changeType="positive" />
        <StatCard title="Total Agents" value={2} change="No change" changeType="neutral" />
        <StatCard title="Request Rate" value="142/s" change="+12%" changeType="positive" />
        <StatCard title="Error Rate" value="0.3%" change="-0.1%" changeType="positive" />
      </div>

      <div className="mb-8 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold text-gray-900">Request Rate Over Time</h2>
        <ResponsiveContainer width="100%" height={300}>
          <LineChart data={requestRateData}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
            <XAxis dataKey="time" tick={{ fontSize: 12 }} stroke="#9ca3af" />
            <YAxis tick={{ fontSize: 12 }} stroke="#9ca3af" />
            <Tooltip />
            <Line
              type="monotone"
              dataKey="rate"
              stroke="#2563eb"
              strokeWidth={2}
              dot={{ r: 3, fill: '#2563eb' }}
              activeDot={{ r: 5 }}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      <div className="flex h-48 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-gray-50 text-gray-400">
        Grafana dashboard embeds will appear here
      </div>
    </div>
  )
}
