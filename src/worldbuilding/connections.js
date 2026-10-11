import {registry,activeEntities,contextualRelations,isFamilyRelation,isInterpersonalRelation,graphData} from './model.js';

export const matches=(e,allowed)=>!!e&&(!allowed?.length||allowed.includes(e.kind)||allowed.includes(registry.types[e.kind]?.category));
const institutionalPersonal=new Set(['founded-by','sponsors','funds','serves','represents','advises','protects','opposes','influences']);
const personalPlaces=new Set(['contractor-for','client-of','informs','founder-of','born-in','resides-in','citizen-of','member-of','attended','employed-by','served-in','practices','belongs-to','follows','participated-in','migrated-from','migrated-to','appointed-to','affirms','rejects','teaches','interprets','disputes','transmits']);
export function connectionKinds(a,b,mode='all') {
 if(!a||!b||a.id===b.id)return [];
 const people=a.kind==='character'&&b.kind==='character';
 return Object.entries(registry.relations).filter(([k,r])=>{
  if(!matches(a,r.sources)||!matches(b,r.destinations))return false;
  if((mode==='all'||mode==='places')&&a.kind==='character'&&registry.types[b.kind]?.category==='systems')return ['system-knowledge','system-access'].includes(k);
  if((mode==='all'||mode==='places')&&b.kind==='character'&&registry.types[a.kind]?.category==='systems')return ['system-developed-by','system-uses'].includes(k);
  if(people)return (mode==='all'||mode==='family')&&r.family==='genealogical'&&k!=='member-of-family'||(mode==='all'||mode==='social')&&r.family==='social';
  if((mode==='all'||mode==='places')&&registry.types[a.kind]?.category==='organizations'&&b.kind==='character')return institutionalPersonal.has(k);
  return (mode==='all'||mode==='places')&&a.kind==='character'&&b.kind!=='character'&&personalPlaces.has(k)&&(['geography','settlements','politics','organizations','culture','history'].includes(registry.types[b.kind]?.category)||b.kind==='organization-position'&&k==='appointed-to');
 }).map(([k])=>k);
}
export function peopleGraph(doc,age,{mode='all',center='',search='',type='',extras=[],hidden=[]}={}){
 const all=activeEntities(doc,age),byID=new Map(all.map(e=>[e.id,e]));
 let edges=contextualRelations(doc,age).filter(r=>{
  const a=byID.get(r.source),b=byID.get(r.destination);
  return a&&b&&(!type||r.kind===type)&&(connectionKinds(a,b,mode).includes(r.kind)||r.kind==='schoolmate'&&['all','social'].includes(mode));
 });
 let included=new Set(all.filter(e=>e.kind==='character').map(e=>e.id));
 edges.forEach(r=>{included.add(r.source);included.add(r.destination)});extras.forEach(id=>included.add(id));
 if(center){included=new Set([center]);let changed=true;while(changed){changed=false;for(const r of edges)if(included.has(r.source)!==included.has(r.destination)){included.add(r.source);included.add(r.destination);changed=true}}extras.forEach(id=>included.add(id))}
 let nodes=all.filter(e=>included.has(e.id)&&!hidden.includes(e.id));
 if(search){const found=new Set(nodes.filter(e=>e.name.toLowerCase().includes(search.toLowerCase())).map(e=>e.id));for(const r of edges)if(found.has(r.source)||found.has(r.destination)){included.add(r.source);included.add(r.destination)}nodes=nodes.filter(e=>found.has(e.id)||edges.some(r=>r.source===e.id&&found.has(r.destination)||r.destination===e.id&&found.has(r.source)))}
 included=new Set(nodes.map(e=>e.id));edges=edges.filter(r=>included.has(r.source)&&included.has(r.destination));
 const family=graphData(doc,age,{genealogy:true}),positions=new Map();
 nodes.forEach((e,i)=>{const saved=e.properties?.graphPosition,p=mode==='family'?family.positions.get(e.id):null;positions.set(e.id,{x:Math.max(110,saved?.x??p?.x??130+(i%5)*260),y:Math.max(65,saved?.y??p?.y??100+Math.floor(i/5)*180),width:e.kind==='character'?160:76,height:e.kind==='character'?90:76})});
 return {nodes,edges,positions,width:Math.max(1200,...[...positions.values()].map(p=>p.x+180)),height:Math.max(620,...[...positions.values()].map(p=>p.y+140))};
}

// Reconnecting preserves relationship identity, descriptions and historical metadata.
export function reconnectRelations(records,from,target){
 return records.map(r=>{const side=r.source===from?'source':r.destination===from?'destination':null;if(!side)throw Error('Connection endpoint no longer exists.');return {...r,[side]:target.id,[side+'Port']:target.port}});
}
