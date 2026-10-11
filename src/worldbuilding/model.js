import registry from '../../internal/history/registry.json';
export {registry};
export const identity=e=>e.continuityId||e.id;
export const activeEntities=(doc,age)=>doc.entities.filter(e=>e.ageId===age&&!e.deleted);
export const entityIn=(doc,age,id)=>doc.entities.find(e=>e.ageId===age&&e.id===id);
export const label=s=>(s||'').replaceAll('-',' ');
export const ageYears=(doc,age)=>(doc.years||[]).filter(y=>y.ageId===age).slice().sort((a,b)=>a.number-b.number);
function canonical(v){if(Array.isArray(v))return v.map(canonical);if(v&&typeof v==='object')return Object.fromEntries(Object.keys(v).sort().map(k=>[k,canonical(v[k])]));return v;}
function signature(doc,e){
 const relations=doc.relations.filter(r=>r.ageId===e.ageId&&(r.source===e.id||r.destination===e.id)).map(({ageId,id,...r})=>r).sort((a,b)=>JSON.stringify(a).localeCompare(JSON.stringify(b)));
 const shapes=doc.representations.filter(r=>r.ageId===e.ageId&&r.entityId===e.id).map(r=>({mapId:r.mapId,shape:r.shape})).sort((a,b)=>a.mapId.localeCompare(b.mapId));
 return JSON.stringify(canonical({name:e.name,kind:e.kind,names:e.names||[],properties:e.properties,relations,shapes}));
}
export function compareAge(doc,age){
 const source=doc.ages.find(a=>a.id===age)?.inheritedFrom,previous=new Map(activeEntities(doc,source).map(e=>[identity(e),e]));
 const rows=activeEntities(doc,age).map(entity=>{const before=previous.get(identity(entity));previous.delete(identity(entity));return {entity,before,status:!before?'added':signature(doc,entity)===signature(doc,before)?'unchanged':'modified'}});
 return [...rows,...[...previous.values()].map(entity=>({entity,status:'removed'}))];
}
export function contextualRelations(doc,age){
 const relations=doc.relations.filter(r=>r.ageId===age),attendance=relations.filter(r=>r.kind==='attended'),result=[];
 for(let i=0;i<attendance.length;i++)for(let j=i+1;j<attendance.length;j++){
  const a=attendance[i],b=attendance[j];if(a.source===b.source||a.destination!==b.destination)continue;
  // Unknown dates are not evidence of overlapping attendance.
  if(!a.validFrom||!a.validTo||!b.validFrom||!b.validTo)continue;
  const values=[a.validFrom,a.validTo,b.validFrom,b.validTo].map(Number);if(!values.every(Number.isFinite))continue;
  if(Math.max(values[0],values[2])>Math.min(values[1],values[3]))continue;
  result.push({id:`schoolmates/${a.id}/${b.id}`,ageId:age,source:a.source,destination:b.source,kind:'schoolmate',automatic:true,contextId:a.destination,description:'Derived from overlapping recorded attendance.',validFrom:String(Math.max(values[0],values[2])),validTo:String(Math.min(values[1],values[3]))});
 }
 return [...relations,...result];
}
export function graphData(doc,age,{center='',family='',type='',genealogy=false,members=[]}={}){
 const all=activeEntities(doc,age),characters=all.filter(e=>e.kind==='character'),ids=new Set(characters.map(e=>e.id));
 let edges=contextualRelations(doc,age).filter(r=>ids.has(r.source)&&ids.has(r.destination)&&(!type||r.kind===type)&&(genealogy?isFamilyRelation(r):isInterpersonalRelation(r)));
 let included=new Set(ids);
 if(family){const view=(doc.world?.networks||[]).find(n=>n.ageId===age&&n.id===family);members=view?.memberIds||doc.relations.filter(r=>r.ageId===age&&r.kind==='member-of-family'&&r.destination===family).map(r=>r.source);}
 if(members.length)included=new Set(members);
 if(center){included=new Set([center]);let changed=true;while(changed){changed=false;for(const r of edges)if(included.has(r.source)!==included.has(r.destination)){included.add(r.source);included.add(r.destination);changed=true}}}
 const nodes=characters.filter(e=>included.has(e.id));edges=edges.filter(r=>included.has(r.source)&&included.has(r.destination));
 const ranks=new Map(nodes.map(e=>[e.id,0]));
 if(genealogy){for(let pass=0;pass<Math.min(nodes.length,8);pass++){let changed=false;for(const r of edges){if(!['biological-parent','adoptive-parent','parent-of','child-of'].includes(r.kind))continue;const source=r.kind==='child-of'?r.destination:r.source,dest=r.kind==='child-of'?r.source:r.destination;const next=Math.min(8,(ranks.get(source)||0)+1);if(next>(ranks.get(dest)||0)){ranks.set(dest,next);changed=true}}if(!changed)break}}
 const rows=new Map(),positions=new Map();nodes.forEach((e,i)=>{const row=genealogy?ranks.get(e.id):Math.floor(i/5),col=rows.get(row)||0;rows.set(row,col+1);positions.set(e.id,{x:120+col*220,y:100+row*170})});
 return {nodes,edges,positions,width:Math.max(800,...[...rows.values()].map(n=>n*220+80)),height:Math.max(450,(Math.max(0,...rows.keys())+1)*170+70)};
}
export function relationshipClusters(doc,age,genealogy=false){
 const {nodes,edges}=graphData(doc,age,{genealogy}),adjacency=new Map(nodes.map(e=>[e.id,new Set()])),byID=new Map(nodes.map(e=>[e.id,e]));
 for(const r of edges){adjacency.get(r.source).add(r.destination);adjacency.get(r.destination).add(r.source)}
 const visited=new Set(),clusters=[];
 for(const e of nodes){if(visited.has(e.id))continue;const members=[],queue=[e.id];visited.add(e.id);while(queue.length){const id=queue.pop();members.push(id);for(const other of adjacency.get(id)){if(!visited.has(other)){visited.add(other);queue.push(other)}}}if(members.length>1)clusters.push({centerId:e.id,members,name:members.slice(0,3).map(id=>byID.get(id).name).join(', ')})}
 return clusters.sort((a,b)=>b.members.length-a.members.length||a.name.localeCompare(b.name));
}

