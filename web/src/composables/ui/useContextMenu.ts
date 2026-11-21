import { ref, shallowRef } from 'vue';
import { useClickOutsideClose } from './useClickOutsideClose';
import { useEscapeClose } from './useEscapeClose';

export interface ContextMenuDoc {
  id: number;
  name: string;
  type: number;
  space_id: number;
  parent_id?: number;
}

export function useContextMenu() {
  const visible = shallowRef(false);
  const position = ref({ x: 0, y: 0 });
  const doc = shallowRef<ContextMenuDoc | null>(null);

  function show(event: MouseEvent, menuDoc: ContextMenuDoc) {
    event.preventDefault();
    doc.value = menuDoc;

    const menuWidth = 160;
    const menuHeight = 200;
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;

    let x = event.clientX;
    let y = event.clientY;

    if (x + menuWidth > viewportWidth) {
      x = viewportWidth - menuWidth - 8;
    }
    if (x < 8) {
      x = 8;
    }
    if (y + menuHeight > viewportHeight) {
      y = viewportHeight - menuHeight - 8;
    }
    if (y < 8) {
      y = 8;
    }

    position.value = { x, y };
    visible.value = true;
  }

  function hide() {
    visible.value = false;
    doc.value = null;
  }

  // 点别处 / Esc 两样都走全站共用的那两个 composable：
  // 共用版按"自己人元素"判，这里没有元素引用，只有 .context-menu 这个类名约定，
  // 所以 own() 现查一次 DOM —— 面板是 v-if 出来的，取引用会取到 null。
  useClickOutsideClose(
    () => visible.value,
    () => hide(),
    () => [...document.querySelectorAll<HTMLElement>('.context-menu')],
  );
  useEscapeClose(() => visible.value, () => hide());

  return { visible, position, doc, show, hide };
}
