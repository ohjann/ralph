import { toasts, dismissToast, type ToastTone } from '../lib/toast';

const TONE_STYLES: Record<ToastTone, string> = {
  success: 'bg-emerald-500/10 border-emerald-500/40 text-emerald-200',
  error: 'bg-red-500/10 border-red-500/40 text-red-200',
  info: 'bg-neutral-900 border-neutral-700 text-neutral-200',
  warn: 'bg-amber-500/10 border-amber-500/40 text-amber-200',
};

export function ToastStack() {
  const list = toasts.value;
  if (list.length === 0) return null;
  return (
    <div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
      {list.map((t) => (
        <div
          key={t.id}
          class={`rounded border px-3 py-2 text-xs shadow-lg ${TONE_STYLES[t.tone]}`}
        >
          <div class="flex items-start gap-2">
            <span class="flex-1 whitespace-pre-wrap break-words">{t.text}</span>
            <button
              type="button"
              onClick={() => dismissToast(t.id)}
              class="text-neutral-500 hover:text-neutral-200"
              aria-label="Dismiss"
            >
              ×
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
