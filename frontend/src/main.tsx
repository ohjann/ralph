import { render } from 'preact';
import { App } from './App';
import { installTheme } from './lib/theme';
import { installTweaks } from './lib/tweaks';
import './styles.css';

installTheme();
installTweaks();

const token = new URLSearchParams(location.search).get('token');
if (token) {
  sessionStorage.setItem('ralph.token', token);
  // Strip token from URL so it does not leak into history/referrer.
  const url = new URL(location.href);
  url.searchParams.delete('token');
  history.replaceState(null, '', url.toString());
  // Swap cookie-style token → header via /api/bootstrap so future requests
  // can pick it up from sessionStorage.
  fetch('/api/bootstrap', { headers: { 'X-Ralph-Token': token } }).catch(() => {});
}

const root = document.getElementById('app');
if (root) render(<App />, root);
