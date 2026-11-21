// Bounds a read that may never be answered. The node calls go over a
// WebSocket with no per-call deadline, so a request the node silently drops
// on an otherwise healthy connection would stay pending forever. The caller
// sees a rejection instead and can report the value as unknown and try again.
// A response that arrives after the bound is ignored.
export const READ_TIMEOUT_MS = 20_000 // two momentum intervals

export function withTimeout<T>(read: Promise<T>, ms = READ_TIMEOUT_MS): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('The node did not answer in time.')), ms)
    read.then(
      (v) => { clearTimeout(timer); resolve(v) },
      (e) => { clearTimeout(timer); reject(e) },
    )
  })
}
