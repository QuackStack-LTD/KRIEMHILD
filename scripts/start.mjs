import fs from 'node:fs';
import path from 'node:path';
import net from 'node:net';
import { spawn, spawnSync } from 'node:child_process';
const root = path.resolve(import.meta.dirname, '..');
process.chdir(root);
// Do this before rebuilding dist: an older server otherwise starts serving the
// new frontend with its old backend after a second launch fails to bind.
const address = process.env.KRIEMHILD_ADDR || '127.0.0.1:8124';
const target = new URL(`http://${address.startsWith(':') ? '127.0.0.1' + address : address}`);
const alreadyRunning = await new Promise(resolve => {
  const socket = net.createConnection({ host: target.hostname.replace(/^\[|\]$/g, ''), port: Number(target.port || 80) });
  const finish = value => { socket.destroy(); resolve(value); };
  socket.setTimeout(1000, () => finish(false));
  socket.once('connect', () => finish(true));
  socket.once('error', () => finish(false));
});
if (alreadyRunning) {
  console.error(`A server is already running at ${address}. Save your open work and close that server, then run start.bat again to activate this checkout. No build files were changed.`);
  process.exit(1);
}
const goLocal = path.join(root, '.tools/go/bin', process.platform === 'win32' ? 'go.exe' : 'go');
const go = fs.existsSync(goLocal) ? goLocal : 'go';
const check = spawnSync(go, ['version'], { stdio: 'ignore' });
if (check.status !== 0) { console.error('Install Go 1.26 or newer from https://go.dev/dl/ and run npm start again.'); process.exit(1); }
const vite = path.join(root, 'node_modules/vite/bin/vite.js');
if (!fs.existsSync(vite)) { console.error('Run npm install first.'); process.exit(1); }
const build = spawnSync(process.execPath, [vite, 'build'], { stdio: 'inherit' });
if (build.status !== 0) process.exit(build.status || 1);
const server = spawn(go, ['run', './cmd/kriemhild'], { stdio: 'inherit' });
process.on('SIGINT', () => server.kill('SIGINT'));
server.on('exit', code => process.exit(code || 0));