export const isPlaceProfile=e=>['settlements','politics'].includes(registry.types[e.kind]?.category);
export function entityDetailMap(doc,entity){return doc.maps.find(m=>m.ageId===entity.ageId&&m.entityId===entity.id&&m.parentId)||null;}
// Present each neighbour once, with directional meanings from this profile's
// perspective. Keep authored political authority separate from spatial facts.
export function profileConnections(doc,entity){
 const groups=new Map(), inverse={'contained-by':'contains','located-within':'contains','part-of':'has part','contains':'located within','capital-of':'has capital','governs':'governed by','controls':'controlled by','claims':'claimed by','born-in':'birthplace of','resides-in':'home to','member-of':'has member'};
 const rows=doc.relations.filter(r=>r.ageId===entity.ageId&&(r.source===entity.id||r.destination===entity.id));
 for(const r of rows){const forward=r.source===entity.id,other=forward?r.destination:r.source;let g=groups.get(other);if(!g){g={other,relations:[],meanings:[]};groups.set(other,g)}
  const spatial=r.automatic&&['contained-by','part-of','located-within','contains'].includes(r.kind);
  const meaning=spatial?(r.kind==='contains'?!forward:forward)?'located within':'contains':forward?label(r.kind):(inverse[r.kind]||`receives ${label(r.kind)} from`);
  g.relations.push(r);if(!g.meanings.some(m=>m.text===meaning&&m.automatic===r.automatic))g.meanings.push({text:meaning,automatic:r.automatic});
 }return [...groups.values()];
}

