import { invalidJob } from "../../domain/errors.js";

/**
 * Returns the existing row id the client referenced, or null for a new row.
 * Throws when a positive id does not belong to this job.
 */
export function existingId(
  label: string,
  existing: Set<number>,
  id: number | undefined,
): number | null {
  const key = id ?? 0;
  if (key <= 0) return null;
  if (!existing.has(key)) throw invalidJob(`${label} id ${key} not found`);
  return key;
}

export async function removeMissing(
  existing: Iterable<number>,
  keep: Set<number>,
  remove: (id: number) => Promise<void>,
): Promise<void> {
  for (const id of existing) if (!keep.has(id)) await remove(id);
}

/** Upserts `inputs` by client id and deletes existing rows that were not sent. */
export async function reconcile<I extends { id?: number }>(
  label: string,
  existing: Set<number>,
  inputs: I[],
  save: (input: I, id: number | null) => Promise<number>,
  remove: (id: number) => Promise<void>,
): Promise<void> {
  const keep = new Set<number>();
  for (const input of inputs)
    keep.add(await save(input, existingId(label, existing, input.id)));
  await removeMissing(existing, keep, remove);
}
