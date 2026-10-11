// A deterministic cartographic renderer. No editor canvas, camera, selection or
// viewport entity list is an input. All dimensions below are SVG page units.
export const MEDIEVAL_VERSION='medieval-3';
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&apos;'}[c]));
const num=n=>Number(n.toFixed(3));
const clamp=(v,a,b)=>Math.max(a,Math.min(b,v));
// Round only sub-page-unit contour corners, within the source sample footprint.
function contour(points,closed=false){if(points.length<3)return path(points);const pairs=points.map((p,i)=>{const a=points[(i+points.length-1)%points.length],z=points[(i+1)%points.length];if(!closed&&(i===0||i===points.length-1))return [p,p];const left=Math.hypot(p[0]-a[0],p[1]-a[1]),right=Math.hypot(z[0]-p[0],z[1]-p[1]),r=Math.min(2,left*.3,right*.3);return [[p[0]+(a[0]-p[0])*r/(left||1),p[1]+(a[1]-p[1])*r/(left||1)],[p[0]+(z[0]-p[0])*r/(right||1),p[1]+(z[1]-p[1])*r/(right||1)]]});let out=path([pairs[0][1]]);for(let k=1;k<points.length+(closed?1:0);k++){const i=k%points.length;out+=`L${pairs[i][0].map(num).join(',')}Q${points[i].map(num).join(',')} ${pairs[i][1].map(num).join(',')}`}return out+(closed?'Z':'');}
const key=p=>`${p[0].toFixed(5)},${p[1].toFixed(5)}`;
const path=p=>p.map((q,i)=>`${i?'L':'M'}${num(q[0])},${num(q[1])}`).join('');
const hash=(x,y)=>{let n=Math.imul(x+149,374761393)^Math.imul(y+311,668265263);n=Math.imul(n^(n>>>13),1274126177);return (n>>>0)/4294967295};
const inside=(p,b)=>p[0]>=b.x&&p[0]<=b.x+b.width&&p[1]>=b.y&&p[1]<=b.y+b.height;
const polygonContains=(p,poly)=>{let yes=false;for(let i=0,j=poly.length-1;i<poly.length;j=i++){const a=poly[i],b=poly[j];if((a[1]>p[1])!==(b[1]>p[1])&&p[0]<(b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0])yes=!yes;}return yes};

// Anchors stay on the feature even for concave settlement boundaries.
export function featureAnchor(points,geometry){
 if(points.length===1)return points[0];
 if(geometry==='line'){const lengths=points.slice(1).map((p,i)=>Math.hypot(p[0]-points[i][0],p[1]-points[i][1]));let half=lengths.reduce((a,b)=>a+b,0)/2;for(let i=0;i<lengths.length;i++){if(half<=lengths[i]){const t=half/(lengths[i]||1);return points[i].map((v,k)=>v+(points[i+1][k]-v)*t)}half-=lengths[i]}return points[0];}
 const mean=[0,1].map(k=>points.reduce((sum,p)=>sum+p[k],0)/points.length);
 if(polygonContains(mean,points))return mean;
 const low=Math.min(...points.map(p=>p[1])),high=Math.max(...points.map(p=>p[1]));let best=null;
 for(let row=1;row<20;row++){const y=low+(high-low)*row/20,xs=[];for(let i=0,j=points.length-1;i<points.length;j=i++){const a=points[j],b=points[i];if((a[1]>y)!==(b[1]>y))xs.push(a[0]+(y-a[1])*(b[0]-a[0])/(b[1]-a[1]));}xs.sort((a,b)=>a-b);for(let i=1;i<xs.length;i+=2){const width=xs[i]-xs[i-1];if(!best||width>best.width)best={width,p:[(xs[i]+xs[i-1])/2,y]};}}
 return best?.p||points[0];
}
export function wrapPlaceName(name,maxChars=20){
 const words=name.trim().split(/\s+/),lines=[];let line='';
 for(const word of words){if(line&&line.length+word.length+1>maxChars){lines.push(line);line=word}else line+=(line?' ':'')+word;}if(line)lines.push(line);return lines;
}

