import { NavLink, useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import ThemePicker from './ThemePicker'

interface Props {
  role: string
  onLogout: () => void
}

const linkCls = ({ isActive }: { isActive: boolean }) =>
  `px-3 py-2 rounded text-sm font-medium transition-colors ${
    isActive ? 'bg-th-nav-active text-white' : 'text-th-nav-text hover:bg-th-nav-hover hover:text-white'
  }`

export default function NavBar({ role, onLogout }: Props) {
  const navigate = useNavigate()

  const handleLogout = async () => {
    await api.logout()
    onLogout()
    navigate('/login')
  }

  return (
    <nav className="bg-th-nav-bg">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center gap-1">
            <span className="text-white font-semibold mr-4">Encoder</span>
            <NavLink to="/" className={linkCls} end>Dashboard</NavLink>
            <NavLink to="/sources" className={linkCls}>Sources</NavLink>
            <NavLink to="/jobs" className={linkCls}>Jobs</NavLink>
            <NavLink to="/agents" className={linkCls}>Agents</NavLink>
            <NavLink to="/audio-convert" className={linkCls}>Audio</NavLink>
            {role === 'admin' && (
              <>
                <NavLink to="/admin/templates" className={linkCls}>Templates</NavLink>
                <NavLink to="/admin/variables" className={linkCls}>Variables</NavLink>
                <NavLink to="/admin/webhooks" className={linkCls}>Webhooks</NavLink>
                <NavLink to="/admin/users" className={linkCls}>Users</NavLink>
                <NavLink to="/admin/path-mappings" className={linkCls}>Path Mappings</NavLink>
                <NavLink to="/admin/enrollment-tokens" className={linkCls}>Tokens</NavLink>
                <NavLink to="/admin/schedules" className={linkCls}>Schedules</NavLink>
              </>
            )}
          </div>
          <div className="flex items-center gap-3">
            <ThemePicker />
            <span className="text-th-text-subtle text-sm capitalize">{role}</span>
            <button
              onClick={handleLogout}
              className="text-th-nav-text hover:text-white text-sm px-3 py-1 rounded hover:bg-th-nav-hover"
            >
              Logout
            </button>
          </div>
        </div>
      </div>
    </nav>
  )
}
