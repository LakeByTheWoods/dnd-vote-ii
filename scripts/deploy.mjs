import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { randomBytes, createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import assert from 'node:assert/strict';
import { files } from './static.mjs';
import { privateDir, cf, neo, api, query, wrangler } from './cloud.mjs';

const command = process.argv[2] ?? 'deploy';
const stamp = new Date().toISOString().replace(/[:.]/g, '-');
if (command === 'export') {
  const output = join(privateDir, 'deployments', `${stamp}-database.sql`);
  wrangler(['d1', 'export', 'DB', '--remote', '--output', output]);
  console.log('Database export saved:', output);
} else if (command === 'rollback') {
  const manifestPath = process.argv[3];
  if (!manifestPath) throw new Error('Pass the deployment manifest path.');
  await rollback(JSON.parse(await readFile(manifestPath, 'utf8')));
} else {
  if (execFileSync('git', ['status', '--porcelain'], { encoding: 'utf8' }).trim()) throw new Error('Commit all source changes before deployment.');
  const identity = await neo('info');
  assert.equal(identity.info.sitename, 'lakebythewoods');
  assert.equal(identity.info.supporter, true);
  const folder = join(privateDir, 'deployments', stamp);
  await mkdir(folder, { recursive: true });
  const manifest = { commit: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(), folder, files: [], attemptedFiles: [], priorVersion: null, testPollIds: [], status: 'preparing' };
  const manifestPath = join(folder, 'manifest.json');
  const save = () => writeFile(manifestPath, JSON.stringify(manifest, null, 2));
  const deployments = await cf('/workers/scripts/session-vote-api/deployments');
  const active = deployments.deployments?.[0];
  if (active?.versions?.length === 1) manifest.priorVersion = active.versions[0].version_id;
  else if (active) throw new Error('Expected a single active Worker version before deployment.');
  const existing = (await neo('list?path=session-vote')).files;
  for (const name of files) {
    const path = 'session-vote/' + name;
    const old = existing.find(f => f.path === path);
    if (old?.is_directory) throw new Error('A deployment filename is a directory: ' + path);
    if (old) await writeFile(join(folder, name), await neo('download?path=' + encodeURIComponent(path)));
    const content = await readFile(name);
    manifest.files.push({ name, existed: !!old, changed: old?.sha1_hash !== createHash('sha1').update(content).digest('hex'), sha256: createHash('sha256').update(content).digest('hex') });
  }
  await save();
  try {
    wrangler(['d1', 'migrations', 'apply', 'DB', '--remote']);
    const secretPath = join(privateDir, 'admin-secret.txt');
    let secret;
    try { secret = (await readFile(secretPath, 'utf8')).trim(); }
    catch (error) { if (error.code !== 'ENOENT') throw error; secret = randomBytes(32).toString('base64url'); await writeFile(secretPath, secret, { flag: 'wx', mode: 0o600 }); }
    await cf('/workers/scripts/session-vote-api/secrets', 'PUT', { name: 'DND_VOTE_ADMIN_SECRET', text: secret, type: 'secret_text' });
    wrangler(['deploy']);
    await api('/api/health');
    const dates = [1, 2, 3].map(n => new Date(Date.now() + n * 86400000).toISOString().slice(0, 10));
    const { poll, managementToken } = await api('/api/polls', 'POST', { title: 'Deployment verification ' + stamp, dates });
    manifest.testPollIds.push(poll.id); await save();
    const path = '/api/polls/' + poll.id;
    await api(path + '/votes', 'POST', { voterName: 'Deployment test', rankings: [poll.dates[1].id, poll.dates[0].id], unavailableDateIds: [poll.dates[2].id] });
    const before = await api(path + '/results');
    assert.equal(before.winner.id, poll.dates[1].id);
    wrangler(['deploy']);
    assert.deepEqual(await api(path + '/results'), before);
    await api(path + '/votes/' + before.poll.votes[0].id, 'DELETE', undefined, { 'X-DnD-Vote-Management-Token': managementToken });
    assert.equal((await api(path)).poll.votes.length, 0);
    await query('DELETE FROM polls WHERE id = ?', [poll.id]);
    manifest.testPollIds = []; manifest.status = 'backend-verified'; await save();
    const changed = manifest.files.filter(f => f.changed);
    if (changed.length) {
      const form = new FormData();
      for (const file of changed) form.append('session-vote/' + file.name, new Blob([await readFile(file.name)]), file.name);
      manifest.attemptedFiles = changed.map(f => f.name); await save();
      await neo('upload', form);
    }
    const uploaded = (await neo('list?path=session-vote')).files;
    for (const file of manifest.files) {
      const content = await readFile(file.name);
      assert.equal(uploaded.find(f => f.path === 'session-vote/' + file.name)?.sha1_hash, createHash('sha1').update(content).digest('hex'));
    }
    manifest.status = 'deployed'; await save();
    console.log('Deployed https://lakebythewoods.neocities.org/session-vote/');
    console.log('Deployment manifest:', manifestPath);
  } catch (error) {
    manifest.status = 'failed'; await save();
    await rollback(manifest);
    throw error;
  }
}

async function rollback(manifest) {
  const identity = await neo('info');
  assert.equal(identity.info.sitename, 'lakebythewoods');
  const restore = new FormData(), remove = new URLSearchParams();
  let restoreCount = 0, removeCount = 0;
  const uploaded = (await neo('list?path=session-vote')).files;
  for (const name of manifest.attemptedFiles) {
    if (!files.includes(name)) throw new Error('Unknown rollback filename.');
    const file = manifest.files.find(f => f.name === name);
    if (file.existed) { restore.append('session-vote/' + name, new Blob([await readFile(join(manifest.folder, name))]), name); restoreCount++; }
    else if (uploaded.some(f => f.path === 'session-vote/' + name)) { remove.append('filenames[]', 'session-vote/' + name); removeCount++; }
  }
  if (restoreCount) await neo('upload', restore);
  if (removeCount) await neo('delete', remove);
  if (manifest.priorVersion) await cf('/workers/scripts/session-vote-api/deployments', 'POST', { strategy: 'percentage', versions: [{ version_id: manifest.priorVersion, percentage: 100 }] });
  console.log('Rollback completed. D1 data was preserved.');
}