const stateKinds=new Set(['country','state','kingdom','empire']);
const settlementKinds=new Set(['city','town','village','capital']);
// Use saved affiliations and actual containment, never proximity or labels.
// This is cartographic aggregation only; it does not invent political authority.
export function illustrationEntities(data){
 const logical=new Map((data.logicalEntities||[]).filter(e=>!e.deleted).map(e=>[e.id,e]));
 const entities=(data.entities||[]).filter(e=>!e.deleted).map(e=>({...e,name:logical.get(e.logicalId||e.id)?.name||e.name}));
 const identity=e=>e.logicalId||e.id,byID=new Map(entities.map(e=>[e.id,identity(e)]));
 const states=entities.filter(e=>stateKinds.has(e.kind)&&e.points?.length>=3&&e.geometry==='polygon');
 const parents=new Map(),add=(child,parent)=>{child=byID.get(child)||child;parent=byID.get(parent)||parent;if(child===parent)return;if(!parents.has(child))parents.set(child,new Set());parents.get(child).add(parent)};
 for(const e of entities)if(e.parentId)add(identity(e),e.parentId);
 for(const r of data.relations||[]){
  if(data.historicalMap?.ageId&&r.ageId&&r.ageId!==data.historicalMap.ageId)continue;
  if(['part-of','contained-by','located-within','subordinate-to','capital-of'].includes(r.kind))add(r.source,r.destination);
  if(['contains','governs','controls'].includes(r.kind))add(r.destination,r.source);
 }
 const area=e=>Math.abs(e.points.reduce((a,p,i)=>{const q=e.points[(i+1)%e.points.length];return a+p[0]*q[1]-q[0]*p[1]},0))/2;
 for(const e of entities){if(!settlementKinds.has(e.kind)&&!stateKinds.has(e.kind))continue;
  for(const s of states)if(identity(e)!==identity(s)&&e.points?.length&&(!stateKinds.has(e.kind)||area(s)>area(e)*1.001)&&e.points.every(p=>polygonContains(p,s.points)))add(identity(e),identity(s));
 }
 const focus=data.historicalMap?.entityId,focusKind=logical.get(focus)?.kind||entities.find(e=>identity(e)===focus)?.kind;
 const detailed=!!data.historicalMap?.parentId||stateKinds.has(focusKind)||settlementKinds.has(focusKind)||data.bounds.width/Math.max(1,data.terrainWidth-1)<.25;
 if(detailed)return {entities,detailed};
 const available=new Set(states.filter(s=>s.name&&!/^example-/i.test(s.name.trim())).map(identity));
 const hidden=e=>{const own=identity(e),seen=new Set([own]),queue=[...(parents.get(own)||[])];while(queue.length){const id=queue.shift();if(seen.has(id))continue;seen.add(id);if(available.has(id))return true;queue.push(...(parents.get(id)||[]))}return false};
 return {entities:entities.filter(e=>!e.name||/^example-/i.test(e.name.trim())||!(stateKinds.has(e.kind)||settlementKinds.has(e.kind))||!hidden(e)),detailed};
}

// Restrict the label anchor to the visible portion of a political polygon.
function clipPolygon(points,b){
 let out=points;
 for(const [axis,value,greater] of [[0,b.x,true],[0,b.x+b.width,false],[1,b.y,true],[1,b.y+b.height,false]]){
  const input=out;out=[];for(let i=0;i<input.length;i++){const a=input[i],z=input[(i+1)%input.length],ia=greater?a[axis]>=value:a[axis]<=value,iz=greater?z[axis]>=value:z[axis]<=value;if(ia)out.push(a);if(ia!==iz){const t=(value-a[axis])/(z[axis]-a[axis]);out.push([a[0]+(z[0]-a[0])*t,a[1]+(z[1]-a[1])*t])}}
 }return out;
}