export const isFamilyRelation=r=>registry.relations[r.kind]?.family==='genealogical';
export const isInterpersonalRelation=r=>registry.relations[r.kind]?.family==='social'||r.kind==='schoolmate';
export const parentKinds=['biological-parent','adoptive-parent','parent-of','child-of'];
export function parentPair(r){return !parentKinds.includes(r.kind)?null:r.kind==='child-of'?[r.destination,r.source]:[r.source,r.destination];}
export const characterRelationKinds=genealogy=>Object.entries(registry.relations).filter(([k,r])=>(genealogy?r.family==='genealogical':r.family==='social')&&k!=='member-of-family'&&(!r.sources?.length||r.sources.includes('character'))&&(!r.destinations?.length||r.destinations.includes('character'))).map(([k])=>k);
export function familyConflict(doc,age,relation,ignore=[]){
 const pair=parentPair(relation),edges=doc.relations.filter(r=>r.ageId===age&&r.id!==relation.id&&!ignore.includes(r.id));
 if(relation.source===relation.destination)return 'Choose two different characters.';
 const same=r=>[r.source,r.destination].sort().join('/')===[relation.source,relation.destination].sort().join('/');
 for(const r of edges){const old=parentPair(r);if(!same(r))continue;
 if((r.kind===relation.kind&&(r.source===relation.source||registry.relations[r.kind]?.reciprocal))||(pair&&old&&pair[0]===old[0]&&pair[1]===old[1]))return 'This family connection already exists. Edit the existing connection.';
 if(pair&&['sibling','spouse','partner'].includes(r.kind)||old&&['sibling','spouse','partner'].includes(relation.kind))return 'A parent and child cannot also be siblings or partners.';
 }
 if(pair){const parents=new Set(edges.map(parentPair).filter(p=>p&&p[1]===pair[1]).map(p=>p[0]));if(parents.size>=2&&!parents.has(pair[0]))return 'This child already has two parents. Edit their parentage.';
 const seen=new Set(),queue=[pair[1]];while(queue.length){const id=queue.pop();if(id===pair[0])return 'This connection would make a character their own ancestor.';if(seen.has(id))continue;seen.add(id);for(const r of edges){const p=parentPair(r);if(p?.[0]===id)queue.push(p[1]);}}
 }return '';
}
export const connectionPorts=['nw','n','ne','e','se','s','sw','w'];
export function portPoint(p,port){if(p.width){const x=p.width/2,y=p.height/2,o={nw:[-x,-y],n:[0,-y],ne:[x,-y],e:[x,0],se:[x,y],s:[0,y],sw:[-x,y],w:[-x,0]}[port]||[0,y];return {x:p.x+o[0],y:p.y+o[1]}}const offset={nw:[-80,-25],n:[0,-25],ne:[80,-25],e:[80,4],se:[80,33],s:[0,33],sw:[-80,33],w:[-80,4]}[port]||[0,33];return {x:p.x+offset[0],y:p.y+offset[1]};}
export function edgeGeometry(a,b,r,index=0){
 const dx=b.x-a.x,dy=b.y-a.y,horizontal=Math.abs(dx)>Math.abs(dy)*1.3;
 const from=r.sourcePort||(horizontal?(dx>=0?'e':'w'):(dy>=0?'s':'n')),to=r.destinationPort||(horizontal?(dx>=0?'w':'e'):(dy>=0?'n':'s'));
 const start=portPoint(a,from),end=portPoint(b,to),normal=port=>{const v={nw:[-1,-1],n:[0,-1],ne:[1,-1],e:[1,0],se:[1,1],s:[0,1],sw:[-1,1],w:[-1,0]}[port];return v};
 const n=normal(from),m=normal(to),bend=45+Math.floor(index/2)*25,spread=(index%2?-1:1)*(16+Math.floor(index/2)*24);
 const c1={x:start.x+n[0]*bend+(horizontal?0:spread),y:start.y+n[1]*bend+(horizontal?spread:0)},c2={x:end.x+m[0]*bend+(horizontal?0:spread),y:end.y+m[1]*bend+(horizontal?spread:0)};
 return {start,end,path:`M${start.x} ${start.y} C${c1.x} ${c1.y} ${c2.x} ${c2.y} ${end.x} ${end.y}`,label:{x:(start.x+3*c1.x+3*c2.x+end.x)/8,y:(start.y+3*c1.y+3*c2.y+end.y)/8}};
}

// Two parent records render as one descent line from a shared parental union.
// A co-parent connector is visual structure, never an invented spouse relation.
export function familyJunctions(data){
 const children=new Map(),hidden=new Set(),unions=new Map(),links=[];
 for(const edge of data.edges){const pair=parentPair(edge);if(!pair)continue;if(!children.has(pair[1]))children.set(pair[1],new Map());children.get(pair[1]).set(pair[0],edge)}
 for(const [child,parents] of children){
  if(parents.size!==2)continue;
  const ids=[...parents.keys()].sort(),key=ids.join('/');
  let union=unions.get(key);
  if(!union){
   const partner=data.edges.find(r=>['spouse','partner'].includes(r.kind)&&ids.includes(r.source)&&ids.includes(r.destination));
   const relation=partner||{source:ids[0],destination:ids[1]};
   // Match the actual displayed partner edge's parallel-edge index.
   const index=partner?data.edges.slice(0,data.edges.indexOf(partner)).filter(r=>[r.source,r.destination].sort().join('/')===key).length:0;
   const geometry=edgeGeometry(data.positions.get(relation.source),data.positions.get(relation.destination),relation,index);
   union={key,parents:ids,partner,geometry};unions.set(key,union);
  }
  const original=parents.get(ids[0]),childPort=(original.source===child?original.sourcePort:original.destinationPort)||'n',end=portPoint(data.positions.get(child),childPort),start=union.geometry.label;
  const midY=(start.y+end.y)/2,curve=`M${start.x} ${start.y} C${start.x+10} ${midY} ${end.x+10} ${midY} ${end.x} ${end.y}`;
  const records=[...parents.values()];records.forEach(r=>hidden.add(r.id));
  links.push({child,parents:ids,records,start,end,path:curve,relation:{...original,kind:'child-of',source:child,destination:ids[0],sourcePort:childPort,destinationPort:original.source===child?original.destinationPort:original.sourcePort}});
 }
 return {hidden,unions:[...unions.values()],links};
}
