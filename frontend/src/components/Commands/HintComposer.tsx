import { useState } from 'preact/hooks';
import { sendCommand } from '../../lib/commands';
import type { WorkerStatus } from '../../lib/live';

export function HintComposer({
  fp,
  workers,
}: {
  fp: string;
  workers: WorkerStatus[];
}) {
  const [workerId, setWorkerId] = useState<number | ''>(
    workers.length > 0 ? workers[0].id : '',
  );
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e: Event) {
    e.preventDefault();
    if (busy || workerId === '' || text.trim() === '') return;
    setBusy(true);
    try {
      await sendCommand(
        fp,
        'hint',
        { worker_id: workerId, text },
        {
          success: `Hint sent to worker #${workerId}`,
          errorPrefix: 'Hint failed',
        },
      );
      setText('');
    } catch {
      /* toast already fired */
    } finally {
      setBusy(false);
    }
  }

  if (workers.length === 0) {
    return (
      <div class="text-[11px] text-neutral-500 italic">
        No active workers — hints are delivered to a specific worker.
      </div>
    );
  }

  return (
    <form onSubmit={submit} class="flex flex-col gap-2">
      <label class="flex items-center gap-2 text-[10px] uppercase tracking-wider text-neutral-500">
        Worker
        <select
          value={workerId === '' ? '' : String(workerId)}
          onChange={(e) => {
            const v = (e.currentTarget as HTMLSelectElement).value;
            setWorkerId(v === '' ? '' : Number(v));
          }}
          class="bg-neutral-900 border border-neutral-700 rounded text-xs px-1 py-0.5 text-neutral-200 focus:outline-none focus:border-neutral-500 normal-case tracking-normal"
        >
          {workers.map((w) => (
            <option key={w.id} value={String(w.id)}>
              #{w.id} · {w.role} · {w.story_id || '—'}
            </option>
          ))}
        </select>
      </label>
      <textarea
        value={text}
        onInput={(e) => setText((e.currentTarget as HTMLTextAreaElement).value)}
        placeholder="Hint to prepend to the next iteration's prompt"
        rows={3}
        class="bg-neutral-900 border border-neutral-700 rounded text-xs p-2 text-neutral-200 placeholder:text-neutral-600 focus:outline-none focus:border-neutral-500 resize-y"
      />
      <button
        type="submit"
        disabled={busy || workerId === '' || text.trim() === ''}
        class="self-start px-3 py-1 rounded border text-xs uppercase tracking-wider bg-indigo-500/10 border-indigo-500/40 text-indigo-200 hover:bg-indigo-500/20 disabled:opacity-50 disabled:hover:bg-indigo-500/10"
      >
        {busy ? 'Sending…' : 'Send hint'}
      </button>
    </form>
  );
}
