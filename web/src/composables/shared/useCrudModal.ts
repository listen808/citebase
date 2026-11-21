import { shallowRef } from 'vue';

export function useCrudModal<T extends Record<string, unknown>>(defaults: T) {
  const show = shallowRef(false);
  const isEdit = shallowRef(false);
  const editing = shallowRef<T | null>(null);
  const submitting = shallowRef(false);

  function openCreate() {
    isEdit.value = false;
    editing.value = { ...defaults };
    show.value = true;
  }

  function openEdit(item: T) {
    isEdit.value = true;
    editing.value = structuredClone(item);
    show.value = true;
  }

  function close() {
    show.value = false;
    editing.value = null;
    submitting.value = false;
  }

  return { show, isEdit, editing, submitting, openCreate, openEdit, close };
}
