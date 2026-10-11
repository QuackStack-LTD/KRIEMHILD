import {renderClimateDescription} from './climate-ui.js';
import {formatMapLocation,terrainInspection} from './terrain-inspection.js';
// Resource descriptions always come from stored occurrences queried by the
// server. The browser never predicts deposits from a biome or rock label.
export function naturalGoodLabel(good,units='cells'){
 const where=good.location==='here'?good.rating:`Nearby (${good.distance.toFixed(1)} ${units})`;
 return `${good.name} — ${where}`;
}

// Merge repeated descriptions, keeping the most relevant occurrence. Local
// occurrences outrank nearby ones; distinct formation descriptions stay separate.
export function deduplicateDescriptions(items,key){
 const groups=new Map();
 for(const item of items||[]){const id=key(item),previous=groups.get(id);
  if(!previous||((item.location==='here')-(previous.location==='here')||((previous.distance||0)-(item.distance||0))||((item.abundance||0)-(previous.abundance||0)))>0)groups.set(id,item);
 }
 return [...groups.values()];
}
const normalized=value=>String(value||'').trim().replace(/\s+/g,' ').toLowerCase();
export const uniqueGoods=items=>deduplicateDescriptions(items,item=>JSON.stringify([item.type||item.name,normalized(item.description)]));
const geologyDescription=f=>`${f.kind.replaceAll('-',' ')}${f.rock?` \u00b7 ${f.rock}`:''} \u00b7 ${Math.round(f.ageMa)} Ma: ${f.process}`;

