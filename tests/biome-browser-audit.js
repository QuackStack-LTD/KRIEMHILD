// Live controls, categorical detail and inspection audit; use a disposable server.
if(typeof document==='undefined'){
 import('./run-terrain-browser-audit.mjs').then(({runTerrainAudit})=>runTerrainAudit(process.argv[1])).catch(error=>{console.error(error);process.exitCode=1;});
}else (async()=>{
 const $=id=>document.getElementById(id),checks=[],original=window.fetch;
 const assert=(ok,message)=>{if(!ok)throw new Error(message)};
 const wait=async(fn,label)=>{const end=Date.now()+60000;while(!fn()&&Date.now()<end)await new Promise(r=>setTimeout(r,40));assert(fn(),label)};
 let state;
 window.fetch=async(...args)=>{const response=await original.apply(window,args);if(String(args[0])==='/api/sessions')state=await response.clone().json();return response};
 try{
  $('shape').value='flat';$('width').value=64;$('height').value=40;$('seed').value='KRIEMHILD';$('instant').checked=true;
  $('majorContinents').value='5';$('oceanSeparation').value='wide';$('landCoverage').value='42';$('environment').checked=true;$('realism').checked=true;
  $('generate').click();await wait(()=>state,'New generation response');await wait(()=>$('statusLine').textContent.startsWith('Done'),'Finished generation');
  assert(!state.error,state.error);assert(!$('error').textContent,$('error').textContent);
  const e=state.environment,f=e.fields;
  assert(e.options.majorContinents===5&&e.options.oceanSeparation==='wide','Controls did not reach the physical generator');
  assert(e.continentalLayout.count===5&&Math.abs(e.continentalLayout.gapCells-4.8)<.001,'Layout ignored selected count/gap');
  assert($('contPoints').disabled,'Legacy point count enabled in physical mode');
  checks.push('Major continent count and wide ocean separation reach the physical layout; legacy count stays separate');
  for(const name of ['aridityIndex','biome','rainfallPotential','rainShadow','moistureSupply'])assert([...$('environmentOverlay').options].some(o=>o.value===name),'Missing diagnostic '+name);
  const names={3:'Semi-arid steppe',4:'Xeric scrubland'},hovered=[];
  $('fitMap').click();await new Promise(r=>setTimeout(r,100));
  const canvas=$('map'),detail=$('detailMap'),bounds=canvas.getBoundingClientRect(),before=[bounds.width,bounds.height,document.documentElement.scrollWidth,document.documentElement.scrollHeight];
  for(const [id,name]of Object.entries(names)){
   const i=f.biome.findIndex((v,i)=>v===Number(id)&&!f.waterBody[i]);assert(i>=0,'Missing test biome '+name);
   const x=i%64,y=Math.floor(i/64),r=detail.getBoundingClientRect();
   canvas.dispatchEvent(new MouseEvent('mousemove',{clientX:r.left+Number(detail.dataset.offsetX)+x*Number(detail.dataset.scale),clientY:r.top+Number(detail.dataset.offsetY)+y*Number(detail.dataset.scale),bubbles:true}));
   await new Promise(resolve=>setTimeout(resolve,50));
   assert($('hover').textContent.includes(name),'Inspection contradicts '+name+': '+$('hover').textContent);
   for(const label of ['AI ','summer/winter rain','mm/yr'])assert($('hover').textContent.includes(label),'Missing hover diagnostic '+label);
   hovered.push($('hover').textContent);
   const response=await original(`/api/sessions/${state.id}/detail/2/${Math.floor(x/8)}/${Math.floor(y/8)}`);
   assert(response.ok,'Detail tile failed');const tile=await response.json();
   const point=tile.points[(y%8)*4*33+(x%8)*4];assert(point.biome===({3:'steppe',4:'scrubland'}[id]),'Detail changed parent biome');
  }
  const after=canvas.getBoundingClientRect();assert(JSON.stringify(before)===JSON.stringify([after.width,after.height,document.documentElement.scrollWidth,document.documentElement.scrollHeight]),'Hover expanded the view');
  checks.push('Steppe and scrubland agree between base fields, actual detail tiles and hover labels');checks.push('Hover reports the water budget and keeps canvas/document dimensions fixed');
  const saved=JSON.parse(localStorage.getItem('kriemhild.settings.v1'));assert(saved.majorContinents==='5'&&saved.oceanSeparation==='wide','New controls not persisted');
  window.__biomeAudit={checks,hovered,layout:e.continentalLayout};
 }finally{window.fetch=original}
})()
