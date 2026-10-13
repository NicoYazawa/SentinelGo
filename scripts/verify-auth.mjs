import { randomBytes } from 'node:crypto';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { root, parseJSON } from './local.mjs';

// Only disposable local test users/roles are modified. SMS must use the dev provider.
const base = (process.argv[2] || 'http://127.0.0.1:8080').replace(/\/$/, '');
const credentials = parseJSON(await readFile(join(root, '.runtime/admin-credentials.json'), 'utf8'));
const adminPassword = process.env.SENTINEL_VERIFY_ADMIN_PASSWORD || credentials.password;
if (!adminPassword) throw new Error('Set SENTINEL_VERIFY_ADMIN_PASSWORD to the formal admin password for this test run; it will not be saved');
const checks = [], users = new Set(), roles = new Set();
async function api(method, path, body, access) {
  const response = await fetch(`${base}/api/v1${path}`, { method,
    headers: { 'Content-Type': 'application/json', ...(access ? { Authorization: `Bearer ${access}` } : {}) },
    body: body == null ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(15000) });
  return response.json();
}
function check(result, expected, name) {
  if (result.code !== expected) throw new Error(`${name}: expected ${expected}, got ${result.code} (${result.msg})`);
  checks.push({ name, passed: true }); console.log(`PASS ${name}`); return result;
}
async function login(username, password) {
  const result = await api('POST', '/auth/login', { username, password });
  if (result.code !== 0) throw new Error(`Login failed: ${result.msg}`);
  return result.data;
}
const admin = await login(credentials.username, adminPassword);
const bootstrapState = await api('GET', '/auth/bootstrap/status');
if (bootstrapState.data?.password_change_required) {
  await api('POST', '/auth/logout', null, admin.access_token);
  throw new Error('Complete first-login password setup before running the normal authentication checks');
}
const stamp = `${Date.now()}${randomBytes(3).toString('hex')}`;
const username = `verify_${stamp}`, password = `Verify-${randomBytes(20).toString('hex')}!`;
const phone = `139${String(Date.now()).slice(-8)}`;
const a = (method, path, body) => api(method, path, body, admin.access_token);
try {
  for (const [port, url] of [[3000, process.env.SENTINEL_ADMIN_URL || 'http://127.0.0.1:3000'], [3001, process.env.SENTINEL_USER_URL || 'http://127.0.0.1:3001']]) {
    const page = await fetch(`${url.replace(/\/$/, '')}/login`, { signal: AbortSignal.timeout(15000) });
    if (!page.ok || !(await page.text()).includes('<div id="app">')) throw new Error(`Frontend ${port} not served`);
    check({ code: 0 }, 0, `frontend ${port}`);
  }
  const registration = check(await api('POST', '/auth/register', { username, password }), 0, 'password registration');
  const id = registration.data.user_id; users.add(id);
  let user = await login(username, password);
  const u = (method, path, body) => api(method, path, body, user.access_token);
  check(await u('GET', '/user/info'), 0, 'new account basic permissions');
  const channels = check(await u('GET', '/auth/channels'), 0, 'password channel initialized');
  if (!channels.data.some(c => c.identity_type === 'password')) throw new Error('Password channel missing');
  const basicRole = (await a('GET', '/roles')).data.find(r => r.code === 'user');
  if (!basicRole) throw new Error('Basic role missing');
  const role = check(await a('POST', '/roles', { name: 'Verification', code: `verify_${stamp}` }), 0, 'temporary role creation');
  const roleID = role.data.id; roles.add(roleID);
  check(await a('PUT', `/roles/${roleID}/permissions`, { permissions: [{ path: '/api/v1/users', method: 'GET' }] }), 0, 'role permissions assignment');
  check(await a('PUT', `/users/${id}/roles`, { role_ids: [roleID] }), 0, 'temporary role assignment');
  check(await u('GET', '/users'), 0, 'assigned role grants access');
  check(await a('PUT', `/users/${id}/roles`, { role_ids: [basicRole.id] }), 0, 'role downgrade');
  check(await u('GET', '/users'), 20002, 'old token loses removed role permissions');
  check(await a('PUT', `/users/${id}/roles`, { role_ids: [roleID] }), 0, 'reassign temporary role');
  check(await a('PUT', `/roles/${roleID}/status`, { status: 0 }), 0, 'disable temporary role');
  check(await u('GET', '/users'), 20002, 'disabled role loses access immediately');
  check(await a('PUT', `/roles/${roleID}/status`, { status: 1 }), 0, 'enable temporary role');
  check(await a('DELETE', `/roles/${roleID}`), 0, 'delete temporary role'); roles.delete(roleID);
  check(await u('GET', '/users'), 20002, 'deleted role loses access immediately');
  check(await a('PUT', `/users/${id}/roles`, { role_ids: [basicRole.id] }), 0, 'restore basic role');
  check(await a('PUT', `/users/${id}/status`, { status: 0 }), 0, 'disable temporary account');
  check(await u('GET', '/user/info'), 20000, 'disabled account rejects old access token');
  check(await api('POST', '/auth/refresh', { refresh_token: user.refresh_token }), 20001, 'disabled account rejects refresh token');
  check(await a('PUT', `/users/${id}/status`, { status: 1 }), 0, 'enable temporary account');
  check(await u('GET', '/user/info'), 20000, 'reenabling does not revive revoked session');
  user = await login(username, password);
  const other = await login(username, password);
  check(await u('POST', '/auth/logout'), 0, 'logout current session');
  check(await api('POST', '/auth/refresh', { refresh_token: user.refresh_token }), 20001, 'logged-out refresh rejected');
  check(await api('GET', '/user/info', null, other.access_token), 0, 'other login session remains valid');
  const rotation = check(await api('POST', '/auth/refresh', { refresh_token: other.refresh_token }), 0, 'refresh rotates token');
  check(await api('POST', '/auth/refresh', { refresh_token: other.refresh_token }), 20001, 'refresh replay rejected');
  user = rotation.data;
  // Read-only channels cannot expose provider configuration. Require explicit local dev verification.
  if (process.env.SENTINEL_VERIFY_DEV_SMS !== '1') throw new Error('Set SENTINEL_VERIFY_DEV_SMS=1 only for a local SMS dev provider; no SMS request was sent');
  const sms = check(await u('POST', '/auth/sms/send', { phone, scene: 'bind' }), 0, 'send binding SMS in development mode');
  if (!sms.data.dev_code) throw new Error('SMS dev provider is required');
  check(await u('POST', '/auth/bind/phone', { phone, code: sms.data.dev_code }), 0, 'bind phone with matching scene');
  check(await u('POST', '/auth/bind/phone', { phone, code: sms.data.dev_code }), 11000, 'binding SMS replay rejected');
  const newPassword = `Changed-${randomBytes(20).toString('hex')}!`;
  check(await a('PUT', `/users/${id}/password`, { password: newPassword }), 0, 'change account password');
  check(await u('GET', '/user/info'), 20000, 'password change revokes access');
  check(await api('POST', '/auth/refresh', { refresh_token: user.refresh_token }), 20001, 'password change revokes refresh');
  user = await login(username, newPassword);
  check(await api('GET', '/auth/oauth/github/callback?code=invalid'), 11000, 'OAuth callback rejects missing state');
  check(await a('DELETE', `/users/${id}`), 0, 'delete temporary account'); users.delete(id);
  check(await u('GET', '/user/info'), 20000, 'deleted account rejects old access');
  await mkdir(join(root, '.runtime'), { recursive: true });
  await writeFile(join(root, '.runtime/verification.json'), JSON.stringify({ verifiedAt: new Date().toISOString(), checks }, null, 2), { mode: 0o600 });
  console.log(`Verified ${checks.length} checks.`);
} finally {
  for (const id of users) { try { const r = await a('DELETE', `/users/${id}`); if (r.code !== 0) throw new Error(r.msg); } catch (e) { console.error(`Could not remove test user ${id}: ${e.message}`); } }
  for (const id of roles) { try { const r = await a('DELETE', `/roles/${id}`); if (r.code !== 0) throw new Error(r.msg); } catch (e) { console.error(`Could not remove test role ${id}: ${e.message}`); } }
  try { await a('POST', '/auth/logout'); } catch { }
}
