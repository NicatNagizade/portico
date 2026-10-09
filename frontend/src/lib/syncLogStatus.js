export const SyncLogStatus = {
  Running: 'running',
  Success: 'success',
  Failed: 'failed',
  Stopped: 'stopped',
}

export const SYNC_LOG_STATUS_OPTIONS = [
  { value: SyncLogStatus.Running, label: 'Running' },
  { value: SyncLogStatus.Success, label: 'Success' },
  { value: SyncLogStatus.Failed, label: 'Failed' },
  { value: SyncLogStatus.Stopped, label: 'Stopped' },
]

export function isSyncLogRunning(status) {
  return status === SyncLogStatus.Running
}
