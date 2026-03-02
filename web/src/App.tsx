import { Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import * as api from './api/client'
import type { User } from './types'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Sources from './pages/Sources'
import SourceDetail from './pages/SourceDetail'
import Jobs from './pages/Jobs'
import JobDetail from './pages/JobDetail'
import TaskDetail from './pages/TaskDetail'
import Agents from './pages/Agents'
import Templates from './pages/admin/Templates'
import Users from './pages/admin/Users'
import Webhooks from './pages/admin/Webhooks'
import Variables from './pages/admin/Variables'

function App() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.getMe()
      .then(u => setUser(u))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center text-gray-500">
        Loading…
      </div>
    )
  }

  if (!user) {
    return (
      <Routes>
        <Route path="/login" element={<Login onLogin={u => setUser(u)} />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  return (
    <Layout role={user.role} onLogout={() => setUser(null)}>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/sources" element={<Sources />} />
        <Route path="/sources/:id" element={<SourceDetail />} />
        <Route path="/jobs" element={<Jobs />} />
        <Route path="/jobs/:id" element={<JobDetail />} />
        <Route path="/tasks/:id" element={<TaskDetail />} />
        <Route path="/agents" element={<Agents />} />
        {user.role === 'admin' && (
          <>
            <Route path="/admin/templates" element={<Templates />} />
            <Route path="/admin/users" element={<Users />} />
            <Route path="/admin/webhooks" element={<Webhooks />} />
            <Route path="/admin/variables" element={<Variables />} />
          </>
        )}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  )
}

export default App
