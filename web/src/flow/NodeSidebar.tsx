import { useState } from 'react'
import type { NodeCategory } from '../types/flow'
import {
  NODE_BY_CATEGORY,
  CATEGORY_LABELS,
  CATEGORY_COLORS,
  CATEGORY_ORDER,
  type NodeTemplate,
} from './nodeRegistry'

// ---------------------------------------------------------------------------
// Draggable node card
// ---------------------------------------------------------------------------

function NodeCard({ tpl }: { tpl: NodeTemplate }) {
  const color = CATEGORY_COLORS[tpl.category]

  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.setData('application/reactflow-nodetype', tpl.type)
    e.dataTransfer.effectAllowed = 'copy'
  }

  return (
    <div
      draggable
      onDragStart={handleDragStart}
      className="flex items-center gap-2 px-2 py-1.5 rounded cursor-grab active:cursor-grabbing hover:bg-th-surface-muted transition-colors select-none"
      title={tpl.description}
    >
      <span
        className="flex-shrink-0 w-6 h-6 flex items-center justify-center rounded text-xs"
        style={{ background: `${color}22`, color }}
      >
        {tpl.icon}
      </span>
      <span className="text-xs text-th-text truncate">{tpl.label}</span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Category section
// ---------------------------------------------------------------------------

function CategorySection({
  category,
  nodes,
  defaultOpen,
}: {
  category: NodeCategory
  nodes: NodeTemplate[]
  defaultOpen: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)
  const color = CATEGORY_COLORS[category]

  return (
    <div className="border-b border-th-border last:border-b-0">
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center justify-between px-3 py-2 hover:bg-th-surface-muted transition-colors"
      >
        <div className="flex items-center gap-2">
          <span
            className="w-2 h-2 rounded-full flex-shrink-0"
            style={{ background: color }}
          />
          <span className="text-xs font-semibold text-th-text-secondary uppercase tracking-wide">
            {CATEGORY_LABELS[category]}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <span
            className="text-xs px-1.5 py-0.5 rounded-full font-medium"
            style={{ background: `${color}22`, color }}
          >
            {nodes.length}
          </span>
          <svg
            className={`w-3.5 h-3.5 text-th-text-muted transition-transform ${open ? '' : '-rotate-90'}`}
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </button>

      {open && (
        <div className="px-1 pb-1.5">
          {nodes.map(tpl => (
            <NodeCard key={tpl.type} tpl={tpl} />
          ))}
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Sidebar
// ---------------------------------------------------------------------------

export default function NodeSidebar() {
  const [search, setSearch] = useState('')

  const filtered = search.trim().toLowerCase()

  return (
    <div
      className="flex flex-col bg-th-surface border-r border-th-border overflow-hidden"
      style={{ width: 220, flexShrink: 0 }}
    >
      {/* Header */}
      <div className="px-3 py-2.5 border-b border-th-border flex-shrink-0">
        <p className="text-xs font-semibold text-th-text-secondary uppercase tracking-wide mb-2">
          Node Palette
        </p>
        <input
          type="text"
          placeholder="Search nodes…"
          value={search}
          onChange={e => setSearch(e.target.value)}
          className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-xs text-th-text focus:outline-none focus:ring-1"
        />
      </div>

      {/* Node list */}
      <div className="flex-1 overflow-y-auto">
        {filtered ? (
          // Flat search results
          <div className="px-1 py-1.5">
            {CATEGORY_ORDER.flatMap(cat => NODE_BY_CATEGORY.get(cat) ?? [])
              .filter(
                tpl =>
                  tpl.label.toLowerCase().includes(filtered) ||
                  tpl.description.toLowerCase().includes(filtered) ||
                  tpl.type.toLowerCase().includes(filtered)
              )
              .map(tpl => (
                <NodeCard key={tpl.type} tpl={tpl} />
              ))}
          </div>
        ) : (
          // Grouped by category
          CATEGORY_ORDER.map((cat, idx) => {
            const nodes = NODE_BY_CATEGORY.get(cat) ?? []
            if (nodes.length === 0) return null
            return (
              <CategorySection
                key={cat}
                category={cat}
                nodes={nodes}
                defaultOpen={idx < 3}
              />
            )
          })
        )}
      </div>

      {/* Footer hint */}
      <div className="px-3 py-2 border-t border-th-border flex-shrink-0">
        <p className="text-xs text-th-text-subtle text-center">Drag nodes onto canvas</p>
      </div>
    </div>
  )
}
