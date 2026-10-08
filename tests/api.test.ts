import { env } from 'cloudflare:workers';
import { expect, test } from 'vitest';
import worker from '../worker/index';
import { normalizePoll } from '../worker/validation';
import { Store } from '../worker/store';

const origin = 'https://lakebythewoods.neocities.org';
async function api(path: string, method = 'GET', body?: unknown, headers = {}, override = {}) {
  const response = await worker.fetch(new Request('https://test' + path, {
    method, headers: { Origin: origin, 'Content-Type': 'application/json', ...headers },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  }), { ...env, ...override });
  return { response, status: response.status, data: await response.json() as any };
}
const create = () => api('/api/polls', 'POST', { title: 'Game', dates: ['2099-01-03', '2099-01-01', '2099-01-02'] });
const ballot = (poll: any, voterName = 'Alex') => ({ voterName, rankings: poll.dates.map((d: any) => d.id), unavailableDateIds: [] });

test('create, list, fetch, results, replacement, and private token exclusion', async () => {
  expect((await api('/api/health')).data).toEqual({ ok: true });
  const { data, status } = await create();
  expect(status).toBe(201); expect(data.managementToken).toMatch(/^[\w-]{43}$/);
  const poll = data.poll, path = `/api/polls/${poll.id}`;
  expect(poll.dates.map((d: any) => d.value)).toEqual(['2099-01-01', '2099-01-02', '2099-01-03']);
  expect((await api(path)).data).toEqual({ poll });
  expect((await api('/api/polls')).data.polls).toEqual([{ id: poll.id, title: 'Game', createdAt: poll.createdAt, dateCount: 3, voteCount: 0 }]);
  const first = await api(path + '/votes', 'POST', ballot(poll));
  expect(first.status).toBe(201);
  const edited = await api(path + '/votes', 'POST', { voterName: 'aLEX', rankings: [poll.dates[1].id], unavailableDateIds: [poll.dates[0].id] });
  expect(edited.data.poll.votes).toHaveLength(1);
  expect(edited.data.poll.votes[0].id).not.toBe(first.data.poll.votes[0].id);
  const results = await api(path + '/results');
  expect(results.data.winner.id).toBe(poll.dates[1].id);
  expect(results.data.talliedDates.find((d: any) => d.id === poll.dates[0].id).disqualified).toBe(true);
  expect(JSON.stringify(results.data)).not.toContain(data.managementToken);
  expect(results.response.headers.get('Cache-Control')).toBe('no-store');
});

test('host deletion checks missing, invalid and wrong-poll tokens', async () => {
  const a = (await create()).data, b = (await create()).data;
  const saved = (await api(`/api/polls/${a.poll.id}/votes`, 'POST', ballot(a.poll))).data;
  const path = `/api/polls/${a.poll.id}/votes/${saved.poll.votes[0].id}`;
  for (const token of ['', 'wrong', b.managementToken]) expect((await api(path, 'DELETE', undefined, { 'X-DnD-Vote-Management-Token': token })).status).toBe(403);
  const auth = { 'X-DnD-Vote-Management-Token': a.managementToken };
  expect((await api(path, 'DELETE', undefined, auth)).data.poll.votes).toEqual([]);
  expect((await api(path, 'DELETE', undefined, auth)).data.error).toBe('Vote not found.');
  expect((await env.DB.prepare('SELECT * FROM vote_rankings').all()).results).toEqual([]);
});

test('admin clear is disabled without configuration and checks the secret', async () => {
  const { poll } = (await create()).data;
  await api(`/api/polls/${poll.id}/votes`, 'POST', { voterName: 'Alex', rankings: [poll.dates[0].id], unavailableDateIds: [poll.dates[1].id] });
  expect((await api('/api/polls', 'DELETE')).data.error).toBe('Admin secret is required.');
  expect((await api('/api/polls', 'DELETE', undefined, { 'X-DnD-Vote-Admin-Secret': 'secret' })).status).toBe(403);
  expect((await api('/api/polls', 'DELETE', undefined, { 'X-DnD-Vote-Admin-Secret': 'wrong' }, { DND_VOTE_ADMIN_SECRET: 'secret' })).status).toBe(403);
  expect((await api('/api/polls', 'DELETE', undefined, { 'X-DnD-Vote-Admin-Secret': 'secret' }, { DND_VOTE_ADMIN_SECRET: 'secret' })).data).toEqual({ ok: true });
  for (const table of ['polls', 'poll_dates', 'votes', 'vote_rankings', 'vote_unavailability']) expect((await env.DB.prepare(`SELECT * FROM ${table}`).all()).results).toEqual([]);
});

