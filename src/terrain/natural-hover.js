import {renderClimateDescription} from './climate-ui.js';
// Resource descriptions always come from stored occurrences queried by the
// server. The browser never predicts deposits from a biome or rock label.
export function naturalGoodLabel(good){
 const where=good.location==='here'?good.rating:`Nearby (${good.distance.toFixed(1)} cells)`;
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

export function createNaturalHover(host,onPinChange=()=>{}){
 if(!host)return {update(){},pin(){},pinned:()=>null,dispose(){}};
 const content=document.createElement('div'),controls=document.createElement('label'),radius=document.createElement('input');
 host.classList.add('natural-hover');content.className='natural-hover-content';
 controls.textContent='Nearby search distance (map cells) ';radius.type='number';radius.min='0';radius.max='20';radius.step='.5';radius.value='2';radius.setAttribute('aria-label','Nearby resource search distance in map cells');
 try{const saved=Number(localStorage.getItem('kriemhild-natural-radius')??2);if(Number.isFinite(saved))radius.value=String(Math.max(0,Math.min(20,saved)));}catch{}
 controls.append(radius);host.replaceChildren(content,controls);content.textContent='Hover over terrain to inspect geology and natural goods.';
 const pinStatus=document.createElement('small'),clearPin=document.createElement('button');clearPin.type='button';clearPin.textContent='Clear pin';clearPin.hidden=true;host.append(pinStatus,clearPin);
 let pinned=null;
 function pinChanged(notify=true){pinStatus.textContent=pinned?`Pinned spot: ${pinned.x.toFixed(2)}, ${pinned.y.toFixed(2)}. Click another spot to move the pin.`:'Click the map to pin a description (use Select or Pan in the builder; turn off the generator brush).';clearPin.hidden=!pinned;if(notify)onPinChange(pinned);}
 pinChanged(false);
 let timer=0,abort=null,sequence=0,solver=null,point=null,key='',disposed=false;const cache=new Map();
 const element=(tag,text)=>{const el=document.createElement(tag);el.textContent=text;return el;};
 function render(data){
  content.replaceChildren(element('strong',`Terrain: ${data.terrain}`),element('small',`Last inspected: ${data.x.toFixed(2)}, ${data.y.toFixed(2)}`));
  content.append(renderClimateDescription(data.climate,element));
  if(!data.available){content.append(element('p','This saved world has no geological-history or resource layer. Its geography has been preserved.'));return;}
  const geology=document.createElement('details');geology.append(element('summary','Geological context'));
  for(const f of deduplicateDescriptions(data.geology,f=>normalized(geologyDescription(f))))geology.append(element('p',geologyDescription(f)));
  content.append(geology,element('strong','Natural goods'));
  if(!data.goods?.length)content.append(element('p','No generated resource occurrences here or within the selected distance.'));
  const list=document.createElement('ul');
  for(const good of uniqueGoods(data.goods)){const row=document.createElement('li');row.append(element('b',naturalGoodLabel(good)),element('span',good.description));if(good.depthMetres>0)row.append(element('small',`Approximate depth: ${Math.round(good.depthMetres)} m`));list.append(row);}
  content.append(list);
  if(data.truncated)content.append(element('small',`Results cover ${data.goods.length} of ${data.total} occurrences; repeated descriptions are combined. Reduce the search distance for a narrower result.`));
 }
 async function request(requestKey,token){
  const controller=new AbortController();abort=controller;
  try{
   const params=new URLSearchParams({x:point.x,y:point.y,radius:radius.value});
   const response=await fetch(`/api/sessions/${solver.id}/world/natural?${params}`,{signal:controller.signal});
   const data=await response.json();if(!response.ok)throw Error(data.error||'Resource query failed');
   if(disposed||token!==sequence)return;
   cache.set(requestKey,data);if(cache.size>64)cache.delete(cache.keys().next().value);render(data);
  }catch(error){if(!disposed&&token===sequence&&error.name!=='AbortError'){content.textContent=`Natural goods unavailable: ${error.message}`;key='';}}
 }
 function schedule(){
  if(!solver?.id||!point)return;
  const nextKey=`${solver.id}/${solver.project?.revision??0}/${point.x}/${point.y}/${radius.value}`;if(nextKey===key)return;key=nextKey;
  clearTimeout(timer);abort?.abort();const token=++sequence;
  if(cache.has(key)){render(cache.get(key));return;}
  content.textContent='Inspecting terrain, geology and natural goods…';timer=setTimeout(()=>request(nextKey,token),140);
 }
 const change=()=>{const value=Number(radius.value);radius.value=String(Number.isFinite(value)?Math.max(0,Math.min(20,value)):2);try{localStorage.setItem('kriemhild-natural-radius',radius.value);}catch{}schedule();};
 radius.addEventListener('change',change);
 const unpin=()=>{pinned=null;pinChanged();};clearPin.addEventListener('click',unpin);
 function setWorld(next){if(next?.id!==solver?.id){clearTimeout(timer);abort?.abort();sequence++;cache.clear();key='';point=null;pinned=null;pinChanged();content.textContent='Hover over terrain to inspect geology and natural goods.';}solver=next;}
 const valid=p=>p&&Number.isFinite(p.x)&&Number.isFinite(p.y);
 return {
  update(next,position){setWorld(next);if(!pinned&&valid(position))point={x:position.x,y:position.y};schedule();},
  pin(next,position){setWorld(next);if(!valid(position))return;pinned=point={x:position.x,y:position.y};pinChanged();schedule();},
  pinned:()=>pinned,
  dispose(){disposed=true;sequence++;clearTimeout(timer);abort?.abort();radius.removeEventListener('change',change);clearPin.removeEventListener('click',unpin);host.replaceChildren();}
 };
}
