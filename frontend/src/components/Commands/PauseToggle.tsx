import { useState } from 'preact/hooks';
import { sendCommand } from '../../lib/commands';

export function PauseToggle({
  fp,
  paused,
}: {
  fp: string;
  paused: boolean;
}) {
  const [busy, setBusy] = useState(false);

  async function handleClick() {
    if (busy) return;
    setBusy(true);
    try {
      if (paused) {
        await sendCommand(fp, 'resume', undefined, {
          success: 'Resumed',
          errorPrefix: 'Resume failed',
        });
      } else {
        await sendCommand(fp, 'pause', undefined, {
          success: 'Paused',
          errorPrefix: 'Pause failed',
        });
      }
    } catch {
      /* toast already fired */
    } finally {
      setBusy(false);
    }
  }

  const cls = paused
    ? 'bg-emerald-500/10 border-emerald-500/40 text-emerald-200 hover:bg-emerald-500/20'
    : 'bg-neutral-800 border-neutral-700 text-neutral-200 hover:bg-neutral-700';

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={busy}
      class={`px-3 py-1 rounded border text-xs uppercase tracking-wider transition ${cls} disabled:opacity-50`}
    >
      {paused ? '▶ Resume' : '⏸ Pause'}
    </button>
  );
}
