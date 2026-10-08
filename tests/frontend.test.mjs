import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
const source = await readFile(new URL('../shared.js', import.meta.url), 'utf8');
function app(href, fetch = async () => ({ ok: true, json: async () => ({}) })) {
  const window = { location: new URL(href), SessionVoteConfig: { apiOrigin: 'https://api.example' }, fetch };
  vm.runInNewContext(source, { window, URL, URLSearchParams, Intl });
  return window.DndVoteApp;
}
test('share links work from a folder URL and from an HTML page', () => {
  for (const href of ['https://site.example/session-vote/', 'https://site.example/session-vote/index.html?old=x#part']) {
    assert.equal(app(href).buildPageLink('vote.html', 'a b'), 'https://site.example/session-vote/vote.html?poll=a%20b');
    assert.equal(app(href).buildPageLink('results.html', 'id'), 'https://site.example/session-vote/results.html?poll=id');
  }
  assert.equal(app('https://site.example/').buildPageLink('vote.html', 'id'), 'https://site.example/vote.html?poll=id');
});
test('API origin and management headers are combined with defaults', async () => {
  let call;
  const api = app('https://site.example/session-vote/', async (...args) => { call = args; return { ok: true, json: async () => ({ poll: {} }) }; });
  await api.deleteVote('poll', 'vote', 'test-token');
  assert.equal(call[0], 'https://api.example/api/polls/poll/votes/vote');
  assert.equal(call[1].headers['Content-Type'], 'application/json');
  assert.equal(call[1].headers['X-DnD-Vote-Management-Token'], 'test-token');
});
