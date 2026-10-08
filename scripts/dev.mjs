import { spawnSync, spawn } from 'node:child_process';
import { serve } from './static.mjs';
const wrangler = 'node_modules/wrangler/bin/wrangler.js';
const migrated = spawnSync(process.execPath, [wrangler, 'd1', 'migrations', 'apply', 'DB', '--local', '--env', 'local'], { stdio: 'inherit', windowsHide: true });
if (migrated.status !== 0) process.exit(migrated.status ?? 1);
const worker = spawn(process.execPath, [wrangler, 'dev', '--env', 'local', '--port', '8787'], { stdio: 'inherit', windowsHide: true });
const server = serve();
process.on('SIGINT', () => { worker.kill(); server.close(); });
worker.on('exit', code => { server.close(); process.exitCode = code ?? 0; });