export function createNaturalHover(host,onPinChange=()=>{},describePoint=terrainInspection){
 if(!host)return {update(){},pin(){},pinned:()=>null,dispose(){}};
 const content=document.createElement('div'),controls=document.createElement('label'),radius=document.createElement('input');
 host.classList.add('natural-hover');content.className='natural-hover-content';
 const radiusLabel=document.createElement('span');controls.append(radiusLabel);radius.type='number';radius.min='0';
 let metric=true;
 function setRadiusMode(km){metric=km;radiusLabel.textContent=km?'Nearby search distance (km) ':'Nearby search distance (unscaled cells) ';radius.max=km?'20000':'20';radius.step=km?'1':'.5';radius.value=km?'500':'2';radius.setAttribute('aria-label',km?'Nearby resource search distance in kilometers':'Nearby resource search distance in unscaled map cells');try{const saved=Number(localStorage.getItem(km?'kriemhild-natural-radius-km':'kriemhild-natural-radius')??radius.value);if(Number.isFinite(saved))radius.value=String(Math.max(0,Math.min(Number(radius.max),saved)));}catch{}}
 setRadiusMode(true);
 controls.append(radius);host.replaceChildren(content,controls);content.textContent='Hover over terrain to inspect geology and natural goods.';
 const pinStatus=document.createElement('small'),clearPin=document.createElement('button');clearPin.type='button';clearPin.textContent='Clear pin';clearPin.hidden=true;host.append(pinStatus,clearPin);
 let pinned=null,lastQuery=0;
 function pinChanged(notify=true){pinStatus.textContent=pinned?`Pinned spot: ${formatMapLocation(solver,pinned)}. Click another spot to move the pin.`:'Click the map to pin a description (use Select or Pan in the builder).';clearPin.hidden=!pinned;if(notify)onPinChange(pinned);}
 pinChanged(false);
 let timer=0,abort=null,sequence=0,solver=null,point=null,key='',disposed=false;const cache=new Map();
 const element=(tag,text)=>{const el=document.createElement(tag);el.textContent=text;return el;};
 function render(data){
  const inspected=describePoint(solver,{x:data.x,y:data.y},point?.elevation!==undefined?point:null);
  content.replaceChildren(element('strong',`Terrain: ${inspected?.terrain||data.terrain}`),element('small',`Last inspected: ${formatMapLocation(solver,data)}`));
  content.append(renderClimateDescription(data.climate,element));
  const bio=data.biosphere;
  if(bio?.available){
   const section=element('section','');section.className='spot-biosphere';section.append(element('strong','Habitats & living species'));
   if(bio.stale)section.append(element('p','These are saved distributions; environmental changes require a habitat review.'));
   for(const h of bio.habitats||[])section.append(element('b',h.name),element('p',h.description));
   const groups=new Map();for(const life of bio.lifeforms||[]){if(!groups.has(life.group))groups.set(life.group,[]);groups.get(life.group).push(life)}
   for(const [group,lives] of groups){const details=element('details','');details.open=['plant','animal'].includes(group);details.append(element('summary',`${group[0].toUpperCase()+group.slice(1)} (${lives.length})`));const list=element('ul','');for(const life of lives){const row=element('li',''),link=element('a',life.name);link.href=`/worlds/${bio.worldId}/profile?entity=${encodeURIComponent(life.id)}&age=${bio.ageId}`;link.target='_blank';link.rel='noopener';row.append(link,element('small',`${Math.round(life.abundance*100)}% relative abundance / ${life.status}${life.seasons?.length?' / '+life.seasons.join(', '):''}${life.roles?.length?' / '+life.roles.join(', '):''}`));list.append(row)}details.append(list);section.append(details)}
   if(!bio.lifeforms?.length)section.append(element('p','No saved living populations at this spot.'));content.append(section);
  }
  if(!data.available){content.append(element('p','This saved world has no geological-history or resource layer. Its geography has been preserved.'));return;}
  const geology=document.createElement('details');geology.append(element('summary','Geological context'));
  for(const f of deduplicateDescriptions(data.geology,f=>normalized(geologyDescription(f))))geology.append(element('p',geologyDescription(f)));
  content.append(geology,element('strong','Natural goods'));
  if(!data.goods?.length)content.append(element('p','No generated resource occurrences here or within the selected distance.'));
  const list=document.createElement('ul');
  for(const good of uniqueGoods(data.goods)){const row=document.createElement('li');row.append(element('b',naturalGoodLabel(good,data.distanceUnits==='km'?'km':'cells')),element('span',good.description));if(good.depthMetres>0)row.append(element('small',`Approximate depth: ${Math.round(good.depthMetres)} m`));list.append(row);}
  content.append(list);
  if(data.truncated)content.append(element('small',`Results cover ${data.goods.length} of ${data.total} occurrences; repeated descriptions are combined. Reduce the search distance for a narrower result.`));
 }
 async function request(requestKey,token){
  const controller=new AbortController();abort=controller;
  try{
   const params=new URLSearchParams({x:point.x,y:point.y,[metric?'radiusKm':'radius']:radius.value});
   const response=await fetch(`/api/sessions/${solver.id}/world/natural?${params}`,{signal:controller.signal});
   const data=await response.json();if(!response.ok)throw Error(data.error||'Resource query failed');
   if(disposed||token!==sequence)return;
   cache.set(requestKey,{data,time:Date.now()});if(cache.size>64)cache.delete(cache.keys().next().value);render(data);
  }catch(error){if(!disposed&&token===sequence&&error.name!=='AbortError'){content.textContent=`Natural goods unavailable: ${error.message}`;key='';}}
 }
 function schedule(){
  if(!solver?.id||!point)return;
  const nextKey=`${solver.id}/${solver.project?.revision??0}/${point.x}/${point.y}/${radius.value}`;if(nextKey===key&&Date.now()-lastQuery<3000)return;key=nextKey;lastQuery=Date.now();
  clearTimeout(timer);abort?.abort();const token=++sequence;
  if(cache.has(key)&&Date.now()-cache.get(key).time<3000){render(cache.get(key).data);return;}
  content.textContent='Inspecting terrain, geology and natural goods…';timer=setTimeout(()=>request(nextKey,token),140);
 }
 const change=()=>{const value=Number(radius.value);radius.value=String(Number.isFinite(value)?Math.max(0,Math.min(Number(radius.max),value)):(metric?500:2));try{localStorage.setItem(metric?'kriemhild-natural-radius-km':'kriemhild-natural-radius',radius.value);}catch{}schedule();};
 radius.addEventListener('change',change);
 const unpin=()=>{pinned=null;pinChanged();};clearPin.addEventListener('click',unpin);
 function setWorld(next){if(next?.id!==solver?.id){clearTimeout(timer);abort?.abort();sequence++;cache.clear();key='';point=null;pinned=null;pinChanged();setRadiusMode(!!next?.environment?.geography);content.textContent='Hover over terrain to inspect geology and natural goods.';}solver=next;}
 const valid=p=>p&&Number.isFinite(p.x)&&Number.isFinite(p.y);
 return {
  update(next,position){setWorld(next);if(!pinned&&valid(position))point={...position};schedule();},
  pin(next,position){setWorld(next);if(!valid(position))return;pinned=point={...position};pinChanged();schedule();},
  pinned:()=>pinned,
  dispose(){disposed=true;sequence++;clearTimeout(timer);abort?.abort();radius.removeEventListener('change',change);clearPin.removeEventListener('click',unpin);host.replaceChildren();}
 };
}
