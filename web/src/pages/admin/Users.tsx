import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { User } from '../../types'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

const ROLES = ['viewer', 'operator', 'admin'] as const

export default function Users() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ username: '', email: '', role: 'viewer' as User['role'], password: '' })
  const [saving, setSaving] = useState(false)
  const [roleUpdates, setRoleUpdates] = useState<Record<string, string>>({})

  const load = useCallback(async () => {
    try {
      const u = await api.listUsers()
      setUsers(u)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      await api.createUser(form)
      setShowForm(false)
      setForm({ username: '', email: '', role: 'viewer', password: '' })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create user')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string, username: string) => {
    if (!confirm(`Delete user "${username}"?`)) return
    try {
      await api.deleteUser(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  const handleRoleUpdate = async (id: string) => {
    const newRole = roleUpdates[id]
    if (!newRole) return
    try {
      await api.updateUserRole(id, newRole)
      setRoleUpdates(r => { const c = { ...r }; delete c[id]; return c })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update role')
    }
  }

  if (loading) return <p className="text-gray-500">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Users</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Add User'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-white rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-gray-700">New User</h2>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Username</label>
              <input value={form.username} onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" required />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Email</label>
              <input type="email" value={form.email} onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" required />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Role</label>
              <select value={form.role} onChange={e => setForm(f => ({ ...f, role: e.target.value as User['role'] }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm">
                {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Password</label>
              <input type="password" value={form.password} onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" required />
            </div>
          </div>
          <button type="submit" disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {saving ? 'Creating…' : 'Create User'}
          </button>
        </form>
      )}

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {['Username', 'Email', 'Role', 'Created', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {users.map(u => (
              <tr key={u.id} className="hover:bg-gray-50">
                <td className="px-4 py-2 font-medium text-gray-900">{u.username}</td>
                <td className="px-4 py-2 text-gray-500">{u.email}</td>
                <td className="px-4 py-2">
                  <div className="flex items-center gap-2">
                    <select
                      value={roleUpdates[u.id] ?? u.role}
                      onChange={e => setRoleUpdates(r => ({ ...r, [u.id]: e.target.value }))}
                      className="border border-gray-300 rounded px-2 py-1 text-xs"
                    >
                      {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
                    </select>
                    {roleUpdates[u.id] && roleUpdates[u.id] !== u.role && (
                      <button onClick={() => handleRoleUpdate(u.id)}
                        className="text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded hover:bg-blue-200">
                        Save
                      </button>
                    )}
                  </div>
                </td>
                <td className="px-4 py-2 text-gray-500 whitespace-nowrap">{fmtDate(u.created_at)}</td>
                <td className="px-4 py-2">
                  <button onClick={() => handleDelete(u.id, u.username)}
                    className="text-xs text-red-600 hover:underline">Delete</button>
                </td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td colSpan={5} className="px-4 py-4 text-center text-gray-400">No users</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
