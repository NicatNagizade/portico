export function now(): string {
  return new Date().toISOString();
}

export function iso(value: unknown): string {
  if (value instanceof Date) return value.toISOString();
  if (typeof value === "string" && value !== "")
    return new Date(value).toISOString();
  return new Date().toISOString();
}

export function isoOrNull(value: unknown): string | null {
  if (value == null || value === "") return null;
  return iso(value);
}

export function asObject(value: unknown): Record<string, unknown> | null {
  if (value == null) return null;
  if (
    typeof value === "object" &&
    !Array.isArray(value) &&
    !Buffer.isBuffer(value)
  ) {
    return value as Record<string, unknown>;
  }
  if (typeof value === "string") {
    const text = value.trim();
    if (text === "" || text === "null") return null;
    const parsed = JSON.parse(text) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed))
      return parsed as Record<string, unknown>;
  }
  return null;
}

export function num(value: unknown): number | null {
  if (value == null || value === "") return null;
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : null;
}

export function bool(value: unknown, fallback = true): boolean {
  if (typeof value === "boolean") return value;
  if (value === 1 || value === "1" || value === "true") return true;
  if (value === 0 || value === "0" || value === "false") return false;
  return fallback;
}

export function parseInt64(v: unknown): number | null {
  if (v == null) return null;
  const n = Number.parseInt(String(v).trim(), 10);
  return Number.isFinite(n) && String(n) === String(v).trim() ? n : null;
}

export function positiveOr(value: number | undefined, fallback: number) {
  return value && value > 0 ? value : fallback;
}

export function splitList(value: string): string[] {
  return value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
