import { useRoute } from 'preact-iso';

export function IterPlaceholder() {
  const { params } = useRoute();
  return (
    <div class="p-8 text-neutral-300">
      <div class="text-xs text-neutral-500 mb-2">
        repo <span class="font-mono">{params.fp}</span> · run{' '}
        <span class="font-mono">{params.runId}</span>
      </div>
      <h2 class="text-lg font-semibold mb-1">
        Story <span class="font-mono">{params.story}</span> · iter{' '}
        <span class="font-mono">{params.iter}</span>
      </h2>
      <p class="text-sm text-neutral-500">
        Chat transcript view lands in RV-010.
      </p>
    </div>
  );
}
