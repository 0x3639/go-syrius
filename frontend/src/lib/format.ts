// formatAmount converts a base-unit integer string to a human decimal string.
export function formatAmount(base: string, decimals: number): string {
  const neg = base.startsWith('-')
  const digits = (neg ? base.slice(1) : base).padStart(decimals + 1, '0')
  const intPart = digits.slice(0, digits.length - decimals)
  let frac = digits.slice(digits.length - decimals).replace(/0+$/, '')
  const out = frac ? `${intPart}.${frac}` : intPart
  return neg ? `-${out}` : out
}

// shortAddress renders z1xxxx…xxxxx for compact display.
export function shortAddress(addr: string): string {
  if (addr.length <= 12) return addr
  return `${addr.slice(0, 6)}…${addr.slice(-5)}`
}
