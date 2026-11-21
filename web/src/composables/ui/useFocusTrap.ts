import { ref, nextTick } from 'vue';

const FOCUSABLE_SELECTORS = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
  '[contenteditable]',
].join(', ');

export function useFocusTrap(containerRef: ReturnType<typeof ref<HTMLElement | null>>) {
  const firstFocusableEl = ref<HTMLElement | null>(null);
  const lastFocusableEl = ref<HTMLElement | null>(null);
  let previousActiveElement: HTMLElement | null = null;

  function getFocusableElements(container: HTMLElement): HTMLElement[] {
    return Array.from(container.querySelectorAll(FOCUSABLE_SELECTORS)) as HTMLElement[];
  }

  function setup() {
    if (!containerRef.value) return;
    const elements = getFocusableElements(containerRef.value);
    if (elements.length > 0) {
      firstFocusableEl.value = elements[0] ?? null;
      lastFocusableEl.value = elements[elements.length - 1] ?? null;
      nextTick(() => {
        firstFocusableEl.value?.focus();
      });
    }
  }

  function handleTabKey(e: KeyboardEvent) {
    if (e.key !== 'Tab') return;
    if (!firstFocusableEl.value || !lastFocusableEl.value) return;

    if (e.shiftKey) {
      if (document.activeElement === firstFocusableEl.value) {
        e.preventDefault();
        lastFocusableEl.value.focus();
      }
    } else {
      if (document.activeElement === lastFocusableEl.value) {
        e.preventDefault();
        firstFocusableEl.value.focus();
      }
    }
  }

  function saveActiveElement() {
    previousActiveElement = document.activeElement as HTMLElement;
  }

  function restoreActiveElement() {
    nextTick(() => {
      previousActiveElement?.focus();
      previousActiveElement = null;
    });
  }

  return {
    firstFocusableEl,
    lastFocusableEl,
    setupFocusTrap: setup,
    handleTabKey,
    saveActiveElement,
    restoreActiveElement,
  };
}

let lockCount = 0;

export function useScrollLock() {
  function lock() {
    lockCount++;
    document.body.style.overflow = 'hidden';
  }

  function unlock() {
    lockCount = Math.max(0, lockCount - 1);
    if (lockCount === 0) {
      document.body.style.overflow = '';
    }
  }

  function forceUnlock() {
    lockCount = 0;
    document.body.style.overflow = '';
  }

  return { lock, unlock, forceUnlock };
}
