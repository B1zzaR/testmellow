import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useRegister } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'

export function RegisterPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [referralCode, setReferralCode] = useState('')
  const [validationError, setValidationError] = useState('')
  const register = useRegister()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationError('')

    if (password.length < 8) {
      setValidationError('Password must be at least 8 characters')
      return
    }
    if (password !== confirm) {
      setValidationError('Passwords do not match')
      return
    }

    register.mutate({
      username,
      password,
      referral_code: referralCode.trim() || undefined,
    })
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-slate-950">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <h1 className="text-3xl font-bold text-gray-900 dark:text-slate-100">VPN Platform</h1>
          <p className="mt-2 text-sm text-gray-500 dark:text-slate-400">Create your account</p>
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-8 shadow-sm dark:border-slate-700 dark:bg-slate-900">
          <form onSubmit={handleSubmit} className="space-y-5" noValidate>
            <Input
              label="Login"
              placeholder="your_login"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoComplete="username"
            />
            <Input
              label="Password"
              type="password"
              placeholder="Minimum 8 characters"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
            />
            <Input
              label="Confirm Password"
              type="password"
              placeholder="Repeat your password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              autoComplete="new-password"
            />
            <Input
              label="Referral Code (optional)"
              placeholder="Enter a referral code"
              value={referralCode}
              onChange={(e) => setReferralCode(e.target.value)}
              autoComplete="off"
              hint="Have a referral code from a friend? Enter it here."
            />

            {(validationError || register.isError) && (
              <Alert
                variant="error"
                message={validationError || register.error?.message || 'Registration failed'}
              />
            )}

            <Button type="submit" className="w-full" loading={register.isPending} size="lg">
              Create Account
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-500 dark:text-slate-400">
            Already have an account?{' '}
            <Link to="/login" className="font-medium text-primary-600 hover:text-primary-700">
              Sign In
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
