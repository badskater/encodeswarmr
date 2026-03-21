import { Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import * as api from './api/client'
import type { User } from './types'
import Layout from './components/Layout'
import Login from './pages/Login'
import Setup from './pages/Setup'
import Dashboard from './pages/Dashboard'
import Sources from './pages/Sources'
import SourceDetail from './pages/SourceDetail'
import Jobs from './pages/Jobs'
import CreateJob from './pages/CreateJob'
import JobDetail from './pages/JobDetail'
import TaskDetail from './pages/TaskDetail'
import Agents from './pages/Agents'
import Templates from './pages/admin/Templates'
import Users from './pages/admin/Users'
import Webhooks from './pages/admin/Webhooks'
import Variables from './pages/admin/Variables'
import AudioConvert from './pages/AudioConvert'
import PathMappings from './pages/admin/PathMappings'
import EnrollmentTokens from './pages/admin/EnrollmentTokens'
import Schedules from './pages/admin/Schedules'
import Plugins from './pages/admin/Plugins'
import ThemeSettings from './pages/admin/ThemeSettings'

function App() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [setupRequired, setSetupRequired] = useState(false)

  useEffect(() => {
    const init = async () => {
      try {
        const status = await api.getSetupStatus()
        if (status.required) {
          setSetupRequired(true)
          return
        }
      } catch {
        // If the setup/status endpoint is unreachable, fall through to normal auth.
      }
      try {
        const u = await api.getMe()
        setUser(u)
      } catch {
        // Not authenticated — Login page handles this.
      }
    }
    init().finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center text-th-text-muted">
        Loading…
      </div>
    )
  }

  if (setupRequired) {
    return (
      <Routes>
        <Route
          path="/setup"
          element={<Setup onComplete={() => setSetupRequired(false)} />}
        />
        <Route path="*" element={<Navigate to="/setup" replace />} />
      </Routes>
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
        <Route path="/jobs/create" element={<CreateJob />} />
        <Route path="/jobs/:id" element={<JobDetail />} />
        <Route path="/tasks/:id" element={<TaskDetail />} />
        <Route path="/agents" element={<Agents />} />
        <Route path="/audio-convert" element={<AudioConvert />} />
        {user.role === 'admin' && (
          <>
            <Route path="/admin/templates" element={<Templates />} />
            <Route path="/admin/users" element={<Users />} />
            <Route path="/admin/webhooks" element={<Webhooks />} />
            <Route path="/admin/variables" element={<Variables />} />
            <Route path="/admin/path-mappings" element={<PathMappings />} />
            <Route path="/admin/enrollment-tokens" element={<EnrollmentTokens />} />
            <Route path="/admin/schedules" element={<Schedules />} />
            <Route path="/admin/plugins" element={<Plugins />} />
            <Route path="/admin/theme" element={<ThemeSettings />} />
          </>
        )}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  )
}

export default App
