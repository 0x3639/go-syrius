// formatAmountExact converts a base-unit integer string to its exact human
// decimal string (full precision, trailing zeros stripped). Use where the
// precise value matters — e.g. the confirm-what-you-sign modal (sub-project B).
export function formatAmountExact(base: string, decimals: number): string {
  const neg = base.startsWith('-')
  const digits = (neg ? base.slice(1) : base).padStart(decimals + 1, '0')
  const intPart = digits.slice(0, digits.length - decimals)
  const frac = digits.slice(digits.length - decimals).replace(/0+$/, '')
  const out = frac ? `${intPart}.${frac}` : intPart
  return neg && out !== '0' ? `-${out}` : out
}

// formatAmount is the display formatter. Precision depends on the size of the
// integer part: 3+ integer digits (>= 100) drops the decimals entirely; smaller
// values round to 2 decimals (half-up, trailing zeros stripped). The integer
// part always gets thousands separators. e.g. 200 -> "200",
// 20.011111 -> "20.01", 50454.01869374 -> "50,454", 500000 -> "500,000". Uses
// BigInt so large balances never lose integer precision. For the exact value,
// use formatAmountExact.
export function formatAmount(base: string, decimals: number): string {
  const neg = base.startsWith('-')
  const b = BigInt((neg ? base.slice(1) : base) || '0')
  const intMagnitude = b / 10n ** BigInt(decimals) // floor of the integer part
  const maxFrac = intMagnitude >= 100n ? 0 : 2 // 3+ integer digits -> no decimals

  let rounded = b
  let dec = decimals
  if (dec > maxFrac) {
    const scale = 10n ** BigInt(dec - maxFrac)
    const q = b / scale
    const r = b % scale
    rounded = r * 2n >= scale ? q + 1n : q // round half-up
    dec = maxFrac
  }

  const s = rounded.toString().padStart(dec + 1, '0')
  let intPart = s.slice(0, s.length - dec)
  const frac = dec ? s.slice(s.length - dec).replace(/0+$/, '') : ''
  intPart = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',') // thousands separators
  const out = frac ? `${intPart}.${frac}` : intPart
  return neg && rounded !== 0n ? `-${out}` : out
}
