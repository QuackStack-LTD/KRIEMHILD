// Evaluate in the shared preview with a completed disposable world open in Builder.
// Exercises the real navigation/export/import controls; the old session is removed.
(async()=>{
 const checks=[],original={fetch:window.fetch.bind(window),url:URL.createObjectURL,click:HTMLAnchorElement.prototype.click};
 const id=location.pathname.match(/^\/world\/([a-f0-9]+)\/build$/)?.[1];if(!id)throw Error('Open a test world in Builder first.');
 const wait=async(fn,label)=>{const end=Date.now()+45000;while(!fn()&&Date.now()<end)await new Promise(r=>setTimeout(r,100));if(!fn())throw Error(label);};
 const assert=(ok,label)=>{if(!ok)throw Error(label);checks.push(label);};
 let archive,imported,generations=0;
 window.fetch=async(...args)=>{if(String(args[0])==='/api/sessions')generations++;const response=await original.fetch(...args);if(String(args[0])==='/api/projects/import'&&response.ok)imported=await response.clone().json();return response;};
 URL.createObjectURL=blob=>{archive=blob;return original.url.call(URL,blob);};
 HTMLAnchorElement.prototype.click=function(){if(!this.download.endsWith('.world.zip'))return original.click.call(this);};
 try{
  const state=await original.fetch(`/api/projects/${id}/open`,{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'}).then(r=>r.json());
  const entities=await original.fetch(`/api/sessions/${state.id}/world/entities`).then(r=>r.json());
  const tile=state.environment?await original.fetch(`/api/sessions/${state.id}/detail/3/0/0`).then(r=>r.text()):null;
  document.querySelector('.application-nav nav button:last-child').click();await wait(()=>archive&&!document.querySelector('.application-nav nav button:last-child').disabled,'Export did not finish');
  assert(archive.size>10000,'ZIP contains structured world data');
  document.querySelector('.application-nav .brand').click();await wait(()=>!!document.querySelector('.project-hub'),'Project hub did not open');
  await original.fetch(`/api/sessions/${state.id}`,{method:'DELETE'});
  const input=document.querySelector('.hub-import input[accept=".zip"]'),transfer=new DataTransfer();transfer.items.add(new File([archive],'Builder.world.zip',{type:'application/zip'}));input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
  await wait(()=>imported&&location.pathname===`/world/${id}/generate`&&!document.querySelector('.application-nav nav button').disabled,'Import did not reopen the saved world');
  assert(generations===0,'Navigation and import never invoked world generation');
  const replay=await original.fetch(`/api/sessions/${imported.id}/world/entities`).then(r=>r.json());
  assert(JSON.stringify(replay.entities)===JSON.stringify(entities.entities),'Entity identities, geometry and hierarchy survive portable import');
  assert(JSON.stringify(imported.project.history)===JSON.stringify(state.project.history),'Builder history survives portable import');
  if(tile){const restored=await original.fetch(`/api/sessions/${imported.id}/detail/3/0/0`).then(r=>r.text());assert(tile===restored,'Edited explored tile is identical after deleting the original session');}
  [...document.querySelectorAll('.application-nav button')].find(e=>e.textContent==='World Builder').click();await wait(()=>!!document.querySelector('.builder-canvas'),'Builder did not reopen');
  assert(location.pathname===`/world/${id}/build`,'Both editors retain the same world identity');
  return {checks,bytes:archive.size,entities:replay.entities.length,storedTiles:imported.storedDetailCount};
 }finally{window.fetch=original.fetch;URL.createObjectURL=original.url;HTMLAnchorElement.prototype.click=original.click;}
})()
