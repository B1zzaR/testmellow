/**
 * Format kopecks (1/100 ruble) to ruble string.
 * 1000 kopecks → "10,00 ₽"
 */
export function formatRubles(kopecks: number): string {
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
  return `${yad.toLocaleString('ru-RU')} YAD`
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
      return '1 Week'
    case '1month':
      return '1 Month'
    case '3months':
      return '3 Months'
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
