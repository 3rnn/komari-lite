import assert from 'node:assert/strict';
import test from 'node:test';
import { register } from 'node:module';
import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';

const root = new URL('../src/', import.meta.url).href;
register('data:text/javascript,' + encodeURIComponent(`
  import { existsSync } from 'node:fs';
  import { fileURLToPath } from 'node:url';
  import { transform } from ${JSON.stringify(import.meta.resolve('esbuild'))};
  export async function resolve(specifier, context, nextResolve) {
    if (!specifier.startsWith('@/')) return nextResolve(specifier, context);
    const url = ${JSON.stringify(root)} + specifier.slice(2);
    const target = ['.ts', '.tsx'].find(ext => existsSync(fileURLToPath(url + ext)));
    return nextResolve(url + target, context);
  }
  export async function load(url, context, nextLoad) {
    if (!url.endsWith('.tsx')) return nextLoad(url, context);
    const { source } = await nextLoad(url, { ...context, format: 'module' });
    const result = await transform(String(source), { loader: 'tsx', format: 'esm', jsx: 'automatic' });
    return { format: 'module', source: result.code, shortCircuit: true };
  }
`));

const { AccountProvider, useAccount } = await import('../src/contexts/AccountContext.tsx');
const { NodeDetailsProvider, useNodeDetails } = await import('../src/contexts/NodeDetailsContext.tsx');
const { resolveAdminAuthView } = await import('../src/utils/adminAuth.ts');
const { requestDashboardAlertItems } = await import('../src/utils/adminAlertFilters.ts');
const { requestDashboard, getDashboardSnapshot } = await import('../src/utils/dashboardApi.ts');
const { fetchDashboardSettings, getDashboardSettingsSnapshot } = await import('../src/hooks/useDashboardSettings.ts');
const { createLoadAlertBootstrapResource } = await import('../src/utils/loadAlertBootstrap.ts');
const { revokeAdminSession } = await import('../src/utils/adminRevocation.ts');
const { SettingsProvider, useSettings } = await import('../src/lib/api.ts');

globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const authenticated = { logged_in: true, uuid: 'operator', username: 'operator' };
const json = value => new Response(JSON.stringify(value));
const tick = () => new Promise(resolve => setTimeout(resolve, 0));

function Probe({ history }) {
  const state = useAccount();
  const nodes = useNodeDetails();
  history.current = { ...state, ...nodes, accountRefresh: state.refresh, nodeRefresh: nodes.refresh, view: resolveAdminAuthView(state) };
  return React.createElement('main', null,
    history.current.view === 'login' ? 'login view' :
      history.current.view === 'admin' && history.route === '/admin/servers'
        ? nodes.nodeDetail.map(n => n.name).join(',')
        : history.current.view);
}

function app(history) {
  return React.createElement(AccountProvider, null,
    React.createElement(NodeDetailsProvider, null, React.createElement(Probe, { history })));
}

async function mount(history) {
  let renderer;
  await act(async () => {
    renderer = TestRenderer.create(app(history));
    await tick();
  });
  return renderer;
}

test('alert 401 transitions the SPA to login and unfiltered servers cannot show old nodes', async t => {
  t.mock.method(globalThis, 'fetch', async url => {
    if (url === '/api/me') return json(authenticated);
    if (url === '/api/admin/client/list') return json([{ uuid: 'old', name: 'private-node', deployment_status: '' }]);
    if (String(url).includes('/dashboard/alerts?')) return new Response('{"message":"expired"}', { status: 401 });
    if (url === '/api/admin/settings/dashboard') return json({ status: 'success', data: {} });
    return json({ private: true });
  });
  const history = { current: null, route: '/admin/dashboard' };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  assert.equal(history.current.view, 'admin');
  assert.deepEqual(history.current.nodeDetail.map(node => node.name), ['private-node']);
  await requestDashboard(['servers'], 5, 'operator');
  await fetchDashboardSettings({ accountKey: 'operator', force: true });
  assert.ok(getDashboardSettingsSnapshot('operator'));
  await act(async () => {
    await assert.rejects(requestDashboardAlertItems('offline', undefined, 'operator'), { status: 401 });
    history.route = '/admin/servers'; // navigate without an alert filter in the same SPA
    renderer.update(app(history));
  });
  assert.equal(history.current.view, 'login');
  assert.equal(history.current.account.uuid, '');
  assert.deepEqual(history.current.nodeDetail, []);
  assert.equal(renderer.toJSON().children.join(''), 'login view');
  assert.equal(getDashboardSnapshot('servers:5', 'operator'), null);
  assert.equal(getDashboardSettingsSnapshot('operator'), null);
});