// Convert SI width through the local projection metric. Physical width controls
// the stroke; class/length affect visibility and label priority, never SI width.
export function riverMetrics(f,index,context,reach={},system={}) {
 const p=f.path[index],prev=f.path[Math.max(0,index-1)],next=f.path[Math.min(f.path.length-1,index+1)];
 const geo=context.geography,scale=context.scale;
 const q=Math.max(0,f.dischargesM3s?.[index]??f.dischargeM3s??reach.dischargeM3s??0);
 const dx=next[0]-prev[0],dy=next[1]-prev[1],len=Math.hypot(dx,dy)||1;
 const kmY=geo?.world_scale||0;
 const lat=geo?(geo.latitude_north+(geo.latitude_south-geo.latitude_north)*(p[1]+.5)/context.terrainHeight)*Math.PI/180:0;
 const kmX=geo?geo.planetary_radius*(geo.longitude_east-geo.longitude_west)*Math.PI/180/context.terrainWidth*Math.max(.02,Math.cos(lat)):0;
 const metresPerNormal=1000*Math.hypot(kmX*dy/len,kmY*dx/len);
 const local=f.widths?.[index]??f.width??reach.width??0;
 const reference=f.physicalWidthMetres||reach.widthMetres||0;
 // Widths are already projected grid widths. SI metadata anchors old profiles
 // and supports missing Widths without a global arbitrary minimum.
 const metres=local>0&&reference>0&&f.width>0?reference*local/f.width:reference||(metresPerNormal>0?local*metresPerNormal:0);
 const physical=local>0?local*scale:metresPerNormal>0?metres/metresPerNormal*scale:0;
 const rank=f.class||reach.class||system.class||'stream';
 const localView=context.bounds.width/Math.max(1,context.terrainWidth-1)<.075;
 const prominence=rank==='major'||rank==='regional'||(system.lengthKm||0)>1200;
 const visible=localView||prominence||physical>=.22||q>=180;
 const proxy=metres>0?metres:Math.max(.5,local*100);
 // Logarithmic optical exaggeration retains a broad distribution, rather than
 // clamping every subpixel river to the same minimum. It fades at local scale.
 const optical=.08+.18*Math.log1p(proxy/2)+.065*Math.log1p(q/8);
 const width=visible?Math.max(physical,optical*(localView?.6:1)):0;
 const slope=Math.max(0,f.grade||0);
 const depth=f.depth>0?f.depth:q>0?clamp(.24*Math.pow(q,.28)/Math.pow(1+slope,.08),.08,28):0;
 const incision=reach.incisionMetres||0;
 const valley=reach.valleyWidthMetres||f.valleyWidthMetres||0;
 const canyon=incision>=25 && (!valley||valley/incision<28) && !/floodplain|delta|braid|anabranch/.test(f.morphology||reach.morphology||'');
 return {width,physical,metres,depth,incision,canyon,visible,rank,q};
}

// Clip each terrain triangle to the inherited water footprint, cancel shared
// edges, then stitch rings. Inland lakes remain separate from marine basins.
export function waterGeometry(data,transform) {
 const {columns:w,rows:h,samples:s,bounds:b}=data,edges=new Map(),coasts=[];
 const point=(x,y)=>[b.x+x*b.width/(w-1),b.y+y*b.height/(h-1),s[y*w+x][0]-.5];
 const edge=(a,z)=>{const ka=key(a),kz=key(z),rev=kz+'|'+ka,k=ka+'|'+kz;if(ka===kz)return;if(edges.has(rev))edges.delete(rev);else edges.set(k,[a,z]);};
 const cut=(a,z)=>{const t=a[2]/(a[2]-z[2]);return [a[0]+(z[0]-a[0])*t,a[1]+(z[1]-a[1])*t,0]};
 for(let y=0;y<h-1;y++)for(let x=0;x<w-1;x++){
  const a=point(x,y),c=point(x+1,y),d=point(x+1,y+1),e=point(x,y+1);
  for(const triangle of [[a,c,d],[a,d,e]]) {const poly=[],cross=[];for(let i=0;i<3;i++){const p=triangle[i],q=triangle[(i+1)%3],pi=p[2]>=0,qi=q[2]>=0;if(pi)poly.push(p);if(pi!==qi){const v=cut(p,q);poly.push(v);cross.push(v);}}if(poly.length>=3)for(let i=0;i<poly.length;i++)edge(poly[i],poly[(i+1)%poly.length]);if(cross.length===2)coasts.push(cross);}
 }
 const starts=new Map();for(const [id,[a,z]] of edges){const k=key(a);if(!starts.has(k))starts.set(k,[]);starts.get(k).push(id)}
 const rings=[];while(edges.size){const [id,[first]]=edges.entries().next().value;let at=id,ring=[];while(at&&edges.has(at)){const [a,z]=edges.get(at);edges.delete(at);ring.push(transform(a));if(key(z)===key(first))break;at=starts.get(key(z))?.find(k=>edges.has(k));}if(ring.length>=3)rings.push(contour(ring,true));}
 const segments=new Map(coasts.map((p,i)=>[i,p])),ends=new Map();for(const [i,p] of segments)for(const v of p){const k=key(v);if(!ends.has(k))ends.set(k,[]);ends.get(k).push(i)}
 const strokes=[];while(segments.size){const [id,first]=segments.entries().next().value;segments.delete(id);const line=[...first];for(const reverse of [false,true]){if(reverse)line.reverse();let again=true;while(again){const ids=ends.get(key(line.at(-1)))||[],next=ids.find(i=>segments.has(i));if(next===undefined){again=false;break}const [a,z]=segments.get(next);segments.delete(next);line.push(key(a)===key(line.at(-1))?z:a)}}const closed=key(line[0])===key(line.at(-1));if(closed)line.pop();strokes.push(contour(line.map(transform),closed));}
 return {water:rings.join(''),coast:strokes.join('')};
}