test('simultaneous replacements leave one complete ballot and no orphan rows', async () => {
  const { poll } = (await create()).data;
  const responses = await Promise.all(Array.from({ length: 8 }, (_, i) => api(`/api/polls/${poll.id}/votes`, 'POST', {
    voterName: i % 2 ? 'ALEX' : 'alex', rankings: [poll.dates[i % 2].id], unavailableDateIds: [poll.dates[2].id],
  })));
  expect(responses.map(r => r.status)).toEqual(Array(8).fill(201));
  const result = (await api(`/api/polls/${poll.id}`)).data.poll;
  expect(result.votes).toHaveLength(1);
  expect(result.votes[0].rankings).toHaveLength(1);
  expect(result.votes[0].unavailableDateIds).toEqual([poll.dates[2].id]);
  expect((await env.DB.prepare('SELECT * FROM vote_rankings').all()).results).toHaveLength(1);
});

test('a failed D1 batch restores the deleted ballot', async () => {
  const { poll } = (await create()).data;
  await api(`/api/polls/${poll.id}/votes`, 'POST', ballot(poll));
  const before = await new Store(env.DB).fetch(poll.id);
  await expect(env.DB.batch([
    env.DB.prepare('DELETE FROM votes WHERE poll_id = ?').bind(poll.id),
    env.DB.prepare('INSERT INTO votes VALUES (?, ?, ?, ?)').bind('bad', 'missing-poll', 'Alex', ''),
  ])).rejects.toThrow();
  expect(await new Store(env.DB).fetch(poll.id)).toEqual(before);
});

test('missing routes and polls return 404', async () => {
  for (const path of ['/api/unknown', '/api/polls/missing', '/api/polls/missing/results']) expect((await api(path)).status).toBe(404);
  expect((await api('/api/polls/missing/votes', 'POST', { voterName: 'Alex', rankings: ['a'], unavailableDateIds: [] })).status).toBe(404);
});

test('larger ballots stay within D1 statement binding limits', async () => {
  const dates = Array.from({ length: 90 }, (_, i) => new Date(Date.UTC(2099, 0, i + 1)).toISOString().slice(0, 10));
  const { data, status } = await api('/api/polls', 'POST', { title: 'Large poll', dates });
  expect(status).toBe(201);
  expect((await api(`/api/polls/${data.poll.id}/votes`, 'POST', ballot(data.poll))).status).toBe(201);
  expect((await api(`/api/polls/${data.poll.id}`)).data.poll.votes[0].rankings).toHaveLength(90);
});

test('results return null winner when every date is unavailable', async () => {
  const { poll } = (await create()).data;
  const ids = poll.dates.map((d: any) => d.id);
  await api(`/api/polls/${poll.id}/votes`, 'POST', { voterName: 'A', rankings: [ids[0]], unavailableDateIds: ids.slice(1) });
  await api(`/api/polls/${poll.id}/votes`, 'POST', { voterName: 'B', rankings: [ids[1]], unavailableDateIds: [ids[0]] });
  const result = (await api(`/api/polls/${poll.id}/results`)).data;
  expect(result.winner).toBeNull();
  expect(result.talliedDates.every((d: any) => d.disqualified && d.rank === null)).toBe(true);
});

