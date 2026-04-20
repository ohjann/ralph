import { useState } from 'preact/hooks';
import { sendCommand } from '../../lib/commands';

// Feature-flagged: hidden until the daemon emits a clarify_pending event.
// Pass `enabled={false}` and this renders nothing.
export function ClarifyPrompt({
  fp,
  enabled,
}: {
  fp: string;
  enabled: boolean;
}) {
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);

  if (!enabled) return null;

  async function submit(e: Event) {
    e.preventDefault();
    if (busy || text.trim() === '') return;
    setBusy(true);
    try {
      await sendCommand(
        fp,
        'clarify',
        { text },
        {
          success: 'Clarification sent',
          errorPrefix: 'Clarify failed',
        },
      );
      setText('');
    } catch {
      /* toast already fired */
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} class="flex flex-col gap-2 border border-amber-500/30 bg-amber-500/5 rounded p-2">
      <div class="text-[10px] uppercase tracking-wider text-amber-300">
        Clarification requested
      </div>
      <textarea
        value={text}
        onInput={(e) => setText((e.currentTarget as HTMLTextAreaElement).value)}
        rows={3}
        class="bg-neutral-900 border border-neutral-700 rounded text-xs p-2 text-neutral-200 focus:outline-none focus:border-amber-500/40 resize-y"
      />
      <button
        type="submit"
        disabled={busy || text.trim() === ''}
        class="self-start px-3 py-1 rounded border text-xs uppercase tracking-wider bg-amber-500/10 border-amber-500/40 text-amber-200 hover:bg-amber-500/20 disabled:opacity-50"
      >
        {busy ? 'Sending…' : 'Send clarification'}
      </button>
    </form>
  );
}
