export function Home() {
  return (
    <div style={{ padding: '22px 28px 80px', minHeight: '100%' }}>
      <div style={{ maxWidth: 640, margin: '80px auto 0', padding: '0 24px' }}>
        <div
          style={{
            fontSize: 13,
            color: 'var(--fg-faint)',
            fontFamily: 'var(--font-mono)',
            letterSpacing: '0.04em',
            textTransform: 'uppercase',
            marginBottom: 14,
          }}
        >
          Ralph Viewer · localhost
        </div>
        <h1
          style={{
            fontSize: 28,
            fontWeight: 600,
            letterSpacing: '-0.02em',
            lineHeight: 1.2,
            margin: '0 0 14px',
            color: 'var(--fg)',
          }}
        >
          Pick a repo to see its runs.
        </h1>
        <p
          style={{
            fontSize: 15,
            color: 'var(--fg-muted)',
            lineHeight: 1.55,
            margin: 0,
          }}
        >
          The sidebar lists every repository Ralph has run against on this
          machine. Expand one to browse its history, grouped by{' '}
          <em>daemon</em>, <em>ad-hoc</em>, <em>retro</em>, and{' '}
          <em>memory</em> runs. Live daemons are marked with a pulsing dot.
        </p>
        <div
          style={{
            marginTop: 28,
            display: 'flex',
            flexDirection: 'column',
            gap: 10,
          }}
        >
          {[
            'Green dot = daemon answering on its socket right now.',
            'Runs are sorted most-recent first within each kind.',
            'Click an iteration badge in a run summary to view its transcript.',
          ].map((text) => (
            <div
              key={text}
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 10,
                fontSize: 13.5,
                color: 'var(--fg-muted)',
              }}
            >
              <span
                style={{ color: 'var(--accent)', marginTop: 2, fontSize: 13 }}
              >
                ›
              </span>
              <span>{text}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
