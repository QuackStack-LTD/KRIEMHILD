// Run this expression with the T3 collaborative preview's evaluate tool.
// Requests go to the actual Go server; the scheduler substitution only lets the
// audit finish when the host pauses animation frames or throttles nested timers.
(async()=>{
  const $=id=>document.getElementById(id),checks=[],calls=[];
  const assert=(value,message)=>{if(!value)throw new Error(message)};
  const original={fetch:window.fetch,confirm:window.confirm,raf:window.requestAnimationFrame,caf:window.cancelAnimationFrame};
  const schedule=fn=>{const ch=new MessageChannel();ch.port1.onmessage=()=>{ch.port1.close();ch.port2.close();fn()};ch.port2.postMessage(0);return ch};
  const yieldTask=()=>new Promise(resolve=>schedule(resolve));
  window.requestAnimationFrame=fn=>schedule(()=>fn(performance.now()));
  window.cancelAnimationFrame=ch=>{if(ch?.port1){ch.port1.close();ch.port2.close()}else original.caf.call(window,ch)};window.confirm=()=>true;
  window.fetch=async(...args)=>{const response=await original.fetch.apply(window,args);if(String(args[0])==='/api/sessions')calls.push(await response.clone().json());return response};
  const change=(id,value)=>{const el=$(id);if(el.type==='checkbox')el.checked=value;else el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}))};
  const session=async(action)=>{
    const n=calls.length;action();const until=Date.now()+15000;
    while(calls.length===n&&Date.now()<until)await yieldTask();
    assert(calls.length>n,'Missing API generation');const state=calls.at(-1);assert(!state.error,state.error);
    while(!$('statusLine').textContent.startsWith('Done')&&Date.now()<until)await yieldTask();
    assert(!$('error').textContent,$('error').textContent);assert($('statusLine').textContent.startsWith('Done'),'Generation failed to finish');return state;
  };
  try{
    $('width').value='64';$('height').value='40';$('seed').value='KRIEMHILD';$('instant').checked=true;
    await session(()=>change('preset','earth'));
    let state=await session(()=>change('realism',true));
    assert($('preset').value==='earth','Enabling realism discarded preset');
    assert($('environment').checked&&state.environment.version==='kriemhild-realism-v2','Switch did not enable realism');
    assert(state.environment.fields.climate.length===2560&&state.environment.entities.rivers.length>0,'Missing climate/rivers');
    checks.push('Switch enables real Go environmental pipeline and completes generation');
    const map=$('map'),ctx=map.getContext('2d');
    const hash=()=>ctx.getImageData(0,0,map.width,map.height).data.reduce((a,b)=>(a*31+b)>>>0,0);
    const terrain=hash();change('environmentOverlay','climate');assert(hash()!==terrain,'Climate overlay unchanged');
    assert($('environmentLegend').textContent.includes('Rainforest'),'Missing categorical legend');
    change('environmentOverlay','wind');assert($('environmentLegend').textContent.includes('Arrows'),'Missing wind legend');
    for(const field of Object.keys(state.environment.fields))assert([...$('environmentOverlay').options].some(o=>o.value===field),'Uninspectable field: '+field);
    checks.push('Every environmental field has an overlay; climate and wind render');
    state=await session(()=>change('realism',false));assert(state.environment.version==='kriemhild-environment-v3'&&!state.environment.entities,'Off did not restore baseline physics');
    state=await session(()=>change('environment',false));assert(!state.environment&&!$('realism').checked,'Terrain-rule mode not restored');
    checks.push('Both earlier modes remain available');
    const simple=window.TerrainPresets.LIST.find(p=>p.rules.onlyTypes);
    assert(simple,'Missing simple palette preset');await session(()=>change('preset',simple.id));
    state=await session(()=>change('realism',true));assert(state.config.types.length>=23,'Missing environmental palette extension');
    checks.push('Realism fills missing biome types when enabled from a reduced preset');
    for(const realism of [false,true]){
      await session(()=>change('realism',realism));
      for(const preset of window.TerrainPresets.LIST){
        state=await session(()=>change('preset',preset.id));
        assert($('preset').value===preset.id&&$('environment').checked&&$('realism').checked===realism,'Preset reset a physical mode');
        assert(state.environment.options.realism===realism,'Mode did not reach Go');
        for(const [key,value]of Object.entries(preset.environment))assert(state.environment.options[key]===value,preset.id+' ignores '+key);
        assert(state.environment.options.radius2===preset.settings.radius2,'Neighborhood ignored');
        for(const [id,weight]of Object.entries(preset.rules.weights||{}))assert(state.config.types.find(t=>t.id===id).weight===weight,preset.id+' ignores terrain weight');
      }
    }
    checks.push('All eight presets compose with both physical modes and preserve profiles, weights and radius');
    await session(()=>change('preset',simple.id));
    state=await session(()=>change('environment',false));
    assert($('preset').value===simple.id&&state.config.types.length===simple.rules.onlyTypes.length,'Disabling physics did not restore reduced preset');
    checks.push('Disabling physics retains the preset and restores its original palette');
    await session(()=>change('preset','earth'));await session(()=>change('realism',true));
    change('environmentOverlay','terrain');
    const saved=JSON.parse(localStorage.getItem('kriemhild.settings.v1'));assert(saved.realism&&saved.environment&&localStorage.getItem('kriemhild.preset')==='earth','Preset/switches not persisted');
    checks.push('Realism setting persisted for reload');
    window.__realismAudit={checks,status:$('statusLine').textContent};
  }finally{window.fetch=original.fetch;window.confirm=original.confirm;window.requestAnimationFrame=original.raf;window.cancelAnimationFrame=original.caf}
})()
