export const SyncLogStatus = {
  Running: 'running',
  Success: 'success',
  Failed: 'failed',
  Stopped: 'stopped',
}

export function isSyncLogRunning(status) {
  return status === SyncLogStatus.Running
}
