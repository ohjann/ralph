import { useState } from 'preact/hooks';
import { sendCommand } from '../../lib/commands';

export function QuitButton({ fp }: { fp: string }) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);

  async function doQuit() {
    if (busy) return;
    setBusy(true);
    try {
      await sendCommand(fp, 'quit', undefined, {
        success: 'Daemon shutting down',
        errorPrefix: 'Quit failed',
      });
    } catch {
      /* toast already fired */
    } finally {
      setBusy(false);
      setConfirming(false);
    }
  }

  if (!confirming) {
    return (
      <button
        type="button"
        onClick={() => setConfirming(true)}
        class="px-3 py-1 rounded border text-xs uppercase tracking-wider bg-neutral-800 border-neutral-700 text-neutral-300 hover:bg-red-500/10 hover:border-red-500/40 hover:text-red-200 transition"
      >
        Quit
      </button>
    );
  }

  return (
    <div class="flex items-center gap-1">
      <span class="text-xs text-neutral-400">Quit daemon?</span>
      <button
        type="button"
        onClick={doQuit}
        disabled={busy}
        class="px-2 py-0.5 rounded border text-xs bg-red-500/10 border-red-500/40 text-red-200 hover:bg-red-500/20 disabled:opacity-50"
      >
        {busy ? '…' : 'Confirm'}
      </button>
      <button
        type="button"
        onClick={() => setConfirming(false)}
        disabled={busy}
        class="px-2 py-0.5 rounded border text-xs border-neutral-700 text-neutral-400 hover:bg-neutral-800"
      >
        Cancel
      </button>
    </div>
  );
}
