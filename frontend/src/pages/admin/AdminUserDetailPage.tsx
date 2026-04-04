import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatDateTime, formatRubles, formatYAD } from '@/utils/formatters'

export function AdminUserDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: user, isLoading } = useQuery({
    queryKey: ['admin-user', id],
    queryFn: () => adminApi.getUser(id!),
    enabled: Boolean(id),
  })

  const [riskModalOpen, setRiskModalOpen] = useState(false)
  const [riskScore, setRiskScore] = useState('')
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-user', id] })
    queryClient.invalidateQueries({ queryKey: ['admin-users'] })
  }

  const banMutation = useMutation({
    mutationFn: () => adminApi.banUser(id!),
    onSuccess: () => { setSuccessMsg('User banned'); invalidate() },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const unbanMutation = useMutation({
    mutationFn: () => adminApi.unbanUser(id!),
    onSuccess: () => { setSuccessMsg('User unbanned'); invalidate() },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const riskMutation = useMutation({
    mutationFn: (score: number) => adminApi.setRiskScore(id!, { score }),
    onSuccess: () => {
      setSuccessMsg('Risk score updated')
      setRiskModalOpen(false)
      invalidate()
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  if (isLoading) return <PageSpinner />
  if (!user) return <Alert variant="error" message="User not found" />

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate('/admin/users')}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100"
        >
          ←
        </button>
        <h1 className="text-xl font-bold text-gray-900">
          {user.email ?? user.username ?? user.id}
        </h1>
        <div className="flex gap-1">
          {user.is_admin && <Badge label="Admin" variant="purple" />}
          {user.is_banned && <Badge label="Banned" variant="red" />}
          {!user.is_banned && !user.is_admin && <Badge label="Active" variant="green" />}
        </div>
      </div>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <div className="grid gap-4 sm:grid-cols-2">
        <Card title="Account Info">
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="text-gray-500">User ID</dt>
              <dd className="mt-0.5 font-mono text-xs text-gray-700">{user.id}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Email</dt>
              <dd className="mt-0.5 font-medium">{user.email ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Username</dt>
              <dd className="mt-0.5 font-medium">{user.username ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Telegram ID</dt>
              <dd className="mt-0.5 font-medium">{String(user.id ?? '—')}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Referral Code</dt>
              <dd className="mt-0.5 font-mono">{user.referral_code}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Joined</dt>
              <dd className="mt-0.5">{formatDateTime(user.created_at)}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Trial Used</dt>
              <dd className="mt-0.5">{user.trial_used ? 'Yes' : 'No'}</dd>
            </div>
          </dl>
        </Card>

        <Card title="Financial Info">
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="text-gray-500">YAD Balance</dt>
              <dd className="mt-0.5 font-semibold text-lg">{formatYAD(user.yad_balance)}</dd>
            </div>
            <div>
              <dt className="text-gray-500">LTV</dt>
              <dd className="mt-0.5 font-semibold">{formatRubles(user.ltv_kopecks)}</dd>
            </div>
            <div>
              <dt className="text-gray-500">Risk Score</dt>
              <dd
                className={`mt-0.5 font-bold text-lg ${
                  user.risk_score >= 70
                    ? 'text-red-600'
                    : user.risk_score >= 40
                    ? 'text-yellow-600'
                    : 'text-green-600'
                }`}
              >
                {user.risk_score} / 100
              </dd>
            </div>
          </dl>
        </Card>
      </div>

      {/* Admin actions */}
      <Card title="Actions">
        <div className="flex flex-wrap gap-3">
          {user.is_banned ? (
            <Button
              variant="secondary"
              loading={unbanMutation.isPending}
              onClick={() => unbanMutation.mutate()}
            >
              Unban User
            </Button>
          ) : (
            <Button
              variant="danger"
              loading={banMutation.isPending}
              onClick={() => banMutation.mutate()}
            >
              Ban User
            </Button>
          )}
          <Button
            variant="secondary"
            onClick={() => {
              setRiskScore(String(user.risk_score))
              setRiskModalOpen(true)
            }}
          >
            Set Risk Score
          </Button>
        </div>
      </Card>

      <Modal
        open={riskModalOpen}
        onClose={() => setRiskModalOpen(false)}
        title="Update Risk Score"
        footer={
          <>
            <Button variant="secondary" onClick={() => setRiskModalOpen(false)}>
              Cancel
            </Button>
            <Button
              loading={riskMutation.isPending}
              onClick={() => {
                const n = parseInt(riskScore, 10)
                if (!isNaN(n) && n >= 0 && n <= 100) riskMutation.mutate(n)
              }}
            >
              Save
            </Button>
          </>
        }
      >
        <Input
          label="Risk Score (0–100)"
          type="number"
          min={0}
          max={100}
          value={riskScore}
          onChange={(e) => setRiskScore(e.target.value)}
        />
        <p className="mt-2 text-xs text-gray-400">
          0–39 = Low risk, 40–69 = Medium, 70–100 = High risk
        </p>
      </Modal>
    </div>
  )
}