function ribbon(points,widths){
 const left=[],right=[];
 for(let i=0;i<points.length;i++){const a=points[Math.max(0,i-1)],b=points[Math.min(points.length-1,i+1)],dx=b[0]-a[0],dy=b[1]-a[1],len=Math.hypot(dx,dy)||1,r=widths[i]/2,p=points[i];left.push([p[0]-dy/len*r,p[1]+dx/len*r]);right.push([p[0]+dy/len*r,p[1]-dx/len*r]);}
 return path([...left,...right.reverse()])+'Z';
}
function intersectsPath(points,b){for(let i=1;i<points.length;i++){const a=points[i-1],z=points[i];if(Math.max(a[0],z[0])>=b.x&&Math.min(a[0],z[0])<=b.x+b.width&&Math.max(a[1],z[1])>=b.y&&Math.min(a[1],z[1])<=b.y+b.height)return true;}return false;}

const symbols=`<defs>
<g id="peak" stroke="#665b4f" stroke-width=".65" fill="#fff"><path d="M-8,4L-2,-10L1,-6L4,-12L11,4M-8,4Q2,1 11,4"/><path d="M4,-12L2,-2L7,3M-2,-10L-1,-2M3,-5L5,-3M3,-2L7,1" fill="none" stroke-width=".5"/></g>
<g id="tree" stroke="#626852" stroke-width=".55"><path d="M0,2V-2"/><path d="M-3,-1Q-6,-5 -2,-7Q-2,-11 1,-9Q6,-10 4,-5Q7,0 0,0Z" fill="#eff1e9"/></g>
<g id="hut" stroke="#665b4f" stroke-width=".65" fill="#fff"><path d="M-4,3V-2L0,-5L4,-2V3ZM-5,-2L0,-6L5,-2M0,3V0H2V3"/><path d="M-3,-2H3"/></g>
<g id="castle" stroke="#564b40" stroke-width=".8" fill="#fff"><path d="M-8,4V-9H-6V-6H-4V-9H-2V4M2,4V-9H4V-6H6V-9H8V4ZM-2,-5H2M-2,4V0Q0,-3 2,0V4M-10,4H10M0,-6V-14L6,-12L0,-10"/></g>
</defs>`;

