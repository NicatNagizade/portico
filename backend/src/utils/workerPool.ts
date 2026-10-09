/** Runs at most `limit` tasks at once; after the first failure, new tasks are rejected. */
export class WorkerPool {
  private active = 0;
  private queue: Array<() => void> = [];
  private failure: unknown = null;
  private pending: Promise<void>[] = [];

  constructor(private limit: number) {}

  add(task: () => Promise<void>): void {
    this.pending.push(this.run(task).catch(() => undefined));
  }

  async wait(): Promise<void> {
    await Promise.all(this.pending);
    if (this.failure) throw this.failure;
  }

  private async run(task: () => Promise<void>): Promise<void> {
    if (this.active >= this.limit)
      await new Promise<void>((resolve) => this.queue.push(resolve));
    if (this.failure) {
      this.queue.shift()?.();
      return;
    }
    this.active++;
    try {
      await task();
    } catch (error) {
      this.failure ??= error;
    } finally {
      this.active--;
      this.queue.shift()?.();
    }
  }
}
