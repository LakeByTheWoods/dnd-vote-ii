import { expect, test } from 'vitest';
import { tally } from '../worker/tally';
import fixtures from './tally-fixtures.json';

for (const f of fixtures) test(f.name, () => {
  const result = tally({
    dates: f.poll.Dates.map(d => ({ id: d.ID, value: d.Value })),
    votes: f.poll.Votes.map((v, i) => ({
      id: String(i), createdAt: '', voterName: v.VoterName,
      rankings: v.Rankings.map(r => ({ dateId: r.DateID, position: r.Position })),
      unavailableDateIds: v.UnavailableDateIDs,
    })),
  });
  expect(result.talliedDates.map(d => d.id)).toEqual(f.order);
  expect(result.talliedDates.map(d => d.record)).toEqual(f.records);
  expect(result.talliedDates.map(d => d.rank ?? 0)).toEqual(f.ranks);
  expect(result.talliedDates.map(d => d.disqualified)).toEqual(f.disqualified);
  expect(result.winner?.id ?? '').toBe(f.winner);
  for (const [id, names] of Object.entries(f.unavailable ?? {})) expect(result.talliedDates.find(d => d.id === id)?.unavailableBy).toEqual(names);
});

const dates = ['a', 'b', 'c'].map((id, i) => ({ id, value: `2030-01-0${i + 1}` }));
test('no ballots retain calendar ordering and a winner', () => {
  const result = tally({ dates, votes: [] });
  expect(result.winner?.id).toBe('a');
  expect(result.talliedDates.map(d => d.record)).toEqual(['0-0-2', '0-0-2', '0-0-2']);
});
test('omitted dates tie behind ranked dates; unavailable names are deduplicated', () => {
  const vote = { id: 'v', createdAt: '', voterName: 'Alex', rankings: [{ dateId: 'b', position: 1 }], unavailableDateIds: [] };
  expect(tally({ dates, votes: [vote] }).winner?.id).toBe('b');
  const result = tally({ dates, votes: [{ ...vote, unavailableDateIds: ['c', 'c'] }] });
  expect(result.talliedDates.find(d => d.id === 'c')?.unavailableBy).toEqual(['Alex']);
});
