import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
export const files = ['index.html', 'vote.html', 'results.html', 'styles.css', 'config.js', 'shared.js', 'host.js', 'vote.js', 'results.js'];
const root = fileURLToPath(new URL('../', import.meta.url));
export function serve() {
  const server = createServer(async (req, res) => {
    const path = new URL(req.url, 'http://localhost').pathname;
    if (path === '/session-vote') { res.writeHead(302, { Location: '/session-vote/' }); res.end(); return; }
    const name = path.replace(/^\/session-vote\//, '').replace(/^\//, '') || 'index.html';
    if (!files.includes(name)) { res.writeHead(404); res.end(); return; }
    const content = name === 'config.js'
      ? "window.SessionVoteConfig = Object.freeze({apiOrigin:'http://localhost:8787'});"
      : await readFile(resolve(root, name));
    const type = name.endsWith('.html') ? 'text/html' : name.endsWith('.css') ? 'text/css' : 'application/javascript';
    res.writeHead(200, { 'Content-Type': type + '; charset=utf-8', 'Cache-Control': 'no-store' }); res.end(content);
  });
  server.listen(8000, '127.0.0.1', () => console.log('Frontend: http://localhost:8000/session-vote/'));
  return server;
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) serve();
