// Wall-clock time of day for "last checked" labels, e.g. 14:02:07.
export function formatClock(ms: number): string {
  if (!ms) return ''
  return new Date(ms).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