test('load-alert admin bootstrap clears on revocation and discards a late old result', async () => {
  let finishOld;
  let calls = 0;
  const resource = createLoadAlertBootstrapResource(() => ++calls === 1
    ? new Promise(resolve => { finishOld = resolve; })
    : Promise.resolve([{ name: 'fresh' }]));
  let old;
  try { resource.read('operator'); } catch (reason) { old = reason; }
  revokeAdminSession();
  let fresh;
  try { resource.read('operator'); } catch (reason) { fresh = reason; }
  assert.notEqual(fresh, old);
  await fresh;
  finishOld([{ name: 'stale' }]);
  await old;
  assert.equal(resource.read('operator').data[0].name, 'fresh');
  revokeAdminSession();
  assert.throws(() => resource.read('operator'), Promise);
});

test('load-alert bootstrap 401 revokes mounted account before its body resolves', async t => {
  let bodyRead = false;
  let bootstrapCalls = 0;
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me'
    ? json(authenticated) : url === '/api/admin/client/list' ? json([])
    : url === '/api/admin/notification/load' && ++bootstrapCalls === 1
      ? { ok: false, status: 401, json: () => { bodyRead = true; return new Promise(() => {}); } }
      : json({ data: {} }));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  const resource = createLoadAlertBootstrapResource();
  let pending;
  await act(async () => {
    try { resource.read('operator'); } catch (promise) { pending = promise; }
    await tick();
  });
  assert.equal(history.current.view, 'login');
  assert.equal(bodyRead, false);
  await pending;
  assert.throws(() => resource.read('operator'), Promise);
});

