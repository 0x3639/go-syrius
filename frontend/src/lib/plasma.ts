// Plasma level + colour mapping, shared by the top-bar indicator and StatusStrip.
// Thresholds are the brief values inherited verbatim from the merged Svelte UI.
export type PlasmaLevel = 'None' | 'Low' | 'Medium' | 'High'

export function plasmaLevel(p: number): PlasmaLevel {
  if (p >= 84000) return 'High'
  if (p >= 21000) return 'Medium'
  if (p > 0) return 'Low'
  return 'None'
}

// Tailwind text-colour class for the plasma bolt: off (none) → red → yellow → green.
export function plasmaColorClass(level: PlasmaLevel): string {
  switch (level) {
    case 'High':
      return 'text-primary'
    case 'Medium':
      return 'text-yellow-500'
    case 'Low':
      return 'text-destructive'
    default:
      return 'text-muted-foreground'
  }
}
