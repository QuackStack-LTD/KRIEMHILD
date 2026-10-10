export async function historyRequest(path='',body,method=body===undefined?'GET':'POST'){
 const response=await fetch(`/api/worlds${path}`,{method,headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});
 if(response.status===204)return null;const data=await response.json();if(!response.ok)throw Error(data.error||'World request failed');return data;
}
export function timelineWindow(timeline,current){const index=timeline.ages.indexOf(current);return {index,ages:timeline.ages.slice(Math.max(0,index-2),index+3),before:index>2,after:index+3<timeline.ages.length};}
// Collapse strongly connected components before ranking. Chronological loops
// share a component; timeline order still determines each lane's visual order.
export function graphLayout(document){
 const ids=document.timelines.flatMap(t=>t.ages),out=new Map(ids.map(id=>[id,[]])),back=new Map(ids.map(id=>[id,[]]));
 for(const e of document.edges){if(!out.has(e.source)||!out.has(e.destination))continue;out.get(e.source).push(e.destination);back.get(e.destination).push(e.source);}
 const visited=new Set(),finish=[];
 for(const root of ids){if(visited.has(root))continue;const stack=[[root,0]];visited.add(root);while(stack.length){const top=stack[stack.length-1],next=out.get(top[0]);if(top[1]<next.length){const child=next[top[1]++];if(!visited.has(child)){visited.add(child);stack.push([child,0]);}}else{finish.push(top[0]);stack.pop();}}}
 const components=new Map();let count=0;
 for(const root of finish.reverse()){if(components.has(root))continue;const stack=[root];components.set(root,count);while(stack.length){for(const child of back.get(stack.pop()))if(!components.has(child)){components.set(child,count);stack.push(child);}}count++;}
 const edges=Array.from({length:count},()=>new Set()),degree=Array(count).fill(0),ranks=Array(count).fill(0);
 for(const e of document.edges){const a=components.get(e.source),b=components.get(e.destination);if(a!==undefined&&b!==undefined&&a!==b&&!edges[a].has(b)){edges[a].add(b);degree[b]++;}}
 const queue=[];degree.forEach((n,i)=>{if(!n)queue.push(i)});for(let head=0;head<queue.length;head++){const a=queue[head];for(const b of edges[a]){ranks[b]=Math.max(ranks[b],ranks[a]+1);if(--degree[b]===0)queue.push(b);}}
 const positions=new Map();document.timelines.forEach((t,lane)=>{let previous=-1;for(const id of t.ages){const x=Math.max(ranks[components.get(id)]||0,previous+1);positions.set(id,{x:100+x*210,y:90+lane*140,lane});previous=x;}});
 return {positions,width:Math.max(800,...[...positions.values()].map(p=>p.x+230)),height:Math.max(360,document.timelines.length*140+100)};
}
export function edgePath(a,b){if(a.y===b.y){if(b.x>a.x)return `M ${a.x} ${a.y} L ${b.x} ${b.y}`;return `M ${a.x} ${a.y} C ${a.x+70} ${a.y-80}, ${b.x-70} ${b.y-80}, ${b.x} ${b.y}`;}const bend=Math.max(70,Math.abs(b.x-a.x)*.5);return `M ${a.x} ${a.y} C ${a.x+bend} ${a.y}, ${b.x-bend} ${b.y}, ${b.x} ${b.y}`;}
export function mergeConflicts(doc,primary,secondary){
 const otherMaps=new Map(doc.maps.filter(m=>m.ageId===secondary).map(m=>[m.id,m]));
 const conflicts=doc.maps.filter(m=>m.ageId===primary&&otherMaps.has(m.id)&&m.snapshot!==otherMaps.get(m.id).snapshot).map(m=>({id:m.id,name:m.name,kind:'Map snapshot'}));
 const others=new Map(doc.entities.filter(e=>e.ageId===secondary).map(e=>[e.id,e]));
 const signature=e=>JSON.stringify({name:e.name,kind:e.kind,properties:e.properties,deleted:!!e.deleted,shapes:doc.representations.filter(r=>r.ageId===e.ageId&&r.entityId===e.id).map(r=>({map:r.mapId,shape:r.shape})).sort((a,b)=>a.map.localeCompare(b.map))});
 for(const e of doc.entities.filter(e=>e.ageId===primary)){const other=others.get(e.id);if(other&&signature(e)!==signature(other))conflicts.push({id:e.id,name:e.name,kind:e.deleted||other.deleted?'Entity deletion':'Entity properties / geometry'});}
 return conflicts;
}

// All graph actions use the inspected Age, including immediately after branching.
export function ageActionSource(doc,page,selected){return page==='graph'&&doc.ages.some(a=>a.id===selected)?selected:doc.world.currentAge;}
