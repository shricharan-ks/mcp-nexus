interface PhaseBadgeProps {
  phase: string
}

const phaseStyles: Record<string, string> = {
  Running: 'bg-green-100 text-green-800',
  Active: 'bg-green-100 text-green-800',
  Synced: 'bg-green-100 text-green-800',
  Deploying: 'bg-yellow-100 text-yellow-800',
  Registering: 'bg-yellow-100 text-yellow-800',
  Pending: 'bg-gray-100 text-gray-800',
  Failed: 'bg-red-100 text-red-800',
  Suspended: 'bg-red-100 text-red-800',
}

export default function PhaseBadge({ phase }: PhaseBadgeProps) {
  const style = phaseStyles[phase] ?? 'bg-gray-100 text-gray-800'

  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${style}`}
    >
      {phase}
    </span>
  )
}
