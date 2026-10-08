// Environmental data comes from Go. These functions only visualize it.
const climateColors = ['#245e94','#136d38','#32904b','#a7b858','#d9b06a','#b9a58b','#c6b769','#64b9a3','#77a16a','#406d62','#acbbb3','#e6f4fa','#a8adc5'];
const ranges = {
  temperature:[-40,40],summer:[-40,40],winter:[-40,40],elevation:[-12000,8000],
  drainageElevation:[-12000,8000],bathymetry:[0,12000],precipitation:[0,3000],
  summerRain:[0,1800],winterRain:[0,1800],snowBalance:[-2400,2400],
  latitude:[-90,90],current:[-3,3],windX:[-1,1],windY:[-1,1],
  duneOrientation:[-Math.PI,Math.PI*1.5],oceanDistance:[0,30],freshwaterDistance:[0,20],
  salinity:[0,35],waterLevel:[-1000,5000],waterDepth:[0,12000],catchmentArea:[0,1000],geology:[0,6],landform:[0,8],reefType:[0,4],
};
export function installEnvironmentOverlays(select) {
  const fields = ['climate','wind','latitude','current','windY','slope','oceanDistance','tectonicStress','summerRain','winterRain','growingSeason','drainageElevation','river','freshwaterDistance','rockType','layering','caprock','erosion','sandSupply','sandTransport','deposition','vegetation','substrate','snowBalance','duneField','duneOrientation','hotspot','vent','cultivated','village','ocean','waterLevel','waterDepth','waterBody','basin','catchmentArea','shelf','seamount','reefType','light','landform','highland','mountainCore','geology'];
  for(const name of fields) if(![...select.options].some(o=>o.value===name)) {
    const option=document.createElement('option'); option.value=name;
    option.textContent=name==='climate'?'Derived climate zones':name==='wind'?'Wind direction (arrows)':name.replace(/([A-Z])/g,' $1').replace(/^./,c=>c.toUpperCase());
    select.append(option);
  }
  const legend=document.createElement('p');legend.id='environmentLegend';legend.className='hint';select.after(legend);
}
export function fieldColor(environment,name,c) {
  const f=environment?.fields;
  if(name==='wind') name='windStrength';
  if(!f?.[name]) return null;
  const v=f[name][c];
  if(name==='climate') return climateColors[v] || '#777777';
  if(name==='rockType') return ['#97919d','#c4ac7a','#695765'][v];
  if(name==='boundary') return ['#18344d','#e77355','#69cbbb','#deb968'][v];
  if(name==='geology')return ['#18344d','#95ba7e','#d78968','#aa9691','#828755','#c4aa78','#c45c45'][v];
  if(name==='reefType')return ['#18344d','#51d8a2','#55b8df','#db97cf','#f1cf63'][v];
  if(name==='landform')return ['#245e94','#a6c76b','#769a58','#6c8053','#bda274','#8a8580','#e1e9ec','#997c60','#c8a583'][v];
  if(['plate','duneField','vent','waterBody','basin'].includes(name)) return `hsl(${v*137.508%360} 48% ${v===0&&name!=='plate'?18:56}%)`;
  let t;
  if(name==='flow'||name==='accumulation') t=Math.log1p(Math.max(0,v))/8;
  else { const [lo,hi]=ranges[name]||[0,1];t=(v-lo)/(hi-lo); }
  t=Math.max(0,Math.min(1,t));
  return `rgb(${Math.round(40+200*t)},${Math.round(80+100*(1-Math.abs(t-.5)*2))},${Math.round(200-160*t)})`;
}
export function overlayLegend(e,name) {
  if(name==='terrain') return e?.options.realism?'Rivers in blue · dune ridges follow wind · volcanic vents in red.':'';
  if(name==='wind') return 'Arrows show prevailing wind direction; color shows strength (blue → red).';
  if(!e?.fields[name]) return 'This layer requires Real-world geology & climate.';
  if(name==='climate') return e.climateZones.map((v,i)=>`${i}: ${v}`).join(' · ');
  if(name==='rockType') return '0: crystalline · 1: sedimentary · 2: volcanic';
  if(name==='boundary') return '0: interior · 1: convergent · 2: divergent · 3: transform';
  if(name==='geology')return '0: ocean basin ? 1: passive margin/lowland ? 2: active margin ? 3: young range ? 4: eroded range ? 5: plateau ? 6: volcanic edifice';
  if(name==='reefType')return '0: none · 1: fringing · 2: barrier · 3: patch · 4: atoll';
  if(name==='landform')return '0: water · 1: lowland · 2: plain · 3: hills · 4: plateau · 5: mountains · 6: peaks · 7: basin/valley · 8: dry below-sea-level depression';
  if(['plate','duneField','vent','waterBody','basin'].includes(name)) return 'Distinct colors identify separate entities. Hover for the ID.';
  const range=ranges[name]||[0,1];
  if(name==='accumulation')return 'Accumulated runoff, logarithmic color scale. Hover for discharge in relative units.';
  return `${name.replace(/([A-Z])/g,' $1')}: blue ${range[0]} → red ${range[1]}. Hover for exact values.${name==='flow'?' Values are downstream cell indices.':''}`;
}
export function drawPhysicalEntities(ctx,solver,vor,overlay) {
  const e=solver.environment;if(!e) return;
  const f=e.fields,sx=ctx.canvas.width/solver.W,sy=ctx.canvas.height/solver.H;
  const point=c=>[(vor?.ptX[c]??c%solver.W+.5)*sx,(vor?.ptY[c]??Math.floor(c/solver.W)+.5)*sy];
  ctx.save();ctx.lineCap='round';
  if(overlay==='wind') {
    ctx.strokeStyle='#f3f7fd';ctx.lineWidth=1;
    const step=Math.max(3,Math.floor(solver.W/35));
    for(let y=1;y<solver.H;y+=step) for(let x=1;x<solver.W;x+=step) {
      const c=y*solver.W+x,[px,py]=point(c),a=Math.atan2(f.windY[c],f.windX[c]);
      const length=Math.min(sx,sy)*step*.7,ex=px+Math.cos(a)*length,ey=py+Math.sin(a)*length;
      ctx.beginPath();ctx.moveTo(px,py);ctx.lineTo(ex,ey);
      ctx.moveTo(ex-Math.cos(a-.6)*3,ey-Math.sin(a-.6)*3);ctx.lineTo(ex,ey);ctx.lineTo(ex-Math.cos(a+.6)*3,ey-Math.sin(a+.6)*3);ctx.stroke();
    }
  }
  if(e.options.realism && (overlay==='terrain'||overlay==='river'||overlay==='accumulation')) {
    ctx.strokeStyle='#62c7ed';
    for(const r of e.entities.rivers||[]) {
      ctx.lineWidth=Math.max(.7,Math.min(3,Math.log1p(r.discharge)*.4));ctx.beginPath();
      r.path.forEach((c,i)=>{const [x,y]=point(c);if(i===0)ctx.moveTo(x,y);else ctx.lineTo(x,y);});ctx.stroke();
    }
  }
  if(e.options.realism && overlay==='terrain') {
    ctx.strokeStyle='rgba(113,75,34,.65)';ctx.lineWidth=.8;
    for(const region of e.entities.duneFields||[]) for(const c of region.cells) {
      const t=solver.typeAt(c),type=solver.rules.types[t];if((type?.environmentType||type?.id)!=='dunes')continue;
      const [x,y]=point(c),a=f.duneOrientation[c],dx=Math.cos(a)*sx*.35,dy=Math.sin(a)*sy*.35;
      ctx.beginPath();ctx.moveTo(x-dx,y-dy);ctx.lineTo(x+dx,y+dy);ctx.stroke();
    }
    ctx.fillStyle='#ff522b';ctx.strokeStyle='#502b24';ctx.lineWidth=.7;
    for(const v of e.entities.volcanoes||[]) if(e.mask[v.cell]) {
      ctx.fillStyle=v.active?'#ff522b':'#937069';
      const [x,y]=point(v.cell),r=Math.max(2,Math.min(4,sx*.6));ctx.beginPath();ctx.moveTo(x,y-r);ctx.lineTo(x-r,y+r);ctx.lineTo(x+r,y+r);ctx.closePath();ctx.fill();ctx.stroke();
    }
  }
  ctx.restore();
}