test('notification sender and test-send endpoints revoke mounted account on 401', async t => {
  const { adminNodeRequest } = await import('../src/utils/adminNodeRequest.ts');
  const { readFileSync } = await import('node:fs');
  const page = readFileSync(new URL('../src/pages/admin/settings/notification.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(page, /\bfetch\s*\(/);
  assert.match(page, /adminNodeRequest\(/);
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me'
    ? json(authenticated) : url === '/api/admin/client/list' ? json([])
      : new Response('{}', { status: 401 }));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => {
    await assert.rejects(adminNodeRequest('/api/admin/settings/message-sender', {}), { status: 401 });
  });
  assert.equal(history.current.view, 'login');
});

test('node response body cannot deliver old private data after session revocation', async t => {
  const { adminNodeRequest } = await import('../src/utils/adminNodeRequest.ts');
  let finishBody;
  t.mock.method(globalThis, 'fetch', async () => ({ ok: true, status: 200,
    json: () => new Promise(resolve => { finishBody = resolve; }) }));
  const response = await adminNodeRequest('/api/admin/client/list', {});
  const pending = response.json();
  revokeAdminSession();
  finishBody({ private: 'old-session' });
  await assert.rejects(pending, { name: 'AbortError' });
});

test('logout revokes mounted private data even when document navigation fails', async t => {
  const { readFileSync } = await import('node:fs');
  const bar = readFileSync(new URL('../src/components/admin/AdminPanelBar.tsx', import.meta.url), 'utf8');
  assert.match(bar, /logoutAdminSession\(/);
  const { logoutAdminSession } = await import('../src/utils/adminRevocation.ts');
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me' ? json(authenticated)
    : url === '/api/admin/client/list' ? json([{ uuid: 'private', name: 'private-node', deployment_status: '' }])
      : url === '/api/logout' ? new Response('', { status: 200 }) : json({ private: true }));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await requestDashboard(['servers'], 5, 'operator');
  await act(async () => { await logoutAdminSession(() => { throw new Error('navigation blocked'); }); });
  assert.equal(history.current.view, 'login');
  assert.deepEqual(history.current.nodeDetail, []);
  assert.equal(getDashboardSnapshot('servers:5', 'operator'), null);
});

test('settings request from old session is never reused by the replacement admin view', async t => {
  let finishOld;
  let calls = 0;
  t.mock.method(globalThis, 'fetch', () => ++calls === 1
    ? new Promise(resolve => { finishOld = resolve; })
    : Promise.resolve(json({ data: { sitename: 'fresh-settings' } })));
  let visible;
  function SettingsProbe() {
    visible = useSettings();
    return React.createElement('div', null, visible.settings.sitename);
  }
  const element = () => React.createElement(SettingsProvider, null, React.createElement(SettingsProbe));
  let first;
  await act(async () => { first = TestRenderer.create(element()); await tick(); });
  await act(async () => { revokeAdminSession(); first.unmount(); });
  let second;
  await act(async () => { second = TestRenderer.create(element()); await tick(); });
  t.after(() => { act(() => second.unmount()); });
  assert.equal(visible.settings.sitename, 'fresh-settings');
  finishOld(json({ data: { sitename: 'old-private-settings' } }));
  await act(tick);
  assert.equal(visible.settings.sitename, 'fresh-settings');
  assert.equal(calls, 2);
});

test('late dashboard settings cannot overwrite a new session snapshot', async t => {
  let finishOld;
  let calls = 0;
  t.mock.method(globalThis, 'fetch', () => ++calls === 1
    ? new Promise(resolve => { finishOld = resolve; })
    : Promise.resolve(json({ status: 'success', data: { ranking_limit: 20 } })));
  const old = fetchDashboardSettings({ accountKey: 'operator', force: true });
  revokeAdminSession();
  const fresh = await fetchDashboardSettings({ accountKey: 'operator', force: true });
  finishOld(json({ status: 'success', data: { ranking_limit: 50 } }));
  await assert.rejects(old, { name: 'AbortError' });
  assert.equal(getDashboardSettingsSnapshot('operator').ranking_limit, fresh.ranking_limit);
  assert.equal(calls, 2);
});

test('late account and node responses from the revoked session cannot restore private data', async t => {
  let finishAccount, finishNodes;
  let accountCalls = 0;
  let nodeCalls = 0;
  t.mock.method(globalThis, 'fetch', url => {
    if (url === '/api/me') return Promise.resolve(++accountCalls === 1 ? json(authenticated) : new Promise(resolve => { finishAccount = resolve; }));
    if (url === '/api/admin/client/list') return ++nodeCalls === 1
      ? Promise.resolve(json([{ uuid: 'old', name: 'private-node', deployment_status: '' }]))
      : new Promise(resolve => { finishNodes = resolve; });
    if (String(url).includes('/dashboard/alerts?')) return Promise.resolve(new Response('{}', { status: 401 }));
    return Promise.resolve(json({}));
  });
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => { void history.current.accountRefresh(); history.current.nodeRefresh(); await tick(); });
  assert.equal(typeof finishAccount, 'function');
  assert.equal(typeof finishNodes, 'function');
  // Both requests began under the old session and finish only after its revocation.
  await act(async () => { await assert.rejects(requestDashboardAlertItems('resource'), { status: 401 }); });
  await act(async () => { finishAccount(json(authenticated)); if (finishNodes) finishNodes(json([{ uuid: 'late', name: 'late-private', deployment_status: '' }])); await tick(); });
  assert.equal(history.current.view, 'login');
  assert.equal(history.current.account.uuid, '');
  assert.deepEqual(history.current.nodeDetail, []);
});

