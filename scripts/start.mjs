import fs from 'node:fs';
import path from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
const root = path.resolve(import.meta.dirname, '..');
process.chdir(root);
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
