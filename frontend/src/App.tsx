import { LocationProvider, Router, Route } from 'preact-iso';
import { Sidebar } from './components/Sidebar/Sidebar';
import { Home } from './routes/Home';
import { RunRoute } from './routes/RunRoute';
import { IterRoute } from './routes/IterRoute';
import { SettingsRoute } from './routes/SettingsRoute';
import { RepoMetaRoute } from './routes/RepoMetaRoute';
import { ToastStack } from './components/Toast';

export function App() {
  return (
    <LocationProvider>
      <div class="flex h-screen bg-neutral-950 text-neutral-100">
        <Sidebar />
        <main class="flex-1 overflow-y-auto">
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
        </main>
      </div>
      <ToastStack />
    </LocationProvider>
  );
}
