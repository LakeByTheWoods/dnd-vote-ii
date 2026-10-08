import { HttpError, type Poll, type PollDate, type Vote } from './types';
import { validateVote, type VoteInput } from './validation';

const now = () => new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');
export class Store {
  constructor(public db: D1Database) {}
  statement(sql: string, ...params: (string | number)[]) { return this.db.prepare(sql).bind(...params); }

  inserts(table: 'poll_dates' | 'vote_rankings' | 'vote_unavailability', columns: string, rows: (string | number)[][]) {
    const statements: D1PreparedStatement[] = [];
    // D1 allows at most 100 bound values per statement.
    for (let i = 0; i < rows.length; i += 30) {
      const chunk = rows.slice(i, i + 30);
      const values = chunk.map(row => '(' + row.map(() => '?').join(',') + ')').join(',');
      statements.push(this.statement(`INSERT INTO ${table} (${columns}) VALUES ${values}`, ...chunk.flat()));
    }
    return statements;
  }

  async fetch(id: string): Promise<Poll> {
    const results = await this.db.batch<Record<string, unknown>>([
      this.statement('SELECT id, title, created_at AS createdAt FROM polls WHERE id = ?', id),
      this.statement('SELECT id, date_value AS value FROM poll_dates WHERE poll_id = ? ORDER BY date_value', id),
      this.statement('SELECT id, voter_name AS voterName, created_at AS createdAt FROM votes WHERE poll_id = ? ORDER BY created_at, voter_name COLLATE NOCASE', id),
      this.statement('SELECT vote_id, date_id AS dateId, position FROM vote_rankings WHERE vote_id IN (SELECT id FROM votes WHERE poll_id = ?) ORDER BY position', id),
      this.statement('SELECT vote_id, date_id FROM vote_unavailability WHERE vote_id IN (SELECT id FROM votes WHERE poll_id = ?) ORDER BY date_id', id),
    ]);
    const row = results[0].results[0] as unknown as Omit<Poll, 'dates' | 'votes'> | undefined;
    if (!row) throw new HttpError(404, 'Poll not found.');
    const votes = results[2].results.map(row => ({ ...row, rankings: [], unavailableDateIds: [] })) as unknown as Vote[];
    const byId = new Map(votes.map(v => [v.id, v]));
    for (const r of results[3].results) byId.get(String(r.vote_id))!.rankings.push({ dateId: String(r.dateId), position: Number(r.position) });
    for (const r of results[4].results) byId.get(String(r.vote_id))!.unavailableDateIds.push(String(r.date_id));
    return { ...row, dates: results[1].results as unknown as PollDate[], votes };
  }

  async list() {
    return (await this.db.prepare(`SELECT p.id, p.title, p.created_at AS createdAt,
      (SELECT COUNT(*) FROM poll_dates d WHERE d.poll_id = p.id) AS dateCount,
      (SELECT COUNT(*) FROM votes v WHERE v.poll_id = p.id) AS voteCount
      FROM polls p ORDER BY p.created_at DESC`).all()).results;
  }

  async create(title: string, values: string[]) {
    const id = crypto.randomUUID(), createdAt = now();
    const bytes = crypto.getRandomValues(new Uint8Array(32));
    const managementToken = btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const dates = values.map(value => ({ id: crypto.randomUUID(), value }));
    await this.db.batch([
      this.statement('INSERT INTO polls (id,title,created_at,management_token) VALUES (?,?,?,?)', id, title, createdAt, managementToken),
      ...this.inserts('poll_dates', 'id,poll_id,date_value', dates.map(d => [d.id, id, d.value])),
    ]);
    return { poll: { id, title, createdAt, dates, votes: [] }, managementToken };
  }

  async replaceVote(pollId: string, input: VoteInput) {
    validateVote(input, await this.fetch(pollId));
    const id = crypto.randomUUID();
    // Match and replace within one transaction. Cascades remove the old ballot's children.
    await this.db.batch([
      this.statement('DELETE FROM votes WHERE poll_id = ? AND voter_name = ? COLLATE NOCASE', pollId, input.voterName),
      this.statement('INSERT INTO votes (id,poll_id,voter_name,created_at) VALUES (?,?,?,?)', id, pollId, input.voterName, now()),
      ...this.inserts('vote_rankings', 'vote_id,date_id,position', input.rankings.map((dateId, i) => [id, dateId, i + 1])),
      ...this.inserts('vote_unavailability', 'vote_id,date_id', input.unavailableDateIds.map(dateId => [id, dateId])),
    ]);
    return this.fetch(pollId);
  }

  async authorize(pollId: string, token: string) {
    if (!token) throw new HttpError(403, 'Management token is required.');
    const row = await this.statement('SELECT management_token FROM polls WHERE id = ?', pollId).first<{management_token: string}>();
    if (!await secretEqual(row?.management_token ?? '', token)) throw new HttpError(403, 'Management token is invalid.');
  }

  async deleteVote(pollId: string, voteId: string) {
    await this.fetch(pollId);
    const result = await this.statement('DELETE FROM votes WHERE id = ? AND poll_id = ?', voteId, pollId).run();
    if (!result.meta.changes) throw new HttpError(404, 'Vote not found.');
    return this.fetch(pollId);
  }
}

export async function secretEqual(expected: string, candidate: string) {
  const encode = new TextEncoder();
  const [a, b] = await Promise.all([expected, candidate].map(s => crypto.subtle.digest('SHA-256', encode.encode(s))));
  return crypto.subtle.timingSafeEqual(a, b) && expected.length > 0;
}
