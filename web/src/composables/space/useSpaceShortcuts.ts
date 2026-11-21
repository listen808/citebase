import { shallowRef, onMounted, onUnmounted } from 'vue';
import { useRouter } from 'vue-router';
import { useDocumentStore } from '@/stores/workspace/document';
import { useUIStore } from '@/stores/app/ui';
import { useDocActions } from '@/composables/space/useDocActions';
import type { ComputedRef } from 'vue';
import i18n from '@/i18n';

const { t } = i18n.global;

export function useSpaceShortcuts(
  spaceId: ComputedRef<number>,
  openShareModal: () => void,
  openHistoryModal: () => void,
) {
  const router = useRouter();
  const docStore = useDocumentStore();
  const ui = useUIStore();
  const { buildShareUrl } = useDocActions();

  const showShortcutHelp = shallowRef(false);

  const shortcutList = [
    { key: 'Ctrl+E', desc: t('tsUi.k37') },
    { key: 'Ctrl+S', desc: t('tsUi.k38') },
    { key: 'Ctrl+Shift+B', desc: t('tsUi.k39') },
    { key: 'Ctrl+Shift+H', desc: t('tsUi.k40') },
    { key: 'Ctrl+Shift+S', desc: t('tsUi.k41') },
    { key: 'Ctrl+Shift+E', desc: t('tsUi.k42') },
    { key: 'Ctrl+/', desc: t('tsUi.k43') },
  ];

  const onCollect = shallowRef<() => void>(() => {});
  const onExport = shallowRef<() => void>(() => {});

  const handleKeyDown = (e: KeyboardEvent) => {
    const target = e.target as HTMLElement;
    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) return;
    if (document.activeElement?.closest('.monaco-editor') || document.activeElement?.closest('[role="textbox"]')) return;
    if (e.ctrlKey || e.metaKey) {
      if (e.shiftKey) {
        switch (e.key.toLowerCase()) {
          case 's':
            e.preventDefault();
            if (docStore.currentDocument) {
              buildShareUrl(spaceId.value, docStore.currentDocument.id);
              openShareModal();
            }
            break;
          case 'e':
            e.preventDefault();
            onExport.value();
            break;
          case 'b':
            // 收藏/取消收藏（原 Ctrl+B，改为 Ctrl+Shift+B 避免与编辑器粗体冲突）
            e.preventDefault();
            onCollect.value();
            break;
          case 'h':
            // 版本历史（原 Ctrl+H，改为 Ctrl+Shift+H 避免与编辑器标题冲突）
            e.preventDefault();
            if (docStore.currentDocument) openHistoryModal();
            break;
        }
      } else {
        switch (e.key.toLowerCase()) {
          case 'e':
            e.preventDefault();
            if (docStore.currentDocument?.id) {
              router.push(`/space/${spaceId.value}/doc/${docStore.currentDocument.id}/edit`);
            }
            break;
          case 's':
            e.preventDefault();
            ui.toggleSidebar();
            break;
          case '/':
            e.preventDefault();
            showShortcutHelp.value = true;
            break;
        }
      }
    }
  };

  onMounted(() => {
    document.addEventListener('keydown', handleKeyDown);
  });

  onUnmounted(() => {
    document.removeEventListener('keydown', handleKeyDown);
  });

  return {
    showShortcutHelp,
    shortcutList,
    onCollect,
    onExport,
  };
}
