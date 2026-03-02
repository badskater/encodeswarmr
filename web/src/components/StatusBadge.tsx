interface Props { status: string }

const colors: Record<string, string> = {
  running:          'bg-blue-100 text-blue-800',
  completed:        'bg-green-100 text-green-800',
  done:             'bg-green-100 text-green-800',
  failed:           'bg-red-100 text-red-800',
  error:            'bg-red-100 text-red-800',
  cancelled:        'bg-gray-100 text-gray-600',
  queued:           'bg-yellow-100 text-yellow-800',
  assigned:         'bg-indigo-100 text-indigo-800',
  pending:          'bg-yellow-100 text-yellow-800',
  idle:             'bg-green-100 text-green-800',
  offline:          'bg-red-100 text-red-800',
  draining:         'bg-orange-100 text-orange-800',
  pending_approval: 'bg-purple-100 text-purple-800',
  ready:            'bg-green-100 text-green-800',
  new:              'bg-gray-100 text-gray-600',
  analysing:        'bg-blue-100 text-blue-800',
  encoding:         'bg-indigo-100 text-indigo-800',
}

export default function StatusBadge({ status }: Props) {
  const cls = colors[status] ?? 'bg-gray-100 text-gray-600'
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${cls}`}>
      {status.replace(/_/g, ' ')}
    </span>
  )
}
