import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { useAuthStore } from '@/app/store/authStore'
import { ApiError } from '@/services/api'
import AuthLayout from './AuthLayout'

interface RegisterForm {
  displayName: string
  email: string
  password: string
  confirmPassword: string
}

export default function Register() {
  const navigate = useNavigate()
  const registerUser = useAuthStore((s) => s.register)
  const [serverError, setServerError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<RegisterForm>()

  const onSubmit = async (data: RegisterForm) => {
    setServerError(null)
    try {
      await registerUser(data.email, data.displayName, data.password)
      navigate('/dashboard', { replace: true })
    } catch (e) {
      setServerError(e instanceof ApiError ? e.message : 'Something went wrong. Please try again.')
    }
  }

  return (
    <AuthLayout>
      <div className="auth-card">

        {/* Header */}
        <div className="mb-8">
          <h2 className="text-2xl font-bold text-brand-text">Create your account</h2>
          <p className="text-brand-muted text-sm mt-1">Your story starts here</p>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-5">

          {/* Display name */}
          <div className="space-y-1.5">
            <label htmlFor="displayName" className="block text-xs font-medium text-brand-muted uppercase tracking-wider">
              Author name
            </label>
            <input
              id="displayName"
              type="text"
              autoComplete="name"
              placeholder="Your pen name or real name"
              className="input-field"
              {...register('displayName', {
                required: 'Author name is required',
                minLength: { value: 2, message: 'Name must be at least 2 characters' },
                maxLength: { value: 50, message: 'Name must be 50 characters or less' },
              })}
            />
            {errors.displayName && (
              <p className="text-red-400 text-xs">{errors.displayName.message}</p>
            )}
          </div>

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
              autoComplete="new-password"
              placeholder="••••••••"
              className="input-field"
              {...register('password', {
                required: 'Password is required',
                minLength: { value: 8, message: 'Password must be at least 8 characters' },
              })}
            />
            {errors.password && (
              <p className="text-red-400 text-xs">{errors.password.message}</p>
            )}
          </div>

          {/* Confirm password */}
          <div className="space-y-1.5">
            <label htmlFor="confirmPassword" className="block text-xs font-medium text-brand-muted uppercase tracking-wider">
              Confirm password
            </label>
            <input
              id="confirmPassword"
              type="password"
              autoComplete="new-password"
              placeholder="••••••••"
              className="input-field"
              {...register('confirmPassword', {
                required: 'Please confirm your password',
                validate: (v) => v === watch('password') || 'Passwords do not match',
              })}
            />
            {errors.confirmPassword && (
              <p className="text-red-400 text-xs">{errors.confirmPassword.message}</p>
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
                Creating account…
              </span>
            ) : (
              'Create account'
            )}
          </button>
        </form>

        {/* Divider */}
        <div className="flex items-center gap-3 my-6">
          <div className="flex-1 h-px bg-brand-border" />
          <span className="text-brand-muted text-xs">or</span>
          <div className="flex-1 h-px bg-brand-border" />
        </div>

        {/* Login link */}
        <p className="text-center text-sm text-brand-muted">
          Already have an account?{' '}
          <Link to="/login" className="btn-ghost">
            Sign in
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