test('successful account refresh to guest clears caches and displayed nodes', async t => {
  let guest = false;
  t.mock.method(globalThis, 'fetch', async url => {
    if (url === '/api/me') return json(guest ? { logged_in: false, uuid: '', username: '' } : authenticated);
    if (url === '/api/admin/client/list') return json([{ uuid: 'old', name: 'private-node', deployment_status: '' }]);
    if (url === '/api/admin/settings/dashboard') return json({ status: 'success', data: {} });
    return json({ data: {} });
  });
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await requestDashboard(['servers'], 5, 'operator');
  await fetchDashboardSettings({ accountKey: 'operator', force: true });
  guest = true;
  await act(async () => { await history.current.accountRefresh(); });
  assert.equal(history.current.view, 'login');
  assert.deepEqual(history.current.nodeDetail, []);
  assert.equal(getDashboardSnapshot('servers:5', 'operator'), null);
  assert.equal(getDashboardSettingsSnapshot('operator'), null);
});

test('repeated guest refresh does not repeatedly invalidate the same identity', async t => {
  const { getAdminRevocationGeneration } = await import('../src/utils/adminRevocation.ts');
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me'
    ? json({ logged_in: false, uuid: '', username: '' }) : json([]));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  const generation = getAdminRevocationGeneration();
  await act(async () => { await history.current.accountRefresh(); });
  assert.equal(getAdminRevocationGeneration(), generation);
});

test('account A to B rejects delayed A nodes and dashboard data', async t => {
  let finishNodes, finishDashboard;
  let nextAccount = authenticated;
  let nodeCalls = 0;
  t.mock.method(globalThis, 'fetch', url => {
    if (url === '/api/me') return Promise.resolve(json(nextAccount));
    if (url === '/api/admin/client/list') return ++nodeCalls === 2
      ? new Promise(resolve => { finishNodes = resolve; })
      : Promise.resolve(json([{ uuid: 'fresh', name: nodeCalls === 1 ? 'A-node' : 'B-node', deployment_status: nodeCalls === 1 ? 'failed' : 'applied' }]));
    if (String(url).startsWith('/api/admin/dashboard')) return new Promise(resolve => { finishDashboard = resolve; });
    return Promise.resolve(json({ data: {} }));
  });
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => { history.current.nodeRefresh(); await tick(); });
  const oldDashboard = requestDashboard(['servers'], 5, 'operator');
  nextAccount = { ...authenticated, uuid: 'replacement', username: 'replacement' };
  await act(async () => { await history.current.accountRefresh(); await tick(); });
  assert.equal(nodeCalls, 3, 'one B load, not an effect-triggered duplicate');
  assert.deepEqual(history.current.nodeDetail.map(node => [node.name, node.deployment_status]), [['B-node', 'applied']]);
  finishNodes(json([{ uuid: 'stale', name: 'A-late', deployment_status: 'failed' }]));
  finishDashboard(json({ status: 'success', data: { servers: { private: 'A-late' } } }));
  await assert.rejects(oldDashboard, { name: 'AbortError' });
  await act(tick);
  assert.deepEqual(history.current.nodeDetail.map(node => [node.name, node.deployment_status]), [['B-node', 'applied']]);
  assert.equal(getDashboardSnapshot('servers:5', 'operator'), null);
});

test('/api/me 401 shows login rather than a retry error', async t => {
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me'
    ? new Response('{}', { status: 401 }) : json([]));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  assert.equal(history.current.view, 'login');
});

test('an obsolete /api/me 401 cannot revoke the newer successful account refresh', async t => {
  const { getAdminRevocationGeneration } = await import('../src/utils/adminRevocation.ts');
  let finishOld;
  let accountCalls = 0;
  t.mock.method(globalThis, 'fetch', url => url === '/api/me'
    ? Promise.resolve(++accountCalls === 1 ? json(authenticated) : accountCalls === 2
      ? new Promise(resolve => { finishOld = resolve; }) : json(authenticated))
    : Promise.resolve(json([])));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  let obsolete;
  await act(async () => { obsolete = history.current.accountRefresh(); await tick(); });
  await act(async () => { await history.current.accountRefresh(); });
  const generation = getAdminRevocationGeneration();
  await act(async () => { finishOld(new Response('{}', { status: 401 })); await obsolete; });
  assert.equal(getAdminRevocationGeneration(), generation);
  assert.equal(history.current.view, 'admin');
});

