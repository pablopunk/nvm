type PidQueues = Map<number, Promise<void>>;

function ignoreSettled() {
  return;
}

function releaseQueueIfIdle(
  queues: PidQueues,
  pid: number,
  tail: Promise<void>,
) {
  if (queues.get(pid) === tail) {
    queues.delete(pid);
  }
}

export function enqueueForPid<Result>(
  queues: PidQueues,
  pid: number,
  task: () => Promise<Result>,
): Promise<Result> {
  const run = (queues.get(pid) ?? Promise.resolve()).then(task);
  const tail = run.then(ignoreSettled, ignoreSettled);
  queues.set(pid, tail);
  tail.then(() => releaseQueueIfIdle(queues, pid, tail));
  return run;
}
