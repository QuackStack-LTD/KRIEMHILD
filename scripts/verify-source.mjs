import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';

// Packages a commit, never a working directory. All outputs stay outside source.
const root = path.resolve(import.meta.dirname, '..');
const revision = process.argv[2] || 'HEAD';
const destination = process.argv[3]
  ? path.resolve(process.argv[3])
  : fs.mkdtempSync(path.join(os.tmpdir(), 'KRIEMHILD-source-'));
if (fs.existsSync(destination) && fs.readdirSync(destination).length)
  throw Error('The verification destination must be empty.');
fs.mkdirSync(destination, { recursive: true });
const git = (...args) => {
  const r = spawnSync('git', ['-C', root, ...args], { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024, windowsHide: true });
  if (r.status !== 0) throw Error(r.stderr || `git ${args[0]} failed`);
  return r.stdout.trim();
};
const commit = git('rev-parse', '--verify', `${revision}^{commit}`);
const format = git('rev-parse', '--show-object-format');
const report = { commit, source: 'git archive', node: process.version, checks: [], warnings: [] };
const save = () => fs.writeFileSync(path.join(destination, 'verification.json'), JSON.stringify(report, null, 2) + '\n');
const source = path.join(destination, 'source');
const localGo = path.join(root, '.tools/go/bin', process.platform === 'win32' ? 'go.exe' : 'go');
const go = fs.existsSync(localGo) ? localGo : 'go';
const env = { ...process.env, PATH: `${path.dirname(go)}${path.delimiter}${process.env.PATH || ''}` };
function run(name, exe, args, cwd = root) {
  const started = performance.now();
  // npm.cmd needs cmd.exe on Windows. Arguments here are fixed, never user text.
  const windowsNpm = process.platform === 'win32' && exe === 'npm';
  const r = spawnSync(windowsNpm ? (process.env.ComSpec || 'cmd.exe') : exe,
    windowsNpm ? ['/d', '/s', '/c', `npm.cmd ${args.join(' ')}`] : args,
    { cwd, env, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, windowsHide: true });
  fs.writeFileSync(path.join(destination, `${name}.txt`), (r.stdout || '') + (r.stderr || '') + (r.error ? `\n${r.error}` : ''));
  report.checks.push({ name, command: [exe, ...args], exit: r.status, signal: r.signal, durationMs: Math.round(performance.now() - started) });
  save();
  console.log(`${name}: ${r.status === 0 ? 'PASS' : 'FAIL'} (${Math.round((performance.now() - started) / 1000)}s)`);
  return r;
}
try {
  const tree = git('ls-tree', '-rz', commit).split('\0').filter(Boolean).map(entry => {
    const [meta, name] = entry.split('\t');
    const [mode, type, oid] = meta.split(' ');
    return { mode, type, oid, name };
  });
  if (tree.some(e => e.mode === '160000')) throw Error('Submodules require an explicit packaging policy.');
  const names = new Set();
  for (const e of tree) {
    if (e.mode === '120000') throw Error(`Symlink requires platform review: ${e.name}`);
    for (const part of e.name.split('/')) {
      if (/[<>:"\\|?*\x00-\x1f]|[. ]$/.test(part) || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(part))
        throw Error(`Windows-incompatible path: ${e.name}`);
    }
    const folded = e.name.toLowerCase();
    if (names.has(folded)) throw Error(`Case collision: ${e.name}`);
    names.add(folded);
  }
  report.archive = path.join(destination, 'KRIEMHILD-source.zip');
  if (run('archive', 'git', ['archive', '--format=zip', `--output=${report.archive}`, commit]).status !== 0) throw Error('Archive creation failed.');
  report.bytes = fs.statSync(report.archive).size;
  report.sha256 = crypto.createHash('sha256').update(fs.readFileSync(report.archive)).digest('hex');
  if (run('list', 'tar', ['-tf', report.archive]).status !== 0) throw Error('Archive listing failed.');
  fs.mkdirSync(source);
  if (run('extract', 'tar', ['-xf', report.archive, '-C', source]).status !== 0) throw Error('Archive extraction failed.');
  let count = 0;
  const walk = directory => {
    for (const e of fs.readdirSync(directory, { withFileTypes: true })) {
      const p = path.join(directory, e.name);
      if (e.isDirectory()) walk(p); else count++;
    }
  };
  walk(source);
  if (count !== tree.length) throw Error(`Extracted ${count} files; expected ${tree.length}.`);
  for (const e of tree) {
    const bytes = fs.readFileSync(path.join(source, e.name));
    const oid = crypto.createHash(format).update(`blob ${bytes.length}\0`).update(bytes).digest('hex');
    if (oid !== e.oid) throw Error(`Missing or altered archive entry: ${e.name} (check export-ignore/export-subst).`);
    if (bytes.length < 2048 && bytes.toString().startsWith('version https://git-lfs.github.com/spec/v1\n'))
      throw Error(`Unresolved Git LFS pointer: ${e.name}`);
  }
  report.filesVerified = count;
  report.extracted = source;
  run('go-version', go, ['version'], source);
  run('go-env', go, ['env', 'GOMOD'], source);
  run('go-build', go, ['build', './...'], source);
  run('go-test', go, ['test', './...', '-timeout', process.env.KRIEMHILD_TEST_TIMEOUT || '10m'], source);
  run('npm-version', 'npm', ['--version'], source);
  if (run('npm-ci', 'npm', ['ci'], source).status === 0) {
    run('npm-test', 'npm', ['test'], source);
    run('frontend-build', 'npm', ['run', 'build'], source);
  }
  report.passed = report.checks.every(c => c.exit === 0);
} catch (e) {
  report.error = e.message;
  report.passed = false;
} finally {
  save();
  console.log(`Verification report: ${path.join(destination, 'verification.json')}`);
}
if (!report.passed) process.exitCode = 1;