test('mounted dashboard settings discard an A response after account key changes', async t => {
  const { useDashboardSettings } = await import('../src/hooks/useDashboardSettings.ts');
  let finishOld;
  t.mock.method(globalThis, 'fetch', (_url) => finishOld
    ? Promise.resolve(json({ status: 'success', data: { preset: 'custom', modules: [{ id: 'server_status', enabled: true, span: 6 }], ranking_limit: 20 } }))
    : new Promise(resolve => { finishOld = resolve; }));
  let visible;
  function SettingsProbe({ accountKey }) {
    visible = useDashboardSettings(accountKey);
    return React.createElement('div', null, visible.settings.ranking_limit);
  }
  let renderer;
  await act(async () => { renderer = TestRenderer.create(React.createElement(SettingsProbe, { accountKey: 'A' })); await tick(); });
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => { renderer.update(React.createElement(SettingsProbe, { accountKey: 'B' })); await tick(); });
  finishOld(json({ status: 'success', data: { preset: 'custom', modules: [{ id: 'server_status', enabled: true, span: 6 }], ranking_limit: 15 } }));
  await act(tick);
  assert.equal(visible.settings.ranking_limit, 20);
});

test('alert items are hidden immediately when the account or route identity changes', async () => {
  const { visibleRouteAlertItems } = await import('../src/utils/adminAlertFilters.ts');
  const oldItems = [{ node_uuid: 'private-node' }];
  assert.deepEqual(visibleRouteAlertItems(oldItems, 'A:offline', 'A:billing'), []);
  assert.deepEqual(visibleRouteAlertItems(oldItems, 'A:offline', 'B:offline'), []);
  assert.deepEqual(visibleRouteAlertItems(oldItems, 'A:offline', 'A:offline'), oldItems);
});

test('alert response after route change ignores a fetch that did not honor abort', async t => {
  let finish;
  t.mock.method(globalThis, 'fetch', () => new Promise(resolve => { finish = resolve; }));
  const controller = new AbortController();
  const oldRoute = requestDashboardAlertItems('offline', controller.signal, 'operator');
  controller.abort(); // route switched to ?alert=billing while the old fetch was in flight
  finish(json({ items: [{ node_uuid: 'old-private-node' }] }));
  await assert.rejects(oldRoute, { name: 'AbortError' });
  const { getDashboardAlertItemsSnapshot } = await import('../src/utils/adminAlertFilters.ts');
  assert.equal(getDashboardAlertItemsSnapshot('offline', 'operator'), null);
});

test('node list 401 revokes the mounted account instead of displaying an error', async t => {
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me'
    ? json(authenticated) : url === '/api/admin/client/list'
      ? new Response('{}', { status: 401 }) : json({ data: {} }));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  assert.equal(history.current.view, 'login');
  assert.deepEqual(history.current.nodeDetail, []);
});

test('node POST 401 revokes mounted account and rejects a stale mutation', async t => {
  const { postAdminNode } = await import('../src/utils/adminNodeRequest.ts');
  t.mock.method(globalThis, 'fetch', async (url, options) => url === '/api/me'
    ? json(authenticated) : url === '/api/admin/client/list' ? json([])
      : options?.method === 'POST' ? new Response('{}', { status: 401 }) : json({ data: {} }));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => { await assert.rejects(postAdminNode('/api/admin/client/add', { name: 'x' }), { status: 401 }); });
  assert.equal(history.current.view, 'login');
});

test('invalid node rotation 2FA is not mistaken for a logged-out session', async t => {
  const { adminNodeRequest } = await import('../src/utils/adminNodeRequest.ts');
  t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ message: 'Invalid 2FA code' }), { status: 401 }));
  const response = await adminNodeRequest('/api/admin/client/token/rotate', { method: 'POST' }, true);
  assert.equal(response.status, 401);
});

test('settings POST 401 revokes mounted account before reading its body', async t => {
  const { updateSettings } = await import('../src/lib/api.ts');
  t.mock.method(globalThis, 'fetch', async (url, options) => url === '/api/me'
    ? json(authenticated) : url === '/api/admin/client/list' ? json([])
      : options?.method === 'POST' ? new Response('{}', { status: 401 }) : json({ data: {} }));
  const history = { current: null };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => { await assert.rejects(updateSettings({ sitename: 'x' })); });
  assert.equal(history.current.view, 'login');
});


