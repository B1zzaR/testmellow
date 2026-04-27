/**
 * Format kopecks (1/100 ruble) to ruble string.
 * 1000 kopecks → "10,00 ₽"
 */
export function formatRubles(kopecks: number | null | undefined): string {
  if (kopecks == null || !isFinite(kopecks)) return '—'
  return (kopecks / 100).toLocaleString('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 0,
  })
}

/**
 * Format YAD balance with unit suffix.
 */
export function formatYAD(yad: number): string {
  return `${yad.toLocaleString('ru-RU')} ЯД`
}

/**
 * Format ISO date string to localised date.
 */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  })
}

/**
 * Format ISO date string to localised date + time.
 */
export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * Map subscription plan key to human-readable label.
 */
export function planLabel(plan: string): string {
  switch (plan) {
    case '1week':
      return '1 неделя'
    case '1month':
      return '1 месяц'
    case '3months':
      return '3 месяца'
    case '99years':
      return 'навсегда (99 лет)'
    case 'device_expansion':
      return '+1 устройство'
    case 'device_expansion2':
      return '+2 устройства'
    default:
      return plan
  }
}

/**
 * Returns number of days remaining until expiresAt or 0 if expired.
 */
export function daysUntil(iso: string): number {
  const diff = new Date(iso).getTime() - Date.now()
  return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)))
}

/**
 * Format bytes to human-readable string (KB, MB, GB, TB).
 */
export function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 Б'
  const units = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const val = bytes / Math.pow(1024, i)
  return `${val.toFixed(i === 0 ? 0 : 2)} ${units[i]}`
}
