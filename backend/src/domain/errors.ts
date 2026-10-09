export class HttpError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

export const ErrConnectionNotFound = new HttpError("connection not found", 404);
export const ErrSyncJobNotFound = new HttpError("sync job not found", 404);
export const ErrSyncLogNotFound = new HttpError("sync log not found", 404);
export const ErrInvalidJob = new HttpError("invalid sync job", 400);
export const ErrInvalidSide = new HttpError(
  "side must be source or destination",
  400,
);
export const ErrInvalidSort = new HttpError(
  "invalid sort field or direction",
  400,
);
export const ErrInvalidFilter = new HttpError("invalid explore filter", 400);
export const ErrDestinationReadUnsupported = new HttpError(
  "destination does not support reading documents",
  400,
);
export const ErrNotRunning = new HttpError("sync log is not running", 409);
export const ErrNotConfigured = new HttpError(
  "openai is not configured (set OPENAI_API_KEY)",
  503,
);
export const ErrAmbiguous = new HttpError(
  "multiple connections match name and type",
  400,
);

export function invalid(message: string): HttpError {
  return new HttpError(message, 400);
}

export function withCode(base: HttpError, message: string): HttpError {
  return new HttpError(message, base.status);
}

export function invalidJob(message: string): HttpError {
  return withCode(ErrInvalidJob, `invalid sync job: ${message}`);
}