test('poll validation retains messages and Unicode limits', async () => {
  const dates = ['2099-01-01', '2099-01-02'];
  const cases = [
    [{ title: '' , dates }, 'Title is required.'],
    [{ title: '😀'.repeat(121), dates }, 'Title must be 120 characters or fewer.'],
    [{ title: 'x', dates: '' }, 'Dates must be an array.'],
    [{ title: 'x', dates: [dates[0], dates[0]] }, 'At least two unique dates are required.'],
    ...['2099-02-30', '2099-1-01', {}, ''].map(value => [{ title: 'x', dates: [dates[0], value] }, 'Dates must use YYYY-MM-DD format.']),
    [{ title: 'x', dates: ['2000-01-01', dates[0]] }, 'Candidate dates must be today or later.'],
  ];
  for (const [body, message] of cases) {
    const result = await api('/api/polls', 'POST', body);
    expect(result.status).toBe(400); expect(result.data.error).toBe(message);
  }
  expect((await api('/api/polls', 'POST', { title: '😀'.repeat(120), dates })).status).toBe(201);
  expect(normalizePoll({ title: 'x', dates: ['2030-01-01', '2030-01-02'] }, new Date('2030-01-01T23:59:59Z')).dates).toHaveLength(2);
  expect(() => normalizePoll({ title: 'x', dates: ['2030-01-01', '2030-01-02'] }, new Date('2030-01-02T00:00:00Z'))).toThrow('Candidate dates must be today or later.');
});

test('vote validation retains errors', async () => {
  const { poll } = (await create()).data;
  const [a, b] = poll.dates.map((d: any) => d.id);
  const base = { voterName: 'Alex', rankings: [a], unavailableDateIds: [] };
  const cases = [
    ...[null, {}, [], true, 42].map(voterName => [{ voterName }, 'Voter name must be a string.']),
    [{ voterName: ' ' }, 'Voter name is required.'],
    [{ voterName: '😀'.repeat(81) }, 'Voter name must be 80 characters or fewer.'],
    [{ rankings: a }, 'Rankings and unavailableDateIds must be arrays.'],
    [{ unavailableDateIds: b }, 'Rankings and unavailableDateIds must be arrays.'],
    [{ rankings: [] }, 'At least one ranked date is required.'],
    [{ rankings: [a, a] }, 'Ranked dates must be unique.'],
    [{ unavailableDateIds: [b, b] }, 'Unavailable dates must be unique.'],
    [{ rankings: ['missing'] }, 'Rankings include an unknown date.'],
    [{ unavailableDateIds: ['missing'] }, 'Unavailable dates include an unknown date.'],
    [{ unavailableDateIds: [a] }, 'A date cannot be both ranked and unavailable.'],
  ];
  for (const [patch, message] of cases) {
    const r = await api(`/api/polls/${poll.id}/votes`, 'POST', { ...base, ...patch as object });
    expect(r.status).toBe(400); expect(r.data.error).toBe(message);
  }
  expect((await api(`/api/polls/${poll.id}/votes`, 'POST', { ...base, voterName: '😀'.repeat(80) })).status).toBe(201);
});

test('malformed, nonobject and oversized streaming bodies are rejected', async () => {
  for (const [body, status] of [['{', 400], ['[]', 400], ['null', 400], [' '.repeat(8193), 413]] as const) {
    const bytes = new TextEncoder().encode(body);
    const stream = new ReadableStream({ start(controller) { controller.enqueue(bytes); controller.close(); } });
    const response = await worker.fetch(new Request('https://test/api/polls', { method: 'POST', body: stream }), env);
    expect(response.status).toBe(status);
  }
});

test('CORS permits the site, rejects other origins, and covers errors', async () => {
  expect((await api('/api/polls/missing')).response.headers.get('Access-Control-Allow-Origin')).toBe(origin);
  const denied = await api('/api/health', 'GET', undefined, { Origin: 'https://other.example' });
  expect(denied.status).toBe(403); expect(denied.response.headers.has('Access-Control-Allow-Origin')).toBe(false);
  const preflight = (extra = {}) => worker.fetch(new Request('https://test/api/polls', { method: 'OPTIONS', headers: {
    Origin: origin, 'Access-Control-Request-Method': 'DELETE', 'Access-Control-Request-Headers': 'content-type,x-dnd-vote-management-token', ...extra,
  } }), env);
  expect((await preflight()).status).toBe(204);
  expect((await preflight({ 'Access-Control-Request-Headers': 'x-other' })).status).toBe(403);
  expect((await preflight({ 'Access-Control-Request-Method': 'PATCH' })).status).toBe(403);
});
