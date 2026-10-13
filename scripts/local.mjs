import { spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { createServer as createHTTPServer } from 'node:http';
import { createServer as createTCPServer } from 'node:net';
import { mkdir, readFile, writeFile, chmod, unlink, access, open, rename } from 'node:fs/promises';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const runtime = join(root, '.runtime');
const stateFile = join(runtime, 'local-supervisor.json');
const sleep = ms => new Promise(r => setTimeout(r, ms));
export const parseJSON = text => JSON.parse(text.replace(/^\uFEFF/, ''));

export async function initCredentials(directory = runtime, env = process.env) {
  await mkdir(directory, { recursive: true, mode: 0o700 });
  if (process.platform !== 'win32') await chmod(directory, 0o700);
  const credentialsFile = join(directory, 'admin-credentials.json');
  const secretFile = join(directory, 'jwt-secret.txt');
  async function createOnce(file, content) {
    try { await writeFile(file, content, { flag: 'wx', mode: 0o600 }); }
    catch (error) { if (error.code !== 'EEXIST') throw error; }
    if (process.platform !== 'win32') await chmod(file, 0o600);
  }
  await createOnce(credentialsFile, JSON.stringify({ username: 'admin', password: env.KS_BOOTSTRAP_ADMIN_PASSWORD || `Local-${randomBytes(24).toString('hex')}!` }, null, 2));
  await createOnce(secretFile, env.KS_JWT_SECRET || randomBytes(48).toString('base64'));
  const credentials = parseJSON(await readFile(credentialsFile, 'utf8'));
  const secret = (await readFile(secretFile, 'utf8')).replace(/^\uFEFF/, '').trim();
  if (credentials.initialized !== true && (typeof credentials.password !== 'string' || Buffer.byteLength(credentials.password) < 12 || Buffer.byteLength(credentials.password) > 72)) throw new Error('Initial password must be 12–72 bytes');
  if (Buffer.byteLength(secret) < 32) throw new Error('JWT secret must be at least 32 bytes');
  return { credentials, secret, credentialsFile };
}

export async function retireCredentials(file) {
  const record = parseJSON(await readFile(file, 'utf8'));
  if (record.initialized === true && !record.password) return;
  const temporary = `${file}.${randomBytes(6).toString('hex')}.tmp`;
  try {
    await writeFile(temporary, JSON.stringify({ username: record.username, initialized: true }, null, 2), { mode: 0o600, flag: 'wx' });
    await rename(temporary, file);
  } finally { await unlink(temporary).catch(() => {}); }
}

export async function assertPortFree(port) {
  const probe = createTCPServer();
  await new Promise((accept, reject) => {
    probe.once('error', () => reject(new Error(`Port ${port} is already in use; stop the existing service first`)));
    probe.listen(port, '127.0.0.1', () => probe.close(accept));
  });
}

function run(command, args, cwd = root, env = process.env, children = []) {
  return new Promise((accept, reject) => {
    const child = spawn(command, args, { cwd, env, stdio: 'inherit', windowsHide: true, detached: process.platform !== 'win32' });
    child.supervisorHelper = true;
    children.push(child);
    child.once('error', reject);
    child.once('exit', code => code === 0 ? accept() : reject(new Error(`${command} exited with code ${code}`)));
  });
}

export async function waitReady(url, timeout = 60000, signal) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    signal?.throwIfAborted();
    try { const response = await fetch(url, { signal: AbortSignal.timeout(1500) }); if (response.ok) return; } catch { }
    await sleep(300);
  }
  throw new Error(`Readiness timeout: ${url}; check .runtime/*.log`);
}

