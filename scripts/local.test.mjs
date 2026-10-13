import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile, rm, stat, mkdir, copyFile, access } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createServer } from 'node:net';
import { initCredentials, assertPortFree, parseJSON, root, retireCredentials } from './local.mjs';

test('credentials survive restarts and PowerShell BOM files remain usable', async t => {
  const directory = await mkdtemp(join(tmpdir(), 'sentinel credentials '));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const first = await initCredentials(directory, {});
  const second = await initCredentials(directory, {});
  assert.deepEqual(first, second);
  assert.ok(Buffer.byteLength(first.credentials.password) >= 12);
  assert.ok(Buffer.byteLength(first.secret) >= 32);
  await writeFile(first.credentialsFile, '\uFEFF' + JSON.stringify(first.credentials));
  assert.deepEqual((await initCredentials(directory, {})).credentials, first.credentials);
  assert.deepEqual(parseJSON(await readFile(first.credentialsFile, 'utf8')), first.credentials);
  if (process.platform !== 'win32') assert.equal((await stat(first.credentialsFile)).mode & 0o777, 0o600);
});

test('busy ports are rejected without stopping their owner', async t => {
  const server = createServer();
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  t.after(() => new Promise(r => server.close(r)));
  await assert.rejects(assertPortFree(server.address().port), /already in use/);
  assert.ok(server.listening);
});

test('retiring bootstrap credentials removes plaintext and restarts never regenerate it', async t => {
  const directory = await mkdtemp(join(tmpdir(), 'sentinel completed '));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const first = await initCredentials(directory, {});
  await retireCredentials(first.credentialsFile);
  const completed = await initCredentials(directory, {});
  assert.deepEqual(completed.credentials, { username: 'admin', initialized: true });
  assert.equal(completed.secret, first.secret);
  assert.ok(!(await readFile(first.credentialsFile, 'utf8')).includes(first.credentials.password));
  await retireCredentials(first.credentialsFile);
});

