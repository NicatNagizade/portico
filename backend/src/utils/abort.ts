export function throwIfAborted(signal: AbortSignal): void {
  if (!signal.aborted) return;
  const error = new Error("canceled");
  error.name = "AbortError";
  throw error;
}

export function isAbort(error: unknown): boolean {
  return (
    error instanceof Error &&
    (error.name === "AbortError" || error.message === "canceled")
  );
}