async function stop() {
  let state;
  try { state = parseJSON(await readFile(stateFile, 'utf8')); }
  catch (error) { if (error.code === 'ENOENT') { console.log('No local supervisor recorded.'); return; } throw error; }
  // Contact the launch-specific controller instead of killing a potentially reused PID.
  const response = await fetch(`http://127.0.0.1:${state.port}/stop`, { method: 'POST', headers: { Authorization: `Bearer ${state.token}` }, signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error('Supervisor identity mismatch; no processes were stopped');
  console.log('Stop requested. MySQL/Redis containers and data are retained.');
}

async function start(skipDependencies) {
  for (const port of [8080, 3000, 3001]) await assertPortFree(port);
  for (const directory of ['frontend', 'user-frontend']) {
    try { await access(join(root, directory, 'node_modules/vite/bin/vite.js')); }
    catch { throw new Error(`Install dependencies first: cd ${directory} && npm ci`); }
  }
  const { credentials, secret, credentialsFile } = await initCredentials();
  const env = { ...process.env, KS_JWT_SECRET: secret, KS_BOOTSTRAP_ADMIN_PASSWORD: credentials.password,
    KS_OAUTH_GITHUB_FRONTEND_URL: process.env.KS_OAUTH_GITHUB_FRONTEND_URL || 'http://localhost:3001' };
  if (!credentials.password) delete env.KS_BOOTSTRAP_ADMIN_PASSWORD;
  const children = [];
  let stopping = false;
  let shutdownPromise;
  let credentialMonitor;
  let stateOwned = false;
  const startup = new AbortController();
  const token = randomBytes(32).toString('hex');
  const controller = createHTTPServer((req, res) => {
    if (req.method !== 'POST' || req.url !== '/stop' || req.headers.authorization !== `Bearer ${token}`) { res.writeHead(403).end(); return; }
    res.writeHead(200).end('Stopping');
    void shutdown();
  });
  function shutdown(code = 0) {
    if (shutdownPromise) return shutdownPromise;
    stopping = true;
    startup.abort(new Error('Services stopped during startup'));
    clearInterval(credentialMonitor);
    controller.close();
    shutdownPromise = (async () => {
      await Promise.all(children.map(async child => {
      if (!child.pid || child.exitCode !== null || child.signalCode !== null) return;
      const exited = new Promise(r => child.once('exit', r));
      function signalChild(signal) {
        try {
          if (child.supervisorHelper && process.platform !== 'win32') process.kill(-child.pid, signal);
          else child.kill(signal);
        } catch (error) { if (error.code !== 'ESRCH') throw error; }
      }
      if (child.supervisorHelper && process.platform === 'win32') {
        // Go builds may own compiler/linker subprocesses. Stop that tree before releasing the launch record.
        await new Promise(r => {
          const killer = spawn('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore', windowsHide: true });
          killer.once('error', () => { signalChild('SIGTERM'); r(); });
          killer.once('exit', code => { if (code !== 0) signalChild('SIGTERM'); r(); });
        });
      } else signalChild('SIGTERM');
      let timeout;
      try { await Promise.race([exited, new Promise(r => { timeout = setTimeout(r, 10000); })]); }
      finally { clearTimeout(timeout); }
      if (child.exitCode === null && child.signalCode === null) signalChild('SIGKILL');
      await exited;
      }));
      if (stateOwned) await unlink(stateFile).catch(() => {});
      process.exitCode = code;
    })();
    return shutdownPromise;
  }
  process.once('SIGINT', () => void shutdown());
  process.once('SIGTERM', () => void shutdown());
  try {
    await new Promise((accept, reject) => { controller.once('error', reject); controller.listen(0, '127.0.0.1', accept); });
    if (stopping) { controller.close(); await shutdown(); return; }
    // A stale record is never interpreted as permission to kill another process.
    try { await writeFile(stateFile, JSON.stringify({ port: controller.address().port, token }), { flag: 'wx', mode: 0o600 }); stateOwned = true; }
    catch (error) { if (error.code === 'EEXIST') throw new Error('Supervisor record exists. Run stop, or inspect and remove the stale .runtime/local-supervisor.json record.'); throw error; }
    // A signal during the asynchronous write can finish shutdown before ownership is recorded.
    if (stopping) { await shutdown(); await unlink(stateFile); return; }
    if (!skipDependencies) {
      if (stopping) return;
      await run('docker', ['compose', 'up', '-d', '--wait', 'mysql', 'redis'], root, env, children);
    }
    if (stopping) return;
    const binary = join(runtime, process.platform === 'win32' ? 'server.exe' : 'server');
    await run('go', ['build', '-o', binary, './cmd/server'], join(root, 'backend'), env, children);
    async function launch(name, command, args, cwd) {
      if (stopping) throw new Error('Startup interrupted');
      const output = await open(join(runtime, `${name}.log`), 'w', 0o600);
      let child;
      try {
        if (stopping) throw new Error('Startup interrupted');
        const childEnv = { ...env };
        if (name !== 'backend') { delete childEnv.KS_BOOTSTRAP_ADMIN_PASSWORD; delete childEnv.KS_JWT_SECRET; }
        child = spawn(command, args, { cwd, env: childEnv, stdio: ['ignore', output.fd, output.fd], windowsHide: true });
        children.push(child);
        // Spawn errors can fire before an asynchronous log close completes.
        child.once('error', error => { console.error(`${name}: ${error.message}`); void shutdown(1); });
        child.once('exit', (code, signal) => { if (!stopping) { console.error(`${name} exited (${code ?? signal}); stopping remaining services`); void shutdown(1); } });
      }
      finally { await output.close(); }
    }
    await launch('backend', binary, [], join(root, 'backend'));
    await waitReady('http://127.0.0.1:8080/readyz', 60000, startup.signal);
    for (const [name, port] of [['frontend', 3000], ['user-frontend', 3001]]) {
      await launch(name, process.execPath, ['node_modules/vite/bin/vite.js', '--host', '127.0.0.1', '--port', String(port), '--strictPort'], join(root, name));
      await waitReady(`http://127.0.0.1:${port}/login`, 60000, startup.signal);
    }
    if (stopping) return;
    console.log('Ready: backend :8080, admin :3000, user :3001');
    console.log(`Administrator credentials: ${credentialsFile}`);
    console.log('Keep this terminal open. Ctrl+C or node scripts/local.mjs stop stops all three services.');
    let checkingCredentials = false;
    async function checkCredentials() {
      if (checkingCredentials || stopping || credentials.initialized === true) return;
      checkingCredentials = true;
      try {
        const res = await fetch('http://127.0.0.1:8080/api/v1/auth/bootstrap/status', { signal: AbortSignal.timeout(2000) });
        const body = await res.json();
        if (body.code === 0 && body.data?.completed === true) {
          await retireCredentials(credentialsFile);
          credentials.initialized = true;
          delete credentials.password; delete env.KS_BOOTSTRAP_ADMIN_PASSWORD;
          clearInterval(credentialMonitor);
          console.log('Initial password removed from local credential file.');
        }
      } catch { /* Keep the credential until completion can be verified. */ }
      finally { checkingCredentials = false; }
    }
    credentialMonitor = setInterval(() => void checkCredentials(), 3000);
    credentialMonitor.unref();
    await checkCredentials();
  } catch (error) {
    if (startup.signal.aborted) { await shutdown(); return; }
    await shutdown(1); throw error;
  }
}

async function main() {
  const [command, ...options] = process.argv.slice(2);
  if (options.some(option => option !== '--skip-dependencies')) throw new Error('Unknown option');
  if (command === 'init') { const result = await initCredentials(); console.log(`Administrator credentials: ${result.credentialsFile}`); }
  else if (command === 'start') await start(options.includes('--skip-dependencies'));
  else if (command === 'stop') await stop();
  else throw new Error('Usage: node scripts/local.mjs init | start [--skip-dependencies] | stop');
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch(error => { console.error(error.message); process.exitCode = 1; });
}
