import { signal } from '@preact/signals';

export type ToastTone = 'success' | 'error' | 'info' | 'warn';

export interface Toast {
  id: number;
  tone: ToastTone;
  text: string;
  createdAt: number;
}

export const toasts = signal<Toast[]>([]);

let nextId = 1;
const DEFAULT_TTL_MS = 5000;

export function pushToast(
  tone: ToastTone,
  text: string,
  ttlMs: number = DEFAULT_TTL_MS,
): number {
  const id = nextId++;
  const t: Toast = { id, tone, text, createdAt: Date.now() };
  toasts.value = [...toasts.value, t];
  if (ttlMs > 0) {
    setTimeout(() => dismissToast(id), ttlMs);
  }
  return id;
}

export function dismissToast(id: number) {
  toasts.value = toasts.value.filter((t) => t.id !== id);
}