test('Linux supervisor starts from paths with spaces and stops only its own services', { skip: process.platform === 'win32', timeout: 30000 }, async t => {
  const directory = await mkdtemp(join(tmpdir(), 'sentinel supervisor '));
  let supervisor;
  t.after(async () => { supervisor?.kill('SIGTERM'); await rm(directory, { recursive: true, force: true }); });
  for (const part of ['scripts', 'backend', 'tools', 'frontend/node_modules/vite/bin', 'user-frontend/node_modules/vite/bin']) {
    await mkdir(join(directory, part), { recursive: true });
  }
  await copyFile(join(root, 'scripts/local.mjs'), join(directory, 'scripts/local.mjs'));
  const service = `const http = require('node:http'); let completed=false; const port = Number(process.argv[process.argv.indexOf('--port') + 1]) || 8080; const server = http.createServer((q,r) => { if(q.url==='/complete')completed=true; r.end(q.url==='/api/v1/auth/bootstrap/status'?JSON.stringify({code:0,data:{completed}}):'ok'); }); server.listen(port,'127.0.0.1'); process.on('SIGTERM',()=>server.close());`;
  for (const frontend of ['frontend', 'user-frontend']) await writeFile(join(directory, frontend, 'node_modules/vite/bin/vite.js'), service);
  await writeFile(join(directory, 'tools/go'), `#!${process.execPath}\nconst fs=require('node:fs');const output=process.argv[process.argv.indexOf('-o')+1];fs.writeFileSync(output,${JSON.stringify(`#!${process.execPath}\n${service}`)},{mode:0o700});`, { mode: 0o700 });
  supervisor = spawn(process.execPath, ['scripts/local.mjs', 'start', '--skip-dependencies'], { cwd: directory, env: { ...process.env, PATH: `${join(directory, 'tools')}:${process.env.PATH}` }, stdio: ['ignore', 'pipe', 'pipe'] });
  let output = '';
  supervisor.stdout.on('data', chunk => { output += chunk; });
  supervisor.stderr.on('data', chunk => { output += chunk; });
  const exited = new Promise(r => supervisor.once('exit', r));
  const deadline = Date.now() + 12000;
  while (!output.includes('Ready:') && Date.now() < deadline && supervisor.exitCode === null) await new Promise(r => setTimeout(r, 100));
  assert.match(output, /Ready:/);
  const state = parseJSON(await readFile(join(directory, '.runtime/local-supervisor.json'), 'utf8'));
  const denied = await fetch(`http://127.0.0.1:${state.port}/stop`, { method: 'POST' });
  assert.equal(denied.status, 403);
  assert.ok((await fetch('http://127.0.0.1:8080/readyz')).ok);
  await fetch('http://127.0.0.1:8080/complete');
  let retired;
  const retireDeadline = Date.now() + 6000;
  while (!retired && Date.now() < retireDeadline) {
    retired = parseJSON(await readFile(join(directory, '.runtime/admin-credentials.json'), 'utf8')).initialized;
    if (!retired) await new Promise(r => setTimeout(r, 50));
  }
  assert.equal(retired, true);
  const stopper = spawn(process.execPath, ['scripts/local.mjs', 'stop'], { cwd: directory, stdio: 'pipe' });
  assert.equal(await new Promise(r => stopper.once('exit', r)), 0);
  assert.equal(await exited, 0);
  await assert.rejects(access(join(directory, '.runtime/local-supervisor.json')));
  for (const port of [8080, 3000, 3001]) await assertPortFree(port);
  // A failing frontend must stop the already-started backend and remove ownership state.
  await writeFile(join(directory, 'frontend/node_modules/vite/bin/vite.js'), 'process.exit(7);');
  supervisor = spawn(process.execPath, ['scripts/local.mjs', 'start', '--skip-dependencies'], { cwd: directory, env: { ...process.env, PATH: `${join(directory, 'tools')}:${process.env.PATH}` }, stdio: 'pipe' });
  assert.equal(await new Promise(r => supervisor.once('exit', r)), 1);
  await assert.rejects(access(join(directory, '.runtime/local-supervisor.json')));
  for (const port of [8080, 3000, 3001]) await assertPortFree(port);
  // A missing executable raises an asynchronous spawn error rather than an exit event.
  await rm(join(directory, '.runtime/server'));
  await writeFile(join(directory, 'tools/go'), `#!${process.execPath}\nprocess.exit(0);`, { mode: 0o700 });
  supervisor = spawn(process.execPath, ['scripts/local.mjs', 'start', '--skip-dependencies'], { cwd: directory, env: { ...process.env, PATH: `${join(directory, 'tools')}:${process.env.PATH}` }, stdio: 'pipe' });
  assert.equal(await new Promise(r => supervisor.once('exit', r)), 1);
  await assert.rejects(access(join(directory, '.runtime/local-supervisor.json')));
  // Stop during a build; neither the build nor its delayed output may outlive the ownership record.
  const marker = join(directory, 'build-finished');
  await writeFile(join(directory, 'tools/go'), `#!${process.execPath}\nsetTimeout(()=>require('node:fs').writeFileSync(${JSON.stringify(marker)},'done'),3000);`, { mode: 0o700 });
  supervisor = spawn(process.execPath, ['scripts/local.mjs', 'start', '--skip-dependencies'], { cwd: directory, env: { ...process.env, PATH: `${join(directory, 'tools')}:${process.env.PATH}` }, stdio: 'pipe' });
  const interrupted = new Promise(r => supervisor.once('exit', r));
  let buildState;
  const stateDeadline = Date.now() + 5000;
  while (!buildState && Date.now() < stateDeadline) {
    try { buildState = parseJSON(await readFile(join(directory, '.runtime/local-supervisor.json'), 'utf8')); } catch { await new Promise(r => setTimeout(r, 30)); }
  }
  assert.ok(buildState);
  // Ensure the helper has started before interrupting it.
  await new Promise(r => setTimeout(r, 150));
  assert.equal((await fetch(`http://127.0.0.1:${buildState.port}/stop`, { method: 'POST', headers: { Authorization: `Bearer ${buildState.token}` } })).status, 200);
  assert.equal(await interrupted, 0);
  await assert.rejects(access(join(directory, '.runtime/local-supervisor.json')));
  await new Promise(r => setTimeout(r, 3100));
  await assert.rejects(access(marker));
});
