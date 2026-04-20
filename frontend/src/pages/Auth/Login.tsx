import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { useAuthStore } from '@/app/store/authStore'
import { ApiError } from '@/services/api'
import AuthLayout from './AuthLayout'

interface LoginForm {
  email: string
  password: string
}

export default function Login() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const login = useAuthStore((s) => s.login)
  const [serverError, setServerError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginForm>()

  const onSubmit = async (data: LoginForm) => {
    setServerError(null)
    try {
      await login(data.email, data.password)
      const redirect = searchParams.get('redirect')
      navigate(redirect ?? '/dashboard', { replace: true })
    } catch (e) {
      setServerError(e instanceof ApiError ? e.message : 'Something went wrong. Please try again.')
    }
  }

  return (
    <AuthLayout>
      <div className="auth-card">

        {/* Header */}
        <div className="mb-8">
          <h2 className="text-2xl font-bold text-brand-text">Welcome back</h2>
          <p className="text-brand-muted text-sm mt-1">Sign in to continue your story</p>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-5">

          {/* Email */}
          <div className="space-y-1.5">
            <label htmlFor="email" className="block text-xs font-medium text-brand-muted uppercase tracking-wider">
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              placeholder="writer@example.com"
              className="input-field"
              {...register('email', {
                required: 'Email is required',
                pattern: { value: /\S+@\S+\.\S+/, message: 'Invalid email address' },
              })}
            />
            {errors.email && (
              <p className="text-red-400 text-xs">{errors.email.message}</p>
            )}
          </div>

          {/* Password */}
          <div className="space-y-1.5">
            <label htmlFor="password" className="block text-xs font-medium text-brand-muted uppercase tracking-wider">
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              className="input-field"
              {...register('password', {
                required: 'Password is required',
                minLength: { value: 6, message: 'Password must be at least 6 characters' },
              })}
            />
            {errors.password && (
              <p className="text-red-400 text-xs">{errors.password.message}</p>
            )}
          </div>

          {/* Server error */}
          {serverError && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3">
              <p className="text-red-400 text-sm">{serverError}</p>
            </div>
          )}

          {/* Submit */}
          <button type="submit" disabled={isSubmitting} className="btn-primary mt-2">
            {isSubmitting ? (
              <span className="flex items-center justify-center gap-2">
                <SpinnerIcon />
                Signing in…
              </span>
            ) : (
              'Sign in'
            )}
          </button>
        </form>

        {/* Divider */}
        <div className="flex items-center gap-3 my-6">
          <div className="flex-1 h-px bg-brand-border" />
          <span className="text-brand-muted text-xs">or</span>
          <div className="flex-1 h-px bg-brand-border" />
        </div>

        {/* Register link */}
        <p className="text-center text-sm text-brand-muted">
          Don't have an account?{' '}
          <Link to="/register" className="btn-ghost">
            Create one
          </Link>
        </p>
      </div>
    </AuthLayout>
  )
}

function SpinnerIcon() {
  return (
    <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  )
}
