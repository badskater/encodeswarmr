import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import ThemePicker from './ThemePicker'
import { useBranding } from '../contexts/BrandingContext'

interface Props {
  role: string
  onLogout: () => void
}

const linkCls = ({ isActive }: { isActive: boolean }) =>
  `px-3 py-2 rounded text-sm font-medium transition-colors ${
    isActive ? 'bg-th-nav-active text-white' : 'text-th-nav-text hover:bg-th-nav-hover hover:text-white'
  }`

// Same style but full-width for the mobile drawer
const mobileLinkCls = ({ isActive }: { isActive: boolean }) =>
  `block px-4 py-2.5 text-sm font-medium transition-colors ${
    isActive ? 'bg-th-nav-active text-white' : 'text-th-nav-text hover:bg-th-nav-hover hover:text-white'
  }`

export default function NavBar({ role, onLogout }: Props) {
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const { branding } = useBranding()

  const handleLogout = async () => {
    await api.logout()
    onLogout()
    navigate('/login')
  }

  const closeMenu = () => setMenuOpen(false)

  return (
    <nav className="bg-th-nav-bg">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">

          {/* Logo + desktop links */}
          <div className="flex items-center gap-1">
            {branding.logoUrl
              ? <img src={branding.logoUrl} alt="Logo" className="h-6 w-6 object-contain mr-3" />
              : null
            }
            <span className="text-white font-semibold mr-4">
              {branding.appTitle || 'Encoder'}
            </span>

            {/* Desktop nav links — hidden on mobile */}
            <div className="hidden md:flex items-center gap-1">
              <NavLink to="/" className={linkCls} end>Dashboard</NavLink>
              <NavLink to="/sources" className={linkCls}>Sources</NavLink>
              <NavLink to="/jobs" className={linkCls}>Jobs</NavLink>
              <NavLink to="/queue" className={linkCls}>Queue</NavLink>
              <NavLink to="/agents" className={linkCls}>Agents</NavLink>
              <NavLink to="/audio-convert" className={linkCls}>Audio</NavLink>
              <NavLink to="/flows" className={linkCls}>Flows</NavLink>
              {role === 'admin' && (
                <>
                  <NavLink to="/admin/templates" className={linkCls}>Templates</NavLink>
                  <NavLink to="/admin/variables" className={linkCls}>Variables</NavLink>
                  <NavLink to="/admin/webhooks" className={linkCls}>Webhooks</NavLink>
                  <NavLink to="/admin/users" className={linkCls}>Users</NavLink>
                  <NavLink to="/admin/agent-pools" className={linkCls}>Pools</NavLink>
                  <NavLink to="/admin/path-mappings" className={linkCls}>Path Mappings</NavLink>
                  <NavLink to="/admin/enrollment-tokens" className={linkCls}>Tokens</NavLink>
                  <NavLink to="/admin/schedules" className={linkCls}>Schedules</NavLink>
                  <NavLink to="/admin/plugins" className={linkCls}>Plugins</NavLink>
                  <NavLink to="/admin/theme" className={linkCls}>Theme</NavLink>
                  <NavLink to="/admin/notifications" className={linkCls}>Notifications</NavLink>
                  <NavLink to="/admin/auto-scaling" className={linkCls}>Auto-Scaling</NavLink>
                  <NavLink to="/admin/watch-folders" className={linkCls}>Watch Folders</NavLink>
                  <NavLink to="/admin/encoding-rules" className={linkCls}>Enc. Rules</NavLink>
                </>
              )}
            </div>
          </div>

          {/* Right side: theme picker, role, logout, hamburger */}
          <div className="flex items-center gap-3">
            <ThemePicker />
            <span className="hidden sm:inline text-th-text-subtle text-sm capitalize">{role}</span>
            <button
              onClick={handleLogout}
              className="hidden md:inline text-th-nav-text hover:text-white text-sm px-3 py-1 rounded hover:bg-th-nav-hover"
            >
              Logout
            </button>

            {/* Hamburger button — visible only on mobile */}
            <button
              onClick={() => setMenuOpen(o => !o)}
              className="md:hidden inline-flex items-center justify-center p-2 rounded text-th-nav-text hover:text-white hover:bg-th-nav-hover focus:outline-none"
              aria-label="Toggle navigation menu"
              aria-expanded={menuOpen}
            >
              {menuOpen ? (
                // X icon
                <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              ) : (
                // Hamburger icon
                <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Mobile drawer — only rendered when open */}
      {menuOpen && (
        <div className="md:hidden border-t border-th-nav-hover">
          <div className="py-1">
            <NavLink to="/" className={mobileLinkCls} end onClick={closeMenu}>Dashboard</NavLink>
            <NavLink to="/sources" className={mobileLinkCls} onClick={closeMenu}>Sources</NavLink>
            <NavLink to="/jobs" className={mobileLinkCls} onClick={closeMenu}>Jobs</NavLink>
            <NavLink to="/queue" className={mobileLinkCls} onClick={closeMenu}>Queue</NavLink>
            <NavLink to="/agents" className={mobileLinkCls} onClick={closeMenu}>Agents</NavLink>
            <NavLink to="/audio-convert" className={mobileLinkCls} onClick={closeMenu}>Audio</NavLink>
            <NavLink to="/flows" className={mobileLinkCls} onClick={closeMenu}>Flows</NavLink>
            {role === 'admin' && (
              <>
                <div className="px-4 pt-2 pb-1 text-xs text-th-nav-text opacity-50 uppercase tracking-wide">Admin</div>
                <NavLink to="/admin/templates" className={mobileLinkCls} onClick={closeMenu}>Templates</NavLink>
                <NavLink to="/admin/variables" className={mobileLinkCls} onClick={closeMenu}>Variables</NavLink>
                <NavLink to="/admin/webhooks" className={mobileLinkCls} onClick={closeMenu}>Webhooks</NavLink>
                <NavLink to="/admin/users" className={mobileLinkCls} onClick={closeMenu}>Users</NavLink>
                <NavLink to="/admin/agent-pools" className={mobileLinkCls} onClick={closeMenu}>Agent Pools</NavLink>
                <NavLink to="/admin/path-mappings" className={mobileLinkCls} onClick={closeMenu}>Path Mappings</NavLink>
                <NavLink to="/admin/enrollment-tokens" className={mobileLinkCls} onClick={closeMenu}>Tokens</NavLink>
                <NavLink to="/admin/schedules" className={mobileLinkCls} onClick={closeMenu}>Schedules</NavLink>
                <NavLink to="/admin/plugins" className={mobileLinkCls} onClick={closeMenu}>Plugins</NavLink>
                <NavLink to="/admin/theme" className={mobileLinkCls} onClick={closeMenu}>Theme</NavLink>
                <NavLink to="/admin/notifications" className={mobileLinkCls} onClick={closeMenu}>Notifications</NavLink>
                <NavLink to="/admin/auto-scaling" className={mobileLinkCls} onClick={closeMenu}>Auto-Scaling</NavLink>
                <NavLink to="/admin/watch-folders" className={mobileLinkCls} onClick={closeMenu}>Watch Folders</NavLink>
                <NavLink to="/admin/encoding-rules" className={mobileLinkCls} onClick={closeMenu}>Enc. Rules</NavLink>
              </>
            )}
            <div className="border-t border-th-nav-hover mt-1 pt-1">
              <button
                onClick={() => { closeMenu(); handleLogout() }}
                className="block w-full text-left px-4 py-2.5 text-sm text-th-nav-text hover:bg-th-nav-hover hover:text-white"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      )}
    </nav>
  )
}
