import { logError } from './error-logger';

/**
 * 并发执行任务，限制同时运行的数量。
 * 单个任务失败不会中断其他任务，失败结果记日志、对应位置留空。
 */
export async function parallelLimit<T>(tasks: (() => Promise<T>)[], limit: number): Promise<T[]> {
  const results: T[] = new Array(tasks.length);
  let nextIndex = 0;
  async function worker() {
    while (nextIndex < tasks.length) {
      const i = nextIndex++;
      const task = tasks[i];
      if (task) {
        try {
          results[i] = await task();
        } catch (e) {
          logError({ module: 'parallelLimit', operation: 'task' }, e);
        }
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, tasks.length) }, () => worker()));
  return results;
}
