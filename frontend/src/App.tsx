import { LocationProvider, Router, Route } from 'preact-iso';
import { Sidebar } from './components/Sidebar/Sidebar';
import { MainTopBar } from './components/MainTopBar';
import { Home } from './routes/Home';
import { RunRoute } from './routes/RunRoute';
import { IterRoute } from './routes/IterRoute';
import { SettingsRoute } from './routes/SettingsRoute';
import { RepoMetaRoute } from './routes/RepoMetaRoute';
import { ToastStack } from './components/Toast';

export function App() {
  return (
    <LocationProvider>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '260px minmax(0, 1fr)',
          gap: 18,
          padding: '18px 18px 18px 0',
          height: '100vh',
          position: 'relative',
          background: 'var(--page)',
        }}
      >
        <div style={{ height: 'calc(100vh - 36px)', minHeight: 0 }}>
          <Sidebar />
        </div>
        <main
          style={{
            minWidth: 0,
            height: 'calc(100vh - 36px)',
          }}
        >
          <section
            style={{
              background: 'var(--bg-card)',
              borderRadius: 14,
              overflow: 'hidden',
              height: '100%',
              boxShadow: 'var(--shadow-md)',
              minWidth: 0,
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            <MainTopBar />
            <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
              <Router>
                <Route path="/" component={Home} />
                <Route
                  path="/repos/:fp/runs/:runId/iter/:story/:iter"
                  component={IterRoute}
                />
                <Route path="/repos/:fp/runs/:runId" component={RunRoute} />
                <Route path="/repos/:fp/settings" component={SettingsRoute} />
                <Route path="/repos/:fp/meta" component={RepoMetaRoute} />
                <Route default component={Home} />
              </Router>
            </div>
          </section>
        </main>
      </div>
      <ToastStack />
    </LocationProvider>
  );
}
