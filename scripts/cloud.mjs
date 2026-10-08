import { readFile, mkdir } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

export const privateDir = join(homedir(), '.config', 'session-vote');
export const credentials = JSON.parse((await readFile(join(privateDir, 'deploy-secrets.json'), 'utf8')).replace(/^\uFEFF/, ''));
export const config = JSON.parse(await readFile('wrangler.jsonc', 'utf8'));
export const apiOrigin = 'https://session-vote-api.elbeaston.workers.dev';
export const siteOrigin = 'https://lakebythewoods.neocities.org';
await mkdir(join(privateDir, 'deployments'), { recursive: true });

export function wrangler(args) {
  const result = spawnSync(process.execPath, ['node_modules/wrangler/bin/wrangler.js', ...args], {
    stdio: 'inherit', windowsHide: true,
    env: { ...process.env, CI: 'true', CLOUDFLARE_ACCOUNT_ID: credentials.CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN: credentials.CLOUDFLARE_API_TOKEN, WRANGLER_SEND_METRICS: 'false' },
  });
  if (result.status !== 0) throw new Error('Wrangler failed: ' + args.slice(0, 3).join(' '));
}
export async function cf(path, method = 'GET', body) {
  const response = await fetch(`https://api.cloudflare.com/client/v4/accounts/${credentials.CLOUDFLARE_ACCOUNT_ID}${path}`, {
    method, headers: { Authorization: `Bearer ${credentials.CLOUDFLARE_API_TOKEN}`, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(60000),
  });
  const data = await response.json();
  if (!response.ok || !data.success) throw new Error(`Cloudflare ${method} ${path} failed (${response.status}; codes ${(data.errors ?? []).map(e => e.code).join(',')})`);
  return data.result;
}
export async function neo(path, body) {
  const response = await fetch('https://neocities.org/api/' + path, {
    method: body ? 'POST' : 'GET', headers: { Authorization: `Bearer ${credentials.NEOCITIES_API_KEY}` },
    body, signal: AbortSignal.timeout(60000),
  });
  if (!response.ok) throw new Error(`Neocities ${path} failed (${response.status})`);
  if (path.startsWith('download')) return new Uint8Array(await response.arrayBuffer());
  const data = await response.json();
  if (data.result !== 'success') throw new Error(`Neocities ${path} failed (${data.error_type})`);
  return data;
}
export async function query(sql, params = []) {
  return cf(`/d1/database/${config.d1_databases[0].database_id}/query`, 'POST', { sql, params });
}
export async function api(path, method = 'GET', body, headers = {}) {
  const response = await fetch(apiOrigin + path, {
    method, headers: { Origin: siteOrigin, 'Content-Type': 'application/json', ...headers },
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(30000),
  });
  if (!response.ok) throw new Error(`App ${method} ${path} failed (${response.status})`);
  if (response.headers.get('Access-Control-Allow-Origin') !== siteOrigin) throw new Error('Missing production CORS header');
  return response.json();
}