export function renderMedievalMap(data,{title=data.name||'Untitled map'}={}){
 const width=1600,height=1100,b=data.bounds,margin=65,top=120,bottom=90;
 const scale=Math.min((width-2*margin)/b.width,(height-top-bottom)/b.height);
 const box={x:(width-b.width*scale)/2,y:top+(height-top-bottom-b.height*scale)/2,width:b.width*scale,height:b.height*scale};
 const transform=p=>[box.x+(p[0]-b.x)*scale,box.y+(p[1]-b.y)*scale];
 const context={...data,scale,bounds:b},hydro=data.hydrology||{},reaches=new Map((hydro.reaches||[]).map(r=>[r.id,r])),systems=new Map((hydro.rivers||[]).map(r=>[r.id,r]));
 const coast=waterGeometry(data,transform),occupied=[],riverCells=new Set(),labels=[],marks=[],politics=[],settlements=[],roads=[];
 const features=[...new Map((data.features||[]).filter(f=>f.path?.length>1&&/river|stream|channel|braid|delta/.test(f.kind)).map(f=>[f.id,f])).values()].sort((a,z)=>a.id.localeCompare(z.id));
 const rivers=[];
 for(const f of features){if(!intersectsPath(f.path,b))continue;const reach=reaches.get(f.networkId)||reaches.get(f.id)||{},system=systems.get(f.riverId)||{},metrics=f.path.map((p,i)=>riverMetrics(f,i,context,reach,system));if(!metrics.some(m=>m.visible))continue;
  // Visibility is decided for the whole connected river system, not fragments.
  const points=f.path.map(transform),widths=metrics.map(m=>m.width||m.physical||.12);
  rivers.push({f,reach,system,points,widths,metrics});
  for(let i=1;i<points.length;i++){const a=points[i-1],z=points[i],n=Math.max(1,Math.ceil(Math.hypot(z[0]-a[0],z[1]-a[1])/5));for(let j=0;j<=n;j++){const x=a[0]+(z[0]-a[0])*j/n,y=a[1]+(z[1]-a[1])*j/n;if(inside([x,y],box))riverCells.add(`${Math.floor(x/7)},${Math.floor(y/7)}`)}}
 }
 // Shared endpoint widths and round caps preserve exact confluences. No
 // smoothing ever changes a split/rejoin point or creates a decorative branch.
 const joints=new Map();for(const r of rivers)for(const i of [0,r.points.length-1]){const k=key(r.f.path[i]);joints.set(k,Math.max(joints.get(k)||0,r.widths[i]))}
 const water=[],banks=[],channelGroups=new Map();
 for(const r of rivers){const {f,points,widths,metrics}=r;for(const i of [0,points.length-1]){const target=joints.get(key(f.path[i]));let dist=0;for(let j=i;j>=0&&j<points.length;j+=i===0?1:-1){if(j!==i)dist+=Math.hypot(points[j][0]-points[j+(i===0?-1:1)][0],points[j][1]-points[j+(i===0?-1:1)][1]);if(dist>8)break;widths[j]=Math.max(widths[j],widths[j]+(target-widths[j])*Math.max(0,1-dist/8));}}
  const groupID=f.riverId||f.networkId||f.id;if(!channelGroups.has(groupID))channelGroups.set(groupID,[]);const channels=channelGroups.get(groupID);
  const depth=metrics.reduce((n,m)=>n+m.depth,0)/metrics.length,ink=depth>7?'#466574':depth>2?'#597887':'#78949d';
  channels.push(`<path data-river="${esc(f.id)}" d="${ribbon(points,widths)}" fill="${ink}"/>`);
  for(const i of [0,points.length-1])channels.push(`<circle cx="${num(points[i][0])}" cy="${num(points[i][1])}" r="${num(widths[i]/2)}" fill="${ink}"/>`);
  if(Math.max(...widths)>3&&depth>2)banks.push(`<path d="${ribbon(points,widths.map(w=>w+.8))}" fill="none" stroke="#889093" stroke-width=".35"/>`);
  let last=-100;for(let i=1;i<points.length-1;i++){const m=metrics[i],p=points[i];if(!m.canyon||scale<22||!inside(p,box)||Math.hypot(p[0]-last,p[1]-(metrics.lastY||0))<7)continue;const a=points[i-1],z=points[i+1],len=Math.hypot(z[0]-a[0],z[1]-a[1])||1,nx=-(z[1]-a[1])/len,ny=(z[0]-a[0])/len;for(const side of [-1,1]){const from=widths[i]/2+1.5,to=from+clamp(m.incision/35,1.5,7);banks.push(`<path d="M${num(p[0]+nx*from*side)},${num(p[1]+ny*from*side)}l${num(nx*(to-from)*side)},${num(ny*(to-from)*side)}" stroke="#8a7967" stroke-width=".5"/>`)}last=p[0];metrics.lastY=p[1];}
 }
 for(const [id,paths] of channelGroups)water.push(`<g data-river-system="${esc(id)}">${paths.join('')}</g>`);
 const nearRiver=(p,r=0)=>{for(let y=-r;y<=r;y++)for(let x=-r;x<=r;x++)if(riverCells.has(`${Math.floor(p[0]/7)+x},${Math.floor(p[1]/7)+y}`))return true;return false};
 const overlap=(r,z)=>r.x<z.x+z.width&&r.x+r.width>z.x&&r.y<z.y+z.height&&r.y+r.height>z.y;
 const reserve=r=>{if(occupied.some(z=>overlap(r,z)))return false;occupied.push(r);return true};
 const candidateLabels=[];
 const selection=illustrationEntities(data),entities=selection.entities.sort((a,z)=>a.id.localeCompare(z.id)),named=new Set();
 for(const e of entities){const pp=e.points||[];if(!pp.length)continue;const political=stateKinds.has(e.kind),visible=political&&e.geometry==='polygon'?clipPolygon(pp,b):pp;if(!visible.length)continue;const p=featureAnchor(visible,e.geometry),at=transform(p),points=pp.map(transform);if(['polygon','line'].includes(e.geometry)&&(political||/countries|borders/.test(e.layer)))politics.push(`<path data-entity="${esc(e.id)}" d="${path(points)}${e.geometry==='polygon'?'Z':''}" fill="none" stroke="#9a8277" stroke-width="1.05" stroke-dasharray="4 4"/>`);
  if(e.layer==='roads')roads.push(`<path d="${path(points)}${e.geometry==='polygon'?'Z':''}" fill="none" stroke="#b1a292" stroke-width=".8" stroke-dasharray="2 2"/>`);
  if(e.layer==='natural-features'&&e.geometry==='polygon'&&/lake|sea|ocean/.test(e.kind))water.push(`<path d="${path(points)}Z" fill="#eaf0f1" stroke="#708890" stroke-width=".7"/>`);
  if(e.layer==='settlements'){
   if(!inside(p,b)&&!intersectsPath(pp,b))continue;
   const capital=e.kind==='capital',city=capital||e.kind==='city'||e.kind==='town';
   if(e.geometry==='polygon'&&(city||b.width/(data.terrainWidth-1)<.1))settlements.push(`<path d="${path(points)}Z" fill="none" stroke="#8d7965" stroke-width="${city?1.2:.7}"/>`);
   const count=capital?7:city?5:3;
   for(let i=0;i<count;i++){const offset=i===0?[0,0]:[(i%3-1)*7,(Math.floor(i/3)-.5)*7],pos=[at[0]+offset[0],at[1]+offset[1]],symbol=i===0&&capital?'castle':'hut';if(!inside(pos,box))continue;settlements.push(`<use href="#${symbol}" transform="translate(${num(pos[0])} ${num(pos[1])})"/>`)}
   occupied.push({x:at[0]-18,y:at[1]-18,width:36,height:32});
  }
  if(e.layer==='buildings'&&inside(p,b)){settlements.push(`<use href="#${/castle|fortification/.test(e.kind)?'castle':'hut'}" transform="translate(${num(at[0])} ${num(at[1])})"/>`)}
  if(e.name&&!/^example-/i.test(e.name.trim())&&e.kind!=='house'&&inside(p,b)&&!named.has(e.logicalId||e.id)){
   named.add(e.logicalId||e.id);
   const source=systems.get(e.sourceId),river=e.kind==='river',labelAt=at;
   const size=political?14:e.kind==='capital'?12:e.layer==='settlements'?10:river?8:9;
   const priority=political?110:e.kind==='capital'?100:e.layer==='settlements'?80:river?40+Math.log1p(source?.lengthKm||0):30;
   candidateLabels.push({id:e.id,name:e.labelName||e.name,at:labelAt,size,priority,river,political,required:political||(selection.detailed&&settlementKinds.has(e.kind)),settlement:e.layer==='settlements'||e.layer==='buildings'});
  }
 }
 for(const l of candidateLabels.sort((a,z)=>z.priority-a.priority||a.name.localeCompare(z.name))){
  const lines=wrapPlaceName(l.name);l.size=Math.min(l.size,(box.width-4)/(Math.max(...lines.map(s=>s.length))*.54));const lw=Math.max(...lines.map(s=>s.length))*l.size*.54,lh=lines.length*l.size*1.18;
  // Prefer a compact, centred label directly above its own symbol. Never
  // push a name far away just to fit it into an unrelated empty area.
  const gap=l.settlement?21:5,offsets=l.political?[[0,-lh/2],[0,-lh-8],[0,8],[-16,-lh/2],[16,-lh/2]]:[[0,-gap-lh],[-8,-gap-lh],[8,-gap-lh],[0,-gap-lh-12],[0,18]];
  let best=null;for(const [dx,dy] of offsets){const r={x:l.at[0]+dx-lw/2,y:l.at[1]+dy,width:lw,height:lh};if(!inside([r.x,r.y],box)||!inside([r.x+r.width,r.y+r.height],box)||occupied.some(z=>overlap(r,z)))continue;best={r};break;}
  // Required state and local settlement names must not disappear because a
  // symbol occupied the preferred anchor. Keep them anchored with a paper halo.
  if(!best&&l.required)best={r:{x:clamp(l.at[0]-lw/2,box.x+1,box.x+box.width-lw-1),y:clamp(l.at[1]-(l.political?lh/2:gap+lh),box.y+1,box.y+box.height-lh-1),width:lw,height:lh}};
  if(!best)continue;const {r}=best;occupied.push(r);const x=r.x+r.width/2;
  labels.push(`<text data-label-for="${esc(l.id)}" aria-label="${esc(l.name)}" x="${num(x)}" y="${num(r.y+l.size)}" text-anchor="middle" font-size="${l.size}"${l.river?' font-style="italic" fill="#526e7b"':''}>${lines.map((line,i)=>`<tspan x="${num(x)}" dy="${i?num(l.size*1.18):0}">${esc(line)}</tspan>`).join('')}</text>`);

 }
 // Terrain symbols use the sampled geography, spaced in page coordinates.
 // The same deterministic spatial hash varies glyph size, not the landforms.
 const sampleAt=(x,y)=>{const ix=clamp(Math.round((x-b.x)/b.width*(data.columns-1)),0,data.columns-1),iy=clamp(Math.round((y-b.y)/b.height*(data.rows-1)),0,data.rows-1);return data.samples[iy*data.columns+ix]};
 const forests=entities.filter(e=>e.kind==='forest'&&e.geometry==='polygon'),mountains=entities.filter(e=>e.kind==='mountain'&&e.geometry==='polygon');
 for(let py=box.y+10,row=0;py<box.y+box.height-8;py+=12,row++)for(let px=box.x+8,col=0;px<box.x+box.width-8;px+=13,col++){
  const j=hash(col,row),p=[px+(j-.5)*7,py+(hash(row,col)-.5)*6],wp=[b.x+(p[0]-box.x)/scale,b.y+(p[1]-box.y)/scale],s=sampleAt(...wp);if(s[0]>.45||nearRiver(p,1)||occupied.some(r=>overlap({x:p[0]-8,y:p[1]-13,width:20,height:19},r)))continue;
  const mountain=s[3]>.2||mountains.some(e=>polygonContains(wp,e.points)),forest=s[2]>.35||forests.some(e=>polygonContains(wp,e.points));
  if(mountain&&j<.62){const size=.65+s[3]*.65;marks.push(`<use href="#peak" transform="translate(${num(p[0])} ${num(p[1])}) scale(${num(size)})"/>`)}
  else if(forest&&j<.65){marks.push(`<use href="#tree" transform="translate(${num(p[0])} ${num(p[1])}) rotate(${num((j-.5)*12)}) scale(${num(.75+j*.45)})"/>`)}
 }

 const g=data.geography,centreLat=g?(g.latitude_north+(g.latitude_south-g.latitude_north)*(b.y+b.height/2)/data.terrainHeight)*Math.PI/180:0;
 const kmPerCell=g?g.planetary_radius*(g.longitude_east-g.longitude_west)*Math.PI/180/data.terrainWidth*Math.cos(centreLat):0;
 const rawDistance=kmPerCell*150/scale,power=10**Math.floor(Math.log10(rawDistance||1)),distance=Math.max(power,Math.floor(rawDistance/power)*power),bar=kmPerCell?distance/kmPerCell*scale:0;
 const scaleBar=bar?`<path d="M90,${height-55}v5h${num(bar)}v-5" fill="none" stroke="#665b4f"/><text x="90" y="${height-65}" font-size="11">${num(distance)} km at centre latitude</text>`:'';
 const svg=`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"><title>${esc(title)}</title><desc>Medieval map illustration. ${esc(data.name)}; source revision ${data.revision}. Channel widths use hydrology; symbols represent the underlying geography.</desc><rect width="${width}" height="${height}" fill="#FFFFFF"/>${symbols}<defs><clipPath id="map-extent"><rect x="${num(box.x)}" y="${num(box.y)}" width="${num(box.width)}" height="${num(box.height)}"/></clipPath></defs><g font-family="Palatino Linotype, Book Antiqua, Georgia, serif" fill="#51483e"><rect x="24" y="24" width="${width-48}" height="${height-48}" fill="none" stroke="#655747" stroke-width="1.2"/><rect x="30" y="30" width="${width-60}" height="${height-60}" fill="none" stroke="#a39480" stroke-width=".5"/><text x="800" y="77" text-anchor="middle" font-size="${num(Math.min(32,1100/Math.max(1,title.length*.63)))}" letter-spacing="2">${esc(title)}</text><path d="M650,91H950" stroke="#a39480" stroke-width=".65"/><g clip-path="url(#map-extent)"><path d="${coast.water}" fill="#edf2f3" fill-rule="evenodd"/><path d="${coast.coast}" fill="none" stroke="#596e72" stroke-width=".8" stroke-linejoin="round"/>${marks.join('')}${politics.join('')}${roads.join('')}${banks.join('')}${water.join('')}${settlements.join('')}<g stroke="#fff" stroke-width="2.5" paint-order="stroke" stroke-linejoin="round">${labels.join('')}</g></g><g transform="translate(1503 83)" stroke="#655747" stroke-width=".7"><path d="M0,-24L5,-4L24,0L5,4L0,24L-5,4L-24,0L-5,-4Z" fill="#fff"/><path d="M0,-24V24M-24,0H24M0,-24L5,-4L0,0Z"/><text x="0" y="-31" text-anchor="middle" stroke="none" font-size="12">N</text></g>${scaleBar}<text x="${width-90}" y="${height-55}" text-anchor="end" font-size="10" fill="#8b8175">${g?'':'Geographic scale unspecified | '}KRIEMHILD | ${MEDIEVAL_VERSION}</text></g></svg>`;
 return {svg,width,height,bounds:{...b},rendererVersion:MEDIEVAL_VERSION,style:'medieval-white',name:title,sourceRevision:data.revision,statistics:{rivers:rivers.length,entities:entities.length,symbols:marks.length,labels:labels.length}};
}

export function downloadArtwork(blob,name){const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=name;document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);}
export async function artworkPNG(artwork,longEdge=4800){
 const ratio=clamp(Number(longEdge)||4800,1600,8192)/Math.max(artwork.width,artwork.height),canvas=document.createElement('canvas');canvas.width=Math.round(artwork.width*ratio);canvas.height=Math.round(artwork.height*ratio);
 const url=URL.createObjectURL(new Blob([artwork.svg],{type:'image/svg+xml;charset=utf-8'}));try{const img=new Image();await new Promise((resolve,reject)=>{img.onload=resolve;img.onerror=()=>reject(Error('Could not rasterize the illustration'));img.src=url});const ctx=canvas.getContext('2d');if(!ctx)throw Error('PNG canvas unavailable');ctx.drawImage(img,0,0,canvas.width,canvas.height);return await new Promise((resolve,reject)=>canvas.toBlob(b=>b?resolve(b):reject(Error('PNG export failed')),'image/png'));}finally{URL.revokeObjectURL(url);canvas.width=canvas.height=1;}
}
