import { HttpError, type Env } from './types';
import { Store, secretEqual } from './store';
import { normalizePoll, normalizeVote, readBody } from './validation';
import { tally } from './tally';

const managementHeader = 'X-DnD-Vote-Management-Token';
const adminHeader = 'X-DnD-Vote-Admin-Secret';
const allowedHeaders = ['content-type', managementHeader.toLowerCase(), adminHeader.toLowerCase()];
const methods = ['GET', 'POST', 'DELETE'];

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const origin = request.headers.get('Origin');
    const allowed = env.ALLOWED_ORIGINS.split(',').map(s => s.trim()).includes(origin ?? '');
    const headers = new Headers({ 'Content-Type': 'application/json', 'Cache-Control': 'no-store', Vary: 'Origin' });
    if (origin && allowed) headers.set('Access-Control-Allow-Origin', origin);
    const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers });
    let failure = 'Request could not be completed.';
    try {
      if (origin && !allowed) throw new HttpError(403, 'Origin is not allowed.');
      if (request.method === 'OPTIONS') {
        const requested = (request.headers.get('Access-Control-Request-Headers') ?? '').toLowerCase().split(',').map(s => s.trim()).filter(Boolean);
        if (!origin || !methods.includes(request.headers.get('Access-Control-Request-Method') ?? '') || requested.some(h => !allowedHeaders.includes(h))) throw new HttpError(403, 'Preflight is not allowed.');
        headers.set('Access-Control-Allow-Methods', methods.join(', '));
        headers.set('Access-Control-Allow-Headers', ['Content-Type', managementHeader, adminHeader].join(', '));
        return new Response(null, { status: 204, headers });
      }
      const path = new URL(request.url).pathname;
      const store = new Store(env.DB);
      if (path === '/api/health') return request.method === 'GET' ? json({ ok: true }) : json({ error: 'Method not allowed.' }, 405);
      if (path === '/api/polls') {
        if (request.method === 'GET') {
          failure = 'Poll summaries could not be fetched.';
          return json({ polls: await store.list() });
        }
        if (request.method === 'POST') {
          failure = 'Poll could not be created.';
          const input = normalizePoll(await readBody(request));
          return json(await store.create(input.title, input.dates), 201);
        }
        if (request.method === 'DELETE') {
          failure = 'Polls could not be cleared.';
          const secret = request.headers.get(adminHeader) ?? '';
          if (!secret) throw new HttpError(403, 'Admin secret is required.');
          if (!await secretEqual(env.DND_VOTE_ADMIN_SECRET ?? '', secret)) throw new HttpError(403, 'Admin secret is invalid.');
          await env.DB.prepare('DELETE FROM polls').run();
          return json({ ok: true });
        }
        return json({ error: 'Method not allowed.' }, 405);
      }
      const match = /^\/api\/polls\/([^/]+)(?:\/(results|votes)(?:\/([^/]+))?)?$/.exec(path);
      if (match) {
        const [, encodedId, action, encodedVoteId] = match;
        const pollId = decodeURIComponent(encodedId);
        if (!action && request.method === 'GET') {
          failure = 'Poll could not be fetched.';
          return json({ poll: await store.fetch(pollId) });
        }
        if (action === 'results' && !encodedVoteId && request.method === 'GET') {
          failure = 'Poll could not be fetched.';
          const poll = await store.fetch(pollId);
          return json({ poll, ...tally(poll) });
        }
        if (action === 'votes' && !encodedVoteId && request.method === 'POST') {
          failure = 'Vote could not be saved.';
          return json({ poll: await store.replaceVote(pollId, normalizeVote(await readBody(request))) }, 201);
        }
        if (action === 'votes' && encodedVoteId && request.method === 'DELETE') {
          failure = 'Vote could not be deleted.';
          await store.authorize(pollId, request.headers.get(managementHeader) ?? '');
          return json({ poll: await store.deleteVote(pollId, decodeURIComponent(encodedVoteId)) });
        }
        if (!encodedVoteId || action === 'votes') return json({ error: 'Method not allowed.' }, 405);
      }
      return json({ error: 'Route not found.' }, 404);
    } catch (error) {
      if (error instanceof HttpError) return json({ error: error.message }, error.status);
      // Do not log bodies, headers, SQL parameters, or private links.
      console.error('API operation failed', { operation: failure, errorType: error instanceof Error ? error.name : 'Unknown' });
      return json({ error: failure }, 500);
    }
  },
};
