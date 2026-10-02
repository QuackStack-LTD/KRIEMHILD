import {spawn} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import {performance} from 'node:perf_hooks';
import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const count=Number(process.env.KRIEMHILD_BENCH_RECORDS||1000),ages=Number(process.env.KRIEMHILD_BENCH_AGES||5);
if(!Number.isInteger(count)||count<1||count>50000||!Number.isInteger(ages)||ages<1||ages>100)throw Error('Use 1–50000 records and 1–100 Ages');
const base='http://127.0.0.1:4786';
const library=path.join(root,'.bench-worlds',randomUUID());await fs.mkdir(library,{recursive:true});
const child=spawn(path.join(root,'bin',process.platform==='win32'?'kriemhild-dev.exe':'kriemhild-dev'),['-addr','127.0.0.1:4786','-data',library,'-web','apps/web/out-dev'],{cwd:root,windowsHide:true,stdio:['ignore','ignore','pipe']});
let startupError='';child.stderr.on('data',d=>startupError+=d);child.on('error',e=>startupError=e.message);let cookie='';
async function api(route,body){const r=await fetch(base+'/api/v1'+route,{headers:{Origin:base,Cookie:cookie,'Content-Type':'application/json'},...(body?{method:'POST',body:JSON.stringify(body)}:{}),signal:AbortSignal.timeout(120000)});cookie=r.headers.getSetCookie().map(c=>c.split(';')[0]).join('; ')||cookie;const data=await r.json();if(!r.ok)throw Error(data.error||r.statusText);return data}
const samples=async(fn,n=12)=>{const ms=[];for(let i=0;i<n;i++){const start=performance.now();await fn();ms.push(performance.now()-start)}ms.sort((a,b)=>a-b);return {p50:Math.round(ms[Math.floor(ms.length*.5)]),p95:Math.round(ms[Math.min(ms.length-1,Math.floor(ms.length*.95))]),samples:n}};
try{
 let ready=false;for(let i=0;i<100;i++){if(startupError)throw Error(startupError);let session;try{session=await api('/session')}catch{}if(session){if(path.resolve(session.library)!==path.resolve(library))throw Error('Benchmark port belongs to another server; refusing to write to its library');ready=true;break}await new Promise(r=>setTimeout(r,100))}if(!ready)throw Error('Benchmark server did not start');
 let state=await api('/projects',{name:`Benchmark ${count} identities / ${ages} Ages`,age:'Age 1'});const prefix=`/projects/${state.root.world.id}`;
 const put=async command=>{state=await api(prefix+'/commands',{...command,expected:state.revision,age:state.age.id})};
 for(let offset=0;offset<count;offset+=500){const records=Array.from({length:Math.min(500,count-offset)},(_,i)=>({id:randomUUID(),kind:'entity',type:'Settlement',name:`Settlement ${String(offset+i).padStart(6,'0')}`,notes:'Authored reference text for repeatable storage measurements.'}));await put({action:'put-many',records})}
 const copyTimes=[];for(let i=1;i<ages;i++){const started=performance.now();await put({action:'copy-age',name:`Age ${i+1}`});copyTimes.push(Math.round(performance.now()-started))}
 const search=()=>api(prefix+`/search-page?age=${state.age.id}&q=Settlement`);const indexStarted=performance.now();await search();const coldSearchMs=Math.round(performance.now()-indexStarted);
 const report={createdAt:new Date().toISOString(),platform:process.platform,node:process.version,fixture:{identities:count,ages,relations:0,events:0,mediaBytes:0,terrainCells:0},scope:'Storage/API baseline only. This is NOT the full planned reference world or a browser rendering/typing/accessibility benchmark. Search returns the first 100 matching records; coldSearch includes index construction.',stateReadMs:await samples(()=>api(prefix+`/state?age=${state.age.id}`)),coldSearchMs,warmSearchPageMs:await samples(search),ageCopyMs:copyTimes,library};
 await fs.mkdir(path.join(root,'.tools'),{recursive:true});const output=path.join(root,'.tools','benchmark-latest.json');await fs.writeFile(output,JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify(report,null,2));console.log('Saved '+output);
}finally{child.kill()}
