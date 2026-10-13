// Browser API integration with a real Python server; no npm dependencies.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');

function storage() {
  const items = new Map();
  return {
    getItem: key => items.get(key) ?? null,
    setItem: (key, value) => items.set(key, String(value)),
    removeItem: key => items.delete(key),
  };
}

function browserApi(origin) {
  const requests = [];
  const window = {
    sessionStorage: storage(), localStorage: storage(),
    location: { href: origin + '/', search: '' },
    fetch: (url, options) => {
      requests.push({ url, options });
      return fetch(new URL(url, origin), options);
    },
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, '..', 'shared.js'), 'utf8'), {
    window, URL, URLSearchParams, Intl,
  });
  return { api: window.DndVoteApp, window, requests };
}

test('host headers, private receipts, public links and editing work over HTTP', async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'dnd-client-'));
  const key = 'test-host-' + 'a'.repeat(40);
  const process = spawn(global.process.env.PYTHON || 'python', ['-u', '-c',
    'import server; server.initialize_database(); s = server.ThreadingHTTPServer(("127.0.0.1", 0), server.AppHandler); print(s.server_address[1], flush=True); s.serve_forever()'], {
    cwd: path.join(__dirname, '..'),
    env: { ...global.process.env, DND_VOTE_ADMIN_TOKEN: key, DND_VOTE_DATA_DIR: directory },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  try {
    const port = await new Promise((resolve, reject) => {
      let output = '';
      const timer = setTimeout(() => reject(new Error('Server startup timed out')), 5000);
      process.once('error', error => { clearTimeout(timer); reject(error); });
      process.once('exit', code => { clearTimeout(timer); reject(new Error(`Server exited: ${code}`)); });
      process.stdout.on('data', chunk => {
        output += chunk;
        if (output.includes('\n')) { clearTimeout(timer); resolve(Number(output.trim())); }
      });
    });
    const origin = `http://127.0.0.1:${port}`;
    const host = browserApi(origin);
    const voter = browserApi(origin);
    const outsider = browserApi(origin);
    await assert.rejects(host.api.getAllPolls(), /Host authorisation required/);
    host.api.setHostKey(key);
    assert.equal(host.api.hasHostKey(), true);
    assert.equal(host.window.localStorage.getItem('dnd-vote-host-key'), null);
    const poll = await host.api.createPoll('Campaign', ['2026-11-01', '2026-11-02']);
    assert.equal((await host.api.getAllPolls()).length, 1);
    const link = host.api.buildPageLink('vote.html', poll.id);
    assert.equal(new URL(link).search, '?poll=' + poll.id);
    assert.equal(link.includes(key), false);
    const ballot = { voterName: 'Alice', rankings: poll.dates.map(date => date.id), unavailableDateIds: [] };
    let voted = await voter.api.saveVote(poll.id, ballot);
    assert.equal(voter.api.hasVoteReceipt(poll.id, 'ALICE'), true);
    assert.equal(voter.requests.at(-1).options.headers.Authorization, undefined);
    assert.equal(voter.requests.at(-1).options.headers['Content-Type'], 'application/json');
    const receipt = voter.window.localStorage.getItem(`dnd-vote-receipt:${poll.id}:alice`);
    await assert.rejects(outsider.api.saveVote(poll.id, ballot), /editing receipt/);
    voted = await voter.api.saveVote(poll.id, { ...ballot, rankings: [...ballot.rankings].reverse() });
    assert.equal(voter.requests.at(-1).options.headers['X-Vote-Token'], receipt);
    assert.equal(voted.votes.length, 1);
    const results = await outsider.api.getResults(poll.id);
    assert.equal(results.poll.votes.length, 1);
    assert.equal(JSON.stringify(results).includes(receipt), false);
    // Public reads do not carry the host key, even in the host's tab.
    await host.api.getPoll(poll.id);
    assert.equal(host.requests.at(-1).options.headers.Authorization, undefined);
    await host.api.getResults(poll.id);
    assert.equal(host.requests.at(-1).options.headers.Authorization, undefined);
    await assert.rejects(voter.api.deleteVote(poll.id, voted.votes[0].id), /Host authorisation/);
    await host.api.deleteVote(poll.id, voted.votes[0].id);
    host.api.setHostKey('');
    assert.equal(host.api.hasHostKey(), false);
    await assert.rejects(host.api.clearPolls(), /Host authorisation/);
    host.api.setHostKey(key);
    await host.api.clearPolls();
    assert.equal((await host.api.getAllPolls()).length, 0);
  } finally {
    process.kill();
    if (process.exitCode === null) await new Promise(resolve => process.once('exit', resolve));
    fs.rmSync(directory, { recursive: true, force: true });
  }
});
