export interface PollDate { id: string; value: string }
export interface Vote {
  id: string;
  voterName: string;
  createdAt: string;
  rankings: { dateId: string; position: number }[];
  unavailableDateIds: string[];
}
export interface Poll {
  id: string;
  title: string;
  createdAt: string;
  dates: PollDate[];
  votes: Vote[];
}
export interface Env {
  DB: D1Database;
  ALLOWED_ORIGINS: string;
  DND_VOTE_ADMIN_SECRET?: string;
}
export class HttpError extends Error {
  constructor(public status: number, message: string) { super(message); }
}
export const bad = (message: string): never => { throw new HttpError(400, message); };
