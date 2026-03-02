import { ReactNode } from 'react'
import NavBar from './NavBar'

interface Props {
  role: string
  onLogout: () => void
  children: ReactNode
}

export default function Layout({ role, onLogout, children }: Props) {
  return (
    <div className="min-h-screen bg-gray-100">
      <NavBar role={role} onLogout={onLogout} />
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>
    </div>
  )
}
