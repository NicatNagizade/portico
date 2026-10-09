export function groupBy<T, K>(items: T[], key: (item: T) => K): Map<K, T[]> {
  const groups = new Map<K, T[]>();
  for (const item of items) {
    const k = key(item);
    const list = groups.get(k);
    if (list) list.push(item);
    else groups.set(k, [item]);
  }
  return groups;
}

export async function forEachChunk<T>(
  items: T[],
  chunkSize: number,
  fn: (chunk: T[]) => Promise<void>,
): Promise<void> {
  const size = chunkSize > 0 ? chunkSize : 500;
  for (let i = 0; i < items.length; i += size)
    await fn(items.slice(i, i + size));
}

export function uniqueBy<T>(items: T[], key: (item: T) => string): T[] {
  const seen = new Set<string>();
  const out: T[] = [];
  for (const item of items) {
    const k = key(item);
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(item);
  }
  return out;
}
