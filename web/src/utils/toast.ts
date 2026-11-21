import { useUIStore } from '@/stores/app/ui';

export interface Toast {
  id: number;
  type: 'success' | 'error' | 'warning' | 'info';
  message: string;
  duration?: number;
}

export function useToast() {
  const ui = useUIStore();
  
  const addToast = (type: Toast['type'], message: string, duration?: number) => {
    ui.showToast(message, type, duration);
    return Date.now();
  };
  
  const removeToast = (id: number) => {
    ui.removeToast(id);
  };
  
  const success = (message: string, duration?: number) => addToast('success', message, duration);
  const error = (message: string, duration?: number) => addToast('error', message, duration);
  const warning = (message: string, duration?: number) => addToast('warning', message, duration);
  const info = (message: string, duration?: number) => addToast('info', message, duration);
  
  return { toasts: ui.toasts, addToast, removeToast, success, error, warning, info };
}