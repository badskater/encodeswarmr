import { describe, it, expect } from 'vitest'
import { NODE_REGISTRY, NODE_REGISTRY_MAP, NODE_BY_CATEGORY, CATEGORY_ORDER, CATEGORY_LABELS } from '../flow/nodeRegistry'

const EXPECTED_CATEGORIES = ['input', 'encoding', 'analysis', 'condition', 'audio', 'output', 'notification', 'template', 'flow']

describe('nodeRegistry', () => {
  it('has 34 node templates registered', () => {
    expect(NODE_REGISTRY.length).toBe(34)
  })

  it('each node has required fields: type, label, category, icon', () => {
    for (const node of NODE_REGISTRY) {
      expect(node.type, `node ${node.type} missing type`).toBeTruthy()
      expect(node.label, `node ${node.type} missing label`).toBeTruthy()
      expect(node.category, `node ${node.type} missing category`).toBeTruthy()
      expect(node.icon, `node ${node.type} missing icon`).toBeTruthy()
    }
  })

  it('each node has a defaultConfig that is an object', () => {
    for (const node of NODE_REGISTRY) {
      expect(typeof node.defaultConfig, `node ${node.type} defaultConfig is not an object`).toBe('object')
    }
  })

  it('categories match expected set', () => {
    const categories = new Set(NODE_REGISTRY.map(n => n.category))
    for (const cat of categories) {
      expect(EXPECTED_CATEGORIES, `unexpected category: ${cat}`).toContain(cat)
    }
  })

  it('has no duplicate node types', () => {
    const types = NODE_REGISTRY.map(n => n.type)
    const uniqueTypes = new Set(types)
    expect(uniqueTypes.size).toBe(types.length)
  })

  it('NODE_REGISTRY_MAP has the same size as NODE_REGISTRY', () => {
    expect(NODE_REGISTRY_MAP.size).toBe(NODE_REGISTRY.length)
  })

  it('can look up each node type from the map', () => {
    for (const node of NODE_REGISTRY) {
      expect(NODE_REGISTRY_MAP.has(node.type)).toBe(true)
      expect(NODE_REGISTRY_MAP.get(node.type)).toEqual(node)
    }
  })

  it('NODE_BY_CATEGORY groups correctly', () => {
    for (const [cat, nodes] of NODE_BY_CATEGORY.entries()) {
      const expected = NODE_REGISTRY.filter(n => n.category === cat)
      expect(nodes.length).toBe(expected.length)
    }
  })

  it('CATEGORY_ORDER contains all 9 categories', () => {
    expect(CATEGORY_ORDER.length).toBe(9)
    for (const cat of EXPECTED_CATEGORIES) {
      expect(CATEGORY_ORDER).toContain(cat)
    }
  })

  it('CATEGORY_LABELS has an entry for every category', () => {
    for (const cat of EXPECTED_CATEGORIES) {
      expect(CATEGORY_LABELS[cat as keyof typeof CATEGORY_LABELS]).toBeTruthy()
    }
  })

  it('condition nodes have 2 outputs', () => {
    const conditionNodes = NODE_REGISTRY.filter(n => n.category === 'condition')
    for (const node of conditionNodes) {
      expect(node.outputs, `condition node ${node.type} should have 2 outputs`).toBe(2)
    }
  })

  it('input nodes have 0 inputs', () => {
    const inputNodes = NODE_REGISTRY.filter(n => n.category === 'input')
    for (const node of inputNodes) {
      expect(node.inputs, `input node ${node.type} should have 0 inputs`).toBe(0)
    }
  })
})