test('legacy deployment profile 401 clears already mounted private server names', async t => {
  let profileCalls = 0;
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me' ? json(authenticated)
    : url === '/api/admin/client/list' ? json([{ uuid: 'private', name: 'private-node' }])
      : String(url).endsWith('/deployment-profile') && ++profileCalls === 1
        ? json({ saved: true, delivery_state: { status: 'saved' } })
        : new Response('{}', { status: 401 }));
  const history = { current: null, route: '/admin/servers' };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  assert.equal(renderer.toJSON().children.join(''), 'private-node');
  await act(async () => { history.current.nodeRefresh(); await tick(); });
  assert.equal(profileCalls, 2);
  assert.equal(history.current.view, 'login');
  assert.deepEqual(history.current.nodeDetail, []);
});

test('legacy profile body arriving after logout cannot restore the old node list', async t => {
  let finishBody;
  let profileCalls = 0;
  t.mock.method(globalThis, 'fetch', async url => url === '/api/me' ? json(authenticated)
    : url === '/api/admin/client/list' ? json([{ uuid: 'private', name: 'private-node' }])
      : String(url).endsWith('/deployment-profile') && ++profileCalls === 1
        ? json({ saved: true }) : { status: 200, ok: true, json: () => new Promise(resolve => { finishBody = resolve; }) });
  const history = { current: null, route: '/admin/servers' };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => { history.current.nodeRefresh(); await tick(); });
  assert.equal(typeof finishBody, 'function');
  await act(async () => { revokeAdminSession(); finishBody({ saved: true }); await tick(); });
  assert.equal(history.current.view, 'login');
  assert.deepEqual(history.current.nodeDetail, []);
});

test('failed navigation still completes server logout and old cookie cannot restore account', async t => {
  const { logoutAdminSession } = await import('../src/utils/adminRevocation.ts');
  let serverSession = true;
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    if (url === '/api/logout') {
      assert.equal(init?.method, 'POST');
      assert.equal(init?.credentials, 'same-origin');
      serverSession = false;
      return new Response('', { status: 200 });
    }
    if (url === '/api/me') return json(serverSession ? authenticated : { logged_in: false });
    if (url === '/api/admin/client/list') return json([{ uuid: 'private', name: 'private-node', deployment_status: '' }]);
    throw new Error(String(url));
  });
  const history = { current: null, route: '/admin/servers' };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  assert.equal(history.current.view, 'admin');
  await act(async () => { await logoutAdminSession(() => { throw new Error('navigation blocked'); }); });
  assert.equal(serverSession, false);
  await act(async () => { await history.current.accountRefresh(); });
  assert.equal(history.current.view, 'login');
  assert.deepEqual(history.current.nodeDetail, []);
});


test('failed server logout keeps mounted state revoked and blocks old-cookie account refresh', async t => {
  const { logoutAdminSession, isAdminLogoutPending } = await import('../src/utils/adminRevocation.ts');
  let navigated = false;
  let accountReads = 0;
  t.mock.method(globalThis, 'fetch', async url => {
    if (url === '/api/me') { accountReads++; return json(authenticated); }
    if (url === '/api/admin/client/list') return json([{ uuid: 'private', name: 'private-node', deployment_status: '' }]);
    if (url === '/api/logout') return new Response('', { status: 503 });
    throw new Error(String(url));
  });
  const history = { current: null, route: '/admin/servers' };
  const renderer = await mount(history);
  t.after(() => { act(() => renderer.unmount()); });
  await act(async () => { await assert.rejects(logoutAdminSession(() => { navigated = true; }), /Logout failed/); });
  assert.equal(navigated, false);
  assert.equal(isAdminLogoutPending(), true);
  await act(async () => { await history.current.accountRefresh(); });
  assert.equal(accountReads, 1);
  assert.equal(history.current.view, 'login');
  assert.deepEqual(history.current.nodeDetail, []);
});
