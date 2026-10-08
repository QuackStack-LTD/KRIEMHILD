import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { spawn, spawnSync } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';

const root = path.resolve(import.meta.dirname, '..');
const reference = vm.createContext({});
for (const name of ['wfc.js', 'environment.js', 'presets.js']) vm.runInContext(fs.readFileSync(path.join(import.meta.dirname, 'reference', name), 'utf8'), reference);
const config = JSON.parse(fs.readFileSync(path.join(import.meta.dirname, 'reference/tiles.json'), 'utf8'));
const currentPresets=vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(root,'src/terrain/presets.js'),'utf8'),currentPresets);
const plain = value => JSON.parse(JSON.stringify(value));
const defaults = { width: 24, height: 16, radius2: 2, selection: 'random', stability: 3, continents: { count: 12, strength: 5 }, climate: { strength: 2, layout: 'both' }, seed: 12345 };
let server;
const base = 'http://127.0.0.1:18125';
before(async () => {
  const local = path.join(root, '.tools/go/bin', process.platform === 'win32' ? 'go.exe' : 'go');
  const go = fs.existsSync(local) ? local : 'go';
  fs.mkdirSync(path.join(root, 'bin'), { recursive: true });
  const binary = path.join(root, 'bin', process.platform === 'win32' ? 'kriemhild-test.exe' : 'kriemhild-test');
  const build = spawnSync(go, ['build', '-o', binary, './cmd/kriemhild'], { cwd: root, encoding: 'utf8' });
  assert.equal(build.status, 0, build.stderr || build.error?.message);
  server = spawn(binary, ['-addr', '127.0.0.1:18125'], { cwd: root, stdio: 'pipe' });
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(base + '/api/health')).ok) return; } catch {}
    await delay(50);
  }
  throw new Error('Test server did not start');
});
after(() => server?.kill());
async function api(route, body, method = 'POST', expected = 200) {
  const response = await fetch(base + '/api/' + route, { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = response.status === 204 ? null : await response.json();
  assert.equal(response.status, expected, JSON.stringify(data));
  return data;
}
function same(s, state, where) {
  assert.deepEqual(state.dom, Array.from(s.dom), where + ' domains');
  assert.deepEqual(state.locked, Array.from(s.locked), where + ' locked');
  assert.deepEqual(state.pinned, Array.from(s.pinned), where + ' pinned');
  for (const key of ['status', 'steps', 'backtracks', 'repairs', 'cleaned', 'settled']) assert.equal(state[key], s[key], `${where} ${key}`);
}
async function finish(id, s, state) {
  let guard = 200;
  while (state.status === 'running' && guard--) state = (await api(`sessions/${id}/step`, { count: 10000 })).state;
  let referenceGuard = s.N * 100;
  while (s.status === 'running' && referenceGuard--) s.step();
  same(s, state, 'finished');
  assert.equal(s.status, 'done');
  return state;
}
async function finishPhysical(state){
  const id=state.id;
  for(let k=0;k<200&&state.status==='running';k++)state=(await api(`sessions/${id}/step`,{count:10000})).state;
  assert.equal(state.status,'done');return state;
}
function assertWaterState(e){
  const f=e.fields;
  for(let i=0;i<e.mask.length;i++){
    for(const values of Object.values(f))assert.ok(Number.isFinite(values[i]));
    if(e.mask[i])assert.equal(f.waterDepth[i],0);
    else assert.ok(Math.abs(f.waterDepth[i]-(f.waterLevel[i]-f.elevation[i]))<.02);
    assert.equal(f.bathymetry[i],f.waterDepth[i]);
  }
}

for (const opts of [defaults, { ...defaults, seed: 0, selection: 'entropy', wrapX: true, radius2: 1 }, { ...defaults, seed: 4294967295, continents: null, climate: null, radius2: 4 }, { ...defaults, seed: 83471, climate: { strength: 3.1, layout: 'north' }, radius2: 5 }]) {
  test(`WFC parity: seed ${opts.seed}, ${opts.selection}, radius ${opts.radius2}`, async () => {
    const s = new reference.TerrainWFC.Solver(reference.TerrainWFC.compileRules(config), opts);
    let state = await api('sessions', { config, options: opts }, 'POST', 201);
    const { id } = state;
    same(s, state, 'initial');
    for (let i = 0; i < 12 && s.status === 'running'; i++) { s.step(); state = (await api(`sessions/${id}/step`, { count: 1 })).state; same(s, state, `step ${i}`); }
    state = await finish(id, s, state);
    for (let i = 0; i < 2; i++) { s.cleanup(); state = (await api(`sessions/${id}/cleanup`, { count: 1 })).state; same(s, state, 'cleanup'); }
    const snap = s.snapshot();
    const remote = await api(`sessions/${id}/snapshot`, {});
    const water = config.types.findIndex(t => t.id === 'water');
    const cells = [Math.floor(s.N / 2) + Math.floor(s.W / 2)];
    const painted = s.paint(cells, water);
    const result = await api(`sessions/${id}/paint`, { cells, type: water });
    assert.equal(result.painted, painted);
    await finish(id, s, result.state);
    s.restore(snap);
    same(s, (await api(`sessions/${id}/restore`, remote)).state, 'undo');
    await api(`sessions/${id}`, undefined, 'DELETE', 204);
  });
}

for (const seed of ['KRIEMHILD', '123456', '世界 🌎']) {
  test(`Continuous physical terrain is deterministic and water-aware: ${seed}`, async () => {
    const environment = { columns: 32, rows: 24, seed, landPercent: 42, rainfall: 1, plateCount: 12, continentCount: 12, selection: 'random', stability: 3 };
    const state = await api('sessions', { config, environment }, 'POST', 201);
    // Physical hydrology intentionally supersedes the source's elevation<0
    // water shortcut. Keep exact reference parity for WFC, test new invariants here.
    const duplicate=await api('sessions',{config,environment},'POST',201);
    assert.deepEqual(state.environment,duplicate.environment);
    assert.deepEqual(state.dom,duplicate.dom);
    await api(`sessions/${duplicate.id}`,undefined,'DELETE',204);
    assertWaterState(state.environment);
    await finishPhysical(state);
    const cleaned=(await api(`sessions/${state.id}/cleanup`,{count:1})).state;
    const before=cleaned.dom;
    const landType = config.types.findIndex(t => t.id === 'mountain');
    const rejected = await api(`sessions/${state.id}/paint`, { cells: [0], type: landType });
    assert.equal(rejected.painted, false);
    assert.deepEqual(rejected.state.dom, before);
    await api(`sessions/${state.id}`, undefined, 'DELETE', 204);
  });
}

test('All reference presets preserve rules-only generation', async () => {
  for (const preset of reference.TerrainPresets.LIST) {
    const palette = plain(reference.TerrainPresets.build(config, preset));
    const settings = preset.settings;
    const opts = { ...defaults, selection: 'entropy', seed: 531, radius2: settings.radius2, stability: settings.stability, continents: settings.continents ? { count: settings.continents.points, strength: settings.continents.strength } : null, climate: settings.climate };
    const s = new reference.TerrainWFC.Solver(reference.TerrainWFC.compileRules(palette), opts);
    const state = await api('sessions', { config: palette, options: opts }, 'POST', 201);
    await finish(state.id, s, state);
    await api(`sessions/${state.id}`, undefined, 'DELETE', 204);
  }
});

test('32nd terrain bit stays unsigned', async () => {
  const ids = Array.from({ length: 32 }, (_, i) => `t${i}`);
  const palette = { continents: [], climates: [], types: ids.map(id => ({ id, weight: 1, neighbors: ids })) };
  const opts = { ...defaults, continents: null, climate: null, stability: 0 };
  const s = new reference.TerrainWFC.Solver(reference.TerrainWFC.compileRules(palette), opts);
  const state = await api('sessions', { config: palette, options: opts }, 'POST', 201);
  await finish(state.id, s, state);
  assert.ok(Array.from(s.dom).includes(0x80000000));
  await api(`sessions/${state.id}`, undefined, 'DELETE', 204);
});

test('Contradiction backtracking and expanding repairs preserve operation order', async () => {
  // Three mutually-exclusive colors cannot tile 2x2 blocks with diagonal constraints.
  const palette = { continents: [], climates: [], types: ['a', 'b', 'c'].map(id => ({ id, neighbors: ['a', 'b', 'c'].filter(other => other !== id) })) };
  const opts = { ...defaults, continents: null, climate: null, seed: 9182, width: 16, height: 16 };
  const s = new reference.TerrainWFC.Solver(reference.TerrainWFC.compileRules(palette), opts);
  const initial = await api('sessions', { config: palette, options: opts }, 'POST', 201);
  for (let i = 0; i < 12 && s.status === 'running'; i++) {
    s.step();
    same(s, (await api(`sessions/${initial.id}/step`, { count: 1 })).state, `repair step ${i}`);
  }
  assert.ok(s.backtracks > 0 && s.repairs > 0);
  await api(`sessions/${initial.id}`, undefined, 'DELETE', 204);
});

test('Full-size physical globe and hot/dry northern hemisphere retain valid surfaces', async () => {
  for (const environment of [
    { columns: 160, rows: 100, seed: '4139746970', selection: 'random', stability: 3 },
    { columns: 48, rows: 24, seed: 'weather-edge', temperatureOffset: 25, rainfall: .1, landPercent: 75, latitudeSouth: 0, plateCount: 32, continentCount: 40 },
  ]) {
    const state = await api('sessions', { config, environment, wrapX: true }, 'POST', 201);
    assertWaterState(state.environment);
    await finishPhysical(state);
    await api(`sessions/${state.id}`, undefined, 'DELETE', 204);
  }
});

test('Full reference stability range is accepted, including slider maximum', async () => {
  const opts = { ...defaults, stability: 300 };
  const s = new reference.TerrainWFC.Solver(reference.TerrainWFC.compileRules(config), opts);
  const state = await api('sessions', { config, options: opts }, 'POST', 201);
  await finish(state.id, s, state);
  await api(`sessions/${state.id}`, undefined, 'DELETE', 204);
});

test('Physical continental markers are returned and renamed biomes retain their meaning', async () => {
  const palette = structuredClone(config);
  const renamed = palette.types.find(t => t.id === 'water');
  renamed.id = 'azure_sea'; renamed.name = 'Azure sea'; renamed.environmentType = 'water';
  for (const t of palette.types) {
    t.neighbors = t.neighbors.map(id => id === 'water' ? 'azure_sea' : id);
    if (t.weightNear?.water !== undefined) { t.weightNear.azure_sea = t.weightNear.water; delete t.weightNear.water; }
  }
  for (const z of palette.climates) if (z.types?.water !== undefined) { z.types.azure_sea = z.types.water; delete z.types.water; }
  const state = await api('sessions', { config: palette, environment: { columns: 32, rows: 24, seed: 'renamed', continentCount: 9 } }, 'POST', 201);
  assert.equal(state.contPoints.length, 9);
  assert.ok(state.contPoints.every(p => p.kind.name && p.kind.color));
  assert.equal(state.config.types.find(t => t.id === 'azure_sea').environmentType, 'water');
  await api(`sessions/${state.id}`, undefined, 'DELETE', 204);
});

test('API validates dimensions, references, brush bounds, expiry, and JSON', async () => {
  await api('sessions', { options: { ...defaults, width: -1 } }, 'POST', 400);
  await api('sessions', { environment: { columns: 256, rows: 256 } }, 'POST', 400);
  await api('sessions', { config: { types: [{ id: 'x', neighbors: ['missing'] }] }, options: defaults }, 'POST', 400);
  const state = await api('sessions', { options: defaults }, 'POST', 201);
  await api(`sessions/${state.id}/paint`, { type: 0, cells: [-1] }, 'POST', 400);
  await api(`sessions/${state.id}/step`, { count: 10001 }, 'POST', 400);
  await api(`sessions/${state.id}/restore`, { token: 'made-up' }, 'POST', 400);
  await api(`sessions/${state.id}`, undefined, 'DELETE', 204);
  await api(`sessions/${state.id}/step`, { count: 1 }, 'POST', 404);
  const malformed = await fetch(base + '/api/sessions', { method: 'POST', body: '{' });
  assert.equal(malformed.status, 400);
});

test('All eight presets layer their physical profiles and weights under both environment modes',async()=>{
  const {LIST,build,withEnvironmentTypes}=currentPresets.TerrainPresets;
  const summaries={};
  for(const realism of [false,true])for(const preset of LIST){
    const palette=plain(withEnvironmentTypes(build(config,preset),config));
    const environment={...plain(preset.environment),realism,columns:96,rows:64,seed:'preset-layers',continentCount:preset.settings.continents.points,stability:preset.settings.stability,radius2:preset.settings.radius2};
    const initial=await api('sessions',{config:palette,environment},'POST',201);
    assert.equal(initial.environment.options.radius2,preset.settings.radius2);
    assert.equal(initial.D,preset.settings.radius2===5?20:8);
    assert.equal(initial.environment.options.realism,realism);
    for(const [id,weight]of Object.entries(preset.rules.weights||{}))assert.equal(initial.config.types.find(t=>t.id===id).weight,weight);
    let state=initial;for(let k=0;k<100&&state.status==='running';k++)state=(await api(`sessions/${initial.id}/step`,{count:10000})).state;
    assert.equal(state.status,'done',`${preset.id} realism=${realism}`);
    const e=initial.environment,land=e.mask.reduce((a,b)=>a+b,0);
    const temperature=e.fields.temperature.reduce((a,b,i)=>a+(e.mask[i]?b:0),0)/land;
    summaries[`${preset.id}/${realism}`]={land,temperature};
    assert.ok(Math.abs(land/6144*100-preset.environment.landPercent)<5,`${preset.id}: coverage ${land/6144*100} differs from ${preset.environment.landPercent}`);
    await api(`sessions/${initial.id}`,undefined,'DELETE',204);
  }
  for(const mode of [false,true]){
    assert.ok(summaries[`islands/${mode}`].land<summaries[`continents/${mode}`].land);
    assert.ok(summaries[`frozen/${mode}`].temperature<summaries[`islands/${mode}`].temperature-20);
    assert.ok(summaries[`desert/${mode}`].temperature>summaries[`earth/${mode}`].temperature);
  }
});


test('Detail API is deterministic, bounded and preserves the parent world', async()=>{
  const state=await api('sessions',{config,environment:{columns:64,rows:48,seed:'detail-api',realism:true}},'POST',201);
  const route=`sessions/${state.id}/detail`;
  const a=await api(`${route}/3/3/3`,undefined,'GET');
  await api(`${route}/6/12/13`,undefined,'GET');
  assert.deepEqual(a,await api(`${route}/3/3/3`,undefined,'GET'));
  const b=await api(`${route}/3/4/3`,undefined,'GET');
  for(let row=0;row<33;row++)assert.deepEqual(a.points[row*33+32],b.points[row*33]);
  const rootTile=await api(`${route}/0/0/0`,undefined,'GET');
  for(let y=0;y<33;y++)for(let x=0;x<33;x++)assert.equal(rootTile.points[y*33+x].elevation,state.environment.fields.elevation[y*64+x]);
  await api(`${route}/9/0/0`,undefined,'GET',400);
  await api(`${route}/2/999/0`,undefined,'GET',400);
  await api(`sessions/${state.id}`,undefined,'DELETE',204);
  await api(`${route}/0/0/0`,undefined,'GET',404);
});
