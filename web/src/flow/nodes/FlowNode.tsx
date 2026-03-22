import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import type { FlowNodeData } from '../../types/flow'
import { CATEGORY_COLORS } from '../nodeRegistry'

function FlowNode({ data, selected }: NodeProps) {
  const nodeData = data as unknown as FlowNodeData
  const color = CATEGORY_COLORS[nodeData.category] ?? '#6b7280'
  const isCondition = nodeData.category === 'condition'

  // Determine handle counts from the nodeType stored in data
  // We read these off data.inputs / data.outputs which FlowEditor sets when creating nodes
  const inputCount = (data as { inputs?: number }).inputs ?? 1
  const outputCount = (data as { outputs?: number }).outputs ?? 1

  return (
    <div
      style={{
        border: selected ? `2px solid ${color}` : '2px solid transparent',
        borderRadius: 8,
        minWidth: 160,
        maxWidth: 220,
        background: 'var(--th-surface)',
        boxShadow: selected
          ? `0 0 0 1px ${color}40, 0 4px 16px rgba(0,0,0,0.18)`
          : '0 2px 8px rgba(0,0,0,0.12)',
        transition: 'box-shadow 0.15s, border-color 0.15s',
        overflow: 'hidden',
      }}
    >
      {/* Coloured header */}
      <div
        style={{
          background: color,
          padding: '6px 10px',
          display: 'flex',
          alignItems: 'center',
          gap: 6,
        }}
      >
        <span style={{ fontSize: 14 }}>{nodeData.icon}</span>
        <span
          style={{
            color: '#fff',
            fontWeight: 600,
            fontSize: 12,
            lineHeight: 1.2,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {nodeData.label}
        </span>
        {isCondition && (
          <span style={{ marginLeft: 'auto', fontSize: 10, opacity: 0.85 }}>⚡</span>
        )}
      </div>

      {/* Body — description when selected */}
      {selected && nodeData.description && (
        <div
          style={{
            padding: '5px 10px 6px',
            fontSize: 11,
            color: 'var(--th-text-muted)',
            lineHeight: 1.4,
            borderBottom: '1px solid var(--th-border)',
          }}
        >
          {nodeData.description}
        </div>
      )}

      {/* Padding so handles have space */}
      <div style={{ height: selected && nodeData.description ? 4 : 10 }} />

      {/* Input handles — left side */}
      {inputCount > 0 &&
        Array.from({ length: inputCount }).map((_, i) => (
          <Handle
            key={`in-${i}`}
            type="target"
            position={Position.Left}
            id={`in-${i}`}
            style={{
              top: inputCount === 1 ? '50%' : `${((i + 1) / (inputCount + 1)) * 100}%`,
              background: color,
              width: 10,
              height: 10,
              border: '2px solid var(--th-surface)',
            }}
          />
        ))}

      {/* Output handles — right side */}
      {outputCount > 0 &&
        Array.from({ length: outputCount }).map((_, i) => {
          const isTrue = isCondition && i === 0
          const isFalse = isCondition && i === 1
          const label = isTrue ? '✓' : isFalse ? '✗' : undefined
          const handleColor = isTrue ? '#22c55e' : isFalse ? '#ef4444' : color

          return (
            <div key={`out-wrapper-${i}`}>
              <Handle
                type="source"
                position={Position.Right}
                id={`out-${i}`}
                style={{
                  top: outputCount === 1 ? '50%' : `${((i + 1) / (outputCount + 1)) * 100}%`,
                  background: handleColor,
                  width: 10,
                  height: 10,
                  border: '2px solid var(--th-surface)',
                }}
              />
              {label && (
                <span
                  style={{
                    position: 'absolute',
                    right: 16,
                    top:
                      outputCount === 1
                        ? 'calc(50% - 7px)'
                        : `calc(${((i + 1) / (outputCount + 1)) * 100}% - 7px)`,
                    fontSize: 10,
                    fontWeight: 700,
                    color: handleColor,
                    pointerEvents: 'none',
                  }}
                >
                  {label}
                </span>
              )}
            </div>
          )
        })}

      <div style={{ height: 10 }} />
    </div>
  )
}

export default memo(FlowNode)
