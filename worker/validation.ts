import { bad, HttpError, type Poll } from './types';

export async function readBody(request: Request): Promise<Record<string, unknown>> {
  const limit = 8192;
  if (Number(request.headers.get('Content-Length')) > limit) throw new HttpError(413, `Request body must be ${limit} bytes or fewer.`);
  const reader = request.body?.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  if (reader) while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > limit) { await reader.cancel(); throw new HttpError(413, `Request body must be ${limit} bytes or fewer.`); }
    chunks.push(value);
  }
  const bytes = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
  let value: unknown;
  try { value = JSON.parse(new TextDecoder().decode(bytes)); }
  catch { return bad('Request body must be valid JSON.'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) return bad('Request body must be a JSON object.');
  return value as Record<string, unknown>;
}

export function normalizePoll(input: Record<string, unknown>, now = new Date()) {
  const title = typeof input.title === 'string' ? input.title.trim() : '';
  if (!title) bad('Title is required.');
  if ([...title].length > 120) bad('Title must be 120 characters or fewer.');
  if (!Array.isArray(input.dates)) return bad('Dates must be an array.');
  const dates = new Set<string>();
  for (const raw of input.dates) {
    if (typeof raw !== 'string') return bad('Dates must use YYYY-MM-DD format.');
    const value = raw.trim();
    const parsed = new Date(value + 'T00:00:00Z');
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || !Number.isFinite(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== value) bad('Dates must use YYYY-MM-DD format.');
    if (value < now.toISOString().slice(0, 10)) bad('Candidate dates must be today or later.');
    dates.add(value);
  }
  if (dates.size < 2) bad('At least two unique dates are required.');
  return { title, dates: [...dates].sort() };
}

export function normalizeVote(input: Record<string, unknown>) {
  if (typeof input.voterName !== 'string') return bad('Voter name must be a string.');
  if (!Array.isArray(input.rankings) || !Array.isArray(input.unavailableDateIds)) return bad('Rankings and unavailableDateIds must be arrays.');
  const voterName = input.voterName.trim();
  if (!voterName) bad('Voter name is required.');
  if ([...voterName].length > 80) bad('Voter name must be 80 characters or fewer.');
  return { voterName, rankings: input.rankings.map(String), unavailableDateIds: input.unavailableDateIds.map(String) };
}
export type VoteInput = ReturnType<typeof normalizeVote>;

export function validateVote(vote: VoteInput, poll: Poll) {
  if (!vote.rankings.length) bad('At least one ranked date is required.');
  if (new Set(vote.rankings).size !== vote.rankings.length) bad('Ranked dates must be unique.');
  if (new Set(vote.unavailableDateIds).size !== vote.unavailableDateIds.length) bad('Unavailable dates must be unique.');
  const valid = new Set(poll.dates.map(d => d.id));
  if (vote.rankings.some(id => !valid.has(id))) bad('Rankings include an unknown date.');
  if (vote.unavailableDateIds.some(id => !valid.has(id))) bad('Unavailable dates include an unknown date.');
  if (vote.unavailableDateIds.some(id => vote.rankings.includes(id))) bad('A date cannot be both ranked and unavailable.');
}
