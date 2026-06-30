// USD display formatter for the dashboard hero + balance cards. Fiat is
// display-only — never used for any signing/amount path (amounts use lib/format).
export function formatFiat(n: number): string {
  return '$' + n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
