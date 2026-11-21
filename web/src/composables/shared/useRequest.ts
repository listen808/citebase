import { ref, shallowRef, readonly, onUnmounted, type Ref } from 'vue';

export interface UseRequestOptions<T, _P extends unknown[]> {
  immediate?: boolean;
  initialData?: T;
  onSuccess?: (data: T) => void;
  onError?: (error: Error) => void;
  resetOnExecute?: boolean;
  debounce?: number;
}

export interface UseRequestReturn<T, P extends unknown[]> {
  data: Ref<T | null>;
  loading: Ref<boolean>;
  error: Ref<Error | null>;
  execute: (...args: P) => Promise<T | null>;
  reset: () => void;
  abort: () => void;
}

export function useRequest<T, P extends unknown[] = []>(
  fn: (...args: P) => Promise<T>,
  options: UseRequestOptions<T, P> = {},
): UseRequestReturn<T, P> {
  const { immediate = false, initialData, onSuccess, onError, resetOnExecute = true, debounce: debounceMs } = options;

  const data = ref<T | null>(initialData ?? null) as Ref<T | null>;
  const loading = shallowRef(false);
  const error = shallowRef<Error | null>(null);
  
  let abortController: AbortController | null = null;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let requestId = 0;

  function abort() {
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  }

  async function execute(...args: P): Promise<T | null> {
    abort();
    
    const currentRequestId = ++requestId;
    abortController = new AbortController();
    
    if (resetOnExecute) {
      error.value = null;
    }

    const doExecute = async () => {
      loading.value = true;
      try {
        const result = await fn(...args);
        if (currentRequestId !== requestId || abortController?.signal.aborted) {
          return null;
        }
        data.value = result;
        onSuccess?.(result);
        return result;
      } catch (e) {
        if (currentRequestId !== requestId || abortController?.signal.aborted) {
          return null;
        }
        const err = e instanceof Error ? e : new Error(String(e));
        if (err.name === 'AbortError') {
          return null;
        }
        error.value = err;
        onError?.(err);
        return null;
      } finally {
        if (currentRequestId === requestId) {
          loading.value = false;
        }
      }
    };

    if (debounceMs && debounceMs > 0) {
      return new Promise((resolve) => {
        debounceTimer = setTimeout(async () => {
          const result = await doExecute();
          resolve(result);
        }, debounceMs);
      });
    }

    return doExecute();
  }

  function reset() {
    abort();
    data.value = initialData ?? null;
    loading.value = false;
    error.value = null;
  }

  onUnmounted(() => {
    abort();
  });

  if (immediate) {
    execute(...([] as unknown as P));
  }

  return {
    data,
    loading: readonly(loading) as Ref<boolean>,
    error: readonly(error) as Ref<Error | null>,
    execute,
    reset,
    abort,
  };
}
