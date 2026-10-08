import './helpers.js';

async function request(path, body, method = 'POST') {
  let response;
  try {
    response = await fetch(`/api/${path}`, { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
  } catch {
    throw new Error('Cannot reach the KRIEMHILD Go server. Start it and generate again.');
  }
  if (response.status === 204) return;
  const data = await response.json();
  if (!response.ok) {const error=new Error(data.error || `Server error (${response.status})`);error.status=response.status;throw error;}
  return data;
}

// Read-only solver mirror for rendering/hover. Every mutation is executed in Go.
export class RemoteSolver extends globalThis.TerrainWFC.SolverView {
  static async create(payload) { return new RemoteSolver(await request('sessions', payload)); }
  static async savedProjects(){return request('projects',undefined,'GET');}
  static async openSavedProject(id){return new RemoteSolver(await request(`projects/${encodeURIComponent(id)}/open`,{}));}
  async autosave(payload) {
    await this.pending;
    const result=await request(`sessions/${this.id}/autosave`,payload);
    if(!result.saved)throw new Error('Server persistence is disabled.');
    this.worldId=result.worldId;this.autosaveError=null;return result;
  }
  static async importProject(files,folder=false) {
    let body=files[0],headers={'Content-Type':'application/zip'};
    if(folder){body=new FormData();for(const file of files)body.append(file.webkitRelativePath||file.name,file,file.name);headers={};}
    const response=await fetch('/api/projects/import',{method:'POST',headers,body});
    const state=await response.json();if(!response.ok)throw new Error(state.error||'Cannot open this world project');
    return new RemoteSolver(state);
  }
  async saveProject(ui) {
    await this.pending;
    await Promise.all([...this.detailStores||[]].map(store=>store.whenIdle()));
    const response=await fetch(`/api/sessions/${this.id}/project`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ui})});
    if(!response.ok){const error=await response.json();throw new Error(error.error||'World project could not be saved');}
    return response.blob();
  }
  constructor(state) {
    super();
    Object.assign(this, state);
    this.contShare = state.contShare ? Float32Array.from(state.contShare) : null;
    this.climShare = state.climShare ? Float32Array.from(state.climShare) : null;
    this.rules = globalThis.TerrainWFC.compileRules(state.config);
    this.nearCounts = new Int32Array(this.T);
    this.dirtyCells = new Set();
    this.pending = Promise.resolve();
    this.apply(state);
    this.markAllDirty();
  }
  apply(state) {
    const previous = this.dom;
    Object.assign(this, state);
    if ('contShare' in state) this.contShare = state.contShare ? Float32Array.from(state.contShare) : null;
    if ('climShare' in state) this.climShare = state.climShare ? Float32Array.from(state.climShare) : null;
    this.dom = Uint32Array.from(state.dom);
    for (let c = 0; c < this.N; c++) if (!previous || previous[c] !== this.dom[c]) this.dirtyCells.add(c);
  }
  action(action, body) {
    const run = this.pending.then(async () => {
      const result = await request(`sessions/${this.id}/${action}`, body);
      if (result.state) this.apply(result.state);
      if(result.autosaveError){this.autosaveError=result.autosaveError;throw new Error(result.autosaveError);}
      return result;
    });
    this.pending = run.catch(() => {});
    return run;
  }
  async step(count = 1) { await this.action('step', { count }); return this.status; }
  async cleanup(count = 1) { return (await this.action('cleanup', { count })).changed; }
  snapshot() { return this.action('snapshot', {}).then(result => result.token); }
  async restore(snapshot) { await this.action('restore', { token: await snapshot }); }
  async paint(cells, type) {
    const result = await this.action('paint', { cells, type });
    if (result.painted) while (this.status === 'running') await this.step(10000);
    return result.painted;
  }
  dispose() { return request(`sessions/${this.id}`, undefined, 'DELETE').catch(() => {}); }
  typeAt(c) { const m = this.dom[c]; return m && !(m & (m - 1)) ? 31 - Math.clz32(m) : -1; }
  optionsFor(c) {
    const choices=super.optionsFor(c);
    if(!this.environment?.options.realism)return choices;
    const f=this.environment.fields,clamp=v=>Math.max(0,Math.min(1,v));
    const factors={forest:.1+f.moisture[c]*f.growingSeason[c]*3,meadow:.1+f.moisture[c]*clamp(1-Math.abs(f.temperature[c]-15)/25),grass:1.2-f.moisture[c]*.7,dunes:.1+f.dune[c]*8,reef:.1+f.reef[c]*12,mesa:.1+f.mesa[c]*5};
    for(const choice of choices){const type=this.config.types[choice.type];choice.weight*=factors[type.environmentType||type.id]??1;}
    return choices;
  }
  zoneAt(c) {
    if (!this.climShare) return -1;
    let best = 0;
    for (let z = 1; z < this.Z; z++) if (this.climShare[c * this.Z + z] > this.climShare[c * this.Z + best]) best = z;
    return best;
  }
  markAllDirty() { for (let c = 0; c < this.N; c++) this.dirtyCells.add(c); }
  consumeDirty(fn) { for (const c of this.dirtyCells) fn(c, this.dom[c]); this.dirtyCells.clear(); }
}
