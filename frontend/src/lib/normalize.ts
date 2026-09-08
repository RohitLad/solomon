// Null-safe payload guards.
//
// Context: Go's encoding/json serializes a nil slice as `null`, not `[]`.
// On a fresh DB the API returned `null` for every empty list and the UI
// crashed with "can't access property map, exp is null".
//
// The backend now guarantees `[]`, but every list-typed API field consumed
// with `.map()` / `.length` / `{#each}` must still go through asArray()
// (and record-typed fields through asRecord()) so a single `null` payload
// can never white-screen the app again.

export function asArray<T>(v: T[] | null | undefined): T[] {
  return Array.isArray(v) ? v : [];
}

export function asRecord<V>(v: Record<string, V> | null | undefined): Record<string, V> {
  return v !== null && typeof v === 'object' && !Array.isArray(v) ? v : {};
}
