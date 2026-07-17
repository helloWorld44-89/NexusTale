import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuthStore } from './store/authStore'
import Login from '@/pages/Auth/Login'
import Register from '@/pages/Auth/Register'
import Dashboard from '@/pages/Dashboard'
import ProjectHome from '@/pages/ProjectHome'
import Editor from '@/pages/Editor'
import Guide from '@/pages/Guide'
import WikiHub from '@/pages/WikiHub'
import MapsHub from '@/pages/MapsHub'
import MapStudio from '@/pages/MapStudio'
import Settings from '@/pages/Settings'
import InviteAccept from '@/pages/InviteAccept'
import About from '@/pages/About'
import Landing from '@/pages/Landing'
import Admin from '@/pages/Admin'
import ImportManuscript from '@/pages/ImportManuscript'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />
}

export function AppRouter() {
  return (
    <Routes>
      <Route path="/"         element={<Landing />} />
      <Route path="/login"    element={<Login />} />
      <Route path="/register" element={<Register />} />

      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <Dashboard />
          </ProtectedRoute>
        }
      />

      <Route
        path="/projects/:id"
        element={
          <ProtectedRoute>
            <ProjectHome />
          </ProtectedRoute>
        }
      />

      <Route
        path="/projects/:id/guide"
        element={
          <ProtectedRoute>
            <Guide />
          </ProtectedRoute>
        }
      />

      <Route
        path="/projects/:id/editor"
        element={
          <ProtectedRoute>
            <Editor />
          </ProtectedRoute>
        }
      />

      <Route
        path="/projects/:id/wiki"
        element={
          <ProtectedRoute>
            <WikiHub />
          </ProtectedRoute>
        }
      />

      <Route
        path="/projects/:id/maps"
        element={
          <ProtectedRoute>
            <MapsHub />
          </ProtectedRoute>
        }
      />

      <Route
        path="/projects/:id/maps/:mid"
        element={
          <ProtectedRoute>
            <MapStudio />
          </ProtectedRoute>
        }
      />

      <Route
        path="/settings"
        element={
          <ProtectedRoute>
            <Settings />
          </ProtectedRoute>
        }
      />

      <Route path="/about" element={<About />} />

      <Route
        path="/import"
        element={
          <ProtectedRoute>
            <ImportManuscript />
          </ProtectedRoute>
        }
      />

      <Route
        path="/admin"
        element={
          <ProtectedRoute>
            <Admin />
          </ProtectedRoute>
        }
      />

      {/* Invite accept — works logged in or out; page handles both states */}
      <Route path="/invites/:token" element={<InviteAccept />} />

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
