import { useRouter, useRoute } from 'vue-router';
import { useHistoryStore } from '@/stores/workspace/history';

export function useBackNavigation(options?: {
  defaultPath?: string;
  onRestoreState?: (state: { expandedNodes?: number[]; sidebarCollapsed?: boolean; scrollTop?: number }) => void;
}) {
  const router = useRouter();
  const route = useRoute();
  const history = useHistoryStore();

  function goBack(fallbackPath?: string): void {
    if (history.canGoBack()) {
      const prevIndex = history.currentIndex - 1;
      const prevState = prevIndex >= 0 && prevIndex < history.stack.length ? history.stack[prevIndex] : null;
      if (prevState) {
        history.goBack();

        if (options?.onRestoreState) {
          options.onRestoreState({
            expandedNodes: prevState.expandedNodes,
            sidebarCollapsed: prevState.sidebarCollapsed,
            scrollTop: prevState.scrollTop,
          });
        }

        router.push(prevState.path);

        requestAnimationFrame(() => {
          window.scrollTo({ top: prevState.scrollTop, behavior: 'smooth' });
        });
        return;
      }
    }

    const defaultPath = fallbackPath || options?.defaultPath || getSmartDefaultPath();
    router.push(defaultPath);
  }

  function goBackTo(path: string): void {
    router.push(path);
  }

  function getSmartDefaultPath(): string {
    const path = route.path;

    if (path.startsWith('/admin')) {
      return '/admin';
    }

    const docEditMatch = path.match(/\/space\/(\d+)\/doc\/(\d+)\/edit/);
    if (docEditMatch) {
      const spaceId = docEditMatch[1];
      const docId = docEditMatch[2];
      return `/space/${spaceId}/doc/${docId}`;
    }

    const docMatch = path.match(/\/space\/(\d+)\/doc\/(\d+)/);
    if (docMatch) {
      const spaceId = docMatch[1];
      return `/space/${spaceId}`;
    }

    if (path.startsWith('/space/')) {
      return '/';
    }

    return '/';
  }

  return {
    goBack,
    goBackTo,
  };
}
