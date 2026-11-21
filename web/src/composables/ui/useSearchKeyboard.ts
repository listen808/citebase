import { onMounted, onUnmounted, type Ref } from 'vue';

interface UseSearchKeyboardOptions {
  resultsCount: Ref<number>;
  selectedIndex: Ref<number>;
  onSelect: (index: number) => void;
  onClose: () => void;
  enabled: Ref<boolean>;
}

export function useSearchKeyboard({
  resultsCount,
  selectedIndex,
  onSelect,
  onClose,
  enabled,
}: UseSearchKeyboardOptions) {
  function handleKeydown(e: KeyboardEvent) {
    if (!enabled.value) return;

    const target = e.target as HTMLElement;
    const isInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable;
    if (isInput) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        if (resultsCount.value > 0) {
          const nextIndex = selectedIndex.value < resultsCount.value - 1
            ? selectedIndex.value + 1
            : 0;
          selectedIndex.value = nextIndex;
          scrollToResult(nextIndex);
        }
        break;

      case 'ArrowUp':
        e.preventDefault();
        if (resultsCount.value > 0) {
          const prevIndex = selectedIndex.value > 0
            ? selectedIndex.value - 1
            : resultsCount.value - 1;
          selectedIndex.value = prevIndex;
          scrollToResult(prevIndex);
        }
        break;

      case 'Enter':
        e.preventDefault();
        if (selectedIndex.value >= 0 && selectedIndex.value < resultsCount.value) {
          onSelect(selectedIndex.value);
        }
        break;

      case 'Escape':
        e.preventDefault();
        onClose();
        break;
    }
  }

  function scrollToResult(index: number) {
    requestAnimationFrame(() => {
      const element = document.querySelector(`[data-result-index="${index}"]`);
      if (element) {
        element.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    });
  }

  onMounted(() => {
    document.addEventListener('keydown', handleKeydown);
  });

  onUnmounted(() => {
    document.removeEventListener('keydown', handleKeydown);
  });
}
