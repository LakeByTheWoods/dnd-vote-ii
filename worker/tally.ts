import type { Poll } from './types';

interface TalliedDate {
  id: string;
  value: string;
  pairwiseWins: number;
  pairwiseLosses: number;
  pairwiseTies: number;
  record: string;
  rank: number | null;
  unavailableBy: string[] | null;
  disqualified: boolean;
}
const compare = (a: string, b: string) => a < b ? -1 : a > b ? 1 : 0;

export function tally(poll: Pick<Poll, 'dates' | 'votes'>) {
  const dates: TalliedDate[] = poll.dates.map(d => ({
    ...d, pairwiseWins: 0, pairwiseLosses: 0, pairwiseTies: 0,
    record: '0-0-0', rank: null, unavailableBy: null, disqualified: false,
  }));
  const byId = new Map(dates.map(d => [d.id, d]));
  for (const vote of poll.votes) for (const id of vote.unavailableDateIds) {
    const date = byId.get(id);
    if (!date) continue;
    date.disqualified = true;
    date.unavailableBy ??= [];
    if (!date.unavailableBy.includes(vote.voterName)) date.unavailableBy.push(vote.voterName);
  }
  const eligible = dates.filter(d => !d.disqualified);
  const matrix = new Map(eligible.map(d => [d.id, new Map(eligible.map(e => [e.id, 0]))]));
  for (const vote of poll.votes) {
    const positions = new Map(vote.rankings.filter(r => matrix.has(r.dateId)).map(r => [r.dateId, r.position]));
    const last = positions.size + 1;
    for (let i = 0; i < eligible.length; i++) for (let j = i + 1; j < eligible.length; j++) {
      const a = eligible[i].id, b = eligible[j].id;
      const ap = positions.get(a) || last, bp = positions.get(b) || last;
      if (ap < bp) matrix.get(a)!.set(b, matrix.get(a)!.get(b)! + 1);
      if (bp < ap) matrix.get(b)!.set(a, matrix.get(b)!.get(a)! + 1);
    }
  }
  const pairs: { winner: string; loser: string; margin: number; support: number }[] = [];
  for (let i = 0; i < eligible.length; i++) for (let j = i + 1; j < eligible.length; j++) {
    const a = eligible[i], b = eligible[j];
    const av = matrix.get(a.id)!.get(b.id)!, bv = matrix.get(b.id)!.get(a.id)!;
    if (av === bv) { a.pairwiseTies++; b.pairwiseTies++; continue; }
    const winner = av > bv ? a : b, loser = av > bv ? b : a;
    winner.pairwiseWins++; loser.pairwiseLosses++;
    pairs.push({ winner: winner.id, loser: loser.id, margin: Math.abs(av - bv), support: Math.max(av, bv) });
  }
  pairs.sort((a, b) => b.margin - a.margin || b.support - a.support || compare(a.winner, b.winner) || compare(a.loser, b.loser));
  const edges = new Map(eligible.map(d => [d.id, new Set<string>()]));
  for (const pair of pairs) {
    const stack = [pair.loser], seen = new Set<string>();
    let cycle = false;
    while (stack.length) {
      const id = stack.pop()!;
      if (id === pair.winner) { cycle = true; break; }
      if (seen.has(id)) continue;
      seen.add(id); stack.push(...edges.get(id)!);
    }
    if (!cycle) edges.get(pair.winner)!.add(pair.loser);
  }
  const incoming = new Map(eligible.map(d => [d.id, 0]));
  for (const targets of edges.values()) for (const id of targets) incoming.set(id, incoming.get(id)! + 1);
  const recordOrder = (a: TalliedDate, b: TalliedDate) => b.pairwiseWins - a.pairwiseWins || a.pairwiseLosses - b.pairwiseLosses || compare(a.value, b.value);
  const remaining = new Set(eligible);
  let rank = 1;
  while (remaining.size) {
    const available = [...remaining].filter(d => incoming.get(d.id) === 0).sort(recordOrder);
    const chosen = available[0];
    chosen.rank = rank++; remaining.delete(chosen);
    for (const id of edges.get(chosen.id)!) incoming.set(id, incoming.get(id)! - 1);
  }
  for (const date of dates) date.record = `${date.pairwiseWins}-${date.pairwiseLosses}-${date.pairwiseTies}`;
  dates.sort((a, b) => Number(a.disqualified) - Number(b.disqualified) || (a.rank ?? 10000) - (b.rank ?? 10000) || recordOrder(a, b));
  return { talliedDates: dates, winner: dates.find(d => !d.disqualified) ?? null };
}
