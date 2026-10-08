// Run in the shared preview. Capture the generated download in memory, then
// import that exact ZIP through the real file input and builder handler.
(async()=>{
 const $=id=>document.getElementById(id),checks=[],old={raf:requestAnimationFrame,caf:cancelAnimationFrame,fetch:window.fetch.bind(window),url:URL.createObjectURL,click:HTMLAnchorElement.prototype.click};
 const task=fn=>{const c=new MessageChannel();c.port1.onmessage=()=>{c.port1.close();c.port2.close();fn()};c.port2.postMessage(0);return c;};
 window.requestAnimationFrame=fn=>task(()=>fn(performance.now()));window.cancelAnimationFrame=c=>{if(c?.port1){c.port1.close();c.port2.close()}else old.caf.call(window,c)};
 let archive,created,restored,creates=0;
 window.fetch=async(...args)=>{const response=await old.fetch(...args),url=String(args[0]);if(url==='/api/sessions'){created=await response.clone().json();creates++;}if(url==='/api/projects/import')restored=await response.clone().json();return response;};
 URL.createObjectURL=blob=>{archive=blob;return old.url.call(URL,blob)};
 HTMLAnchorElement.prototype.click=function(){if(!this.download.endsWith('.world.zip'))return old.click.call(this);};
 const wait=async(fn,label,ms=45000)=>{const end=Date.now()+ms;while(!fn()&&Date.now()<end)await new Promise(task);if(!fn())throw new Error(label+': '+$('error').textContent);};
 const change=(id,value)=>{const el=$(id);if(el.type==='checkbox')el.checked=value;else el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}));};
 const assert=(ok,message)=>{if(!ok)throw new Error(message)};
 try{
  change('view3d',false);$('instant').checked=true;
  if(!$('environment').checked){change('environment',true);await wait(()=>$('statusLine').textContent.startsWith('Done'),'physical mode');}
  $('width').value=64;$('height').value=48;$('generate').click();await wait(()=>created&&$('statusLine').textContent.startsWith('Done'),'generation');
  $('fitMap').click();await new Promise(task);
  const canvas=$('detailMap'),rect=canvas.getBoundingClientRect();canvas.dispatchEvent(new WheelEvent('wheel',{clientX:rect.left+rect.width*.55,clientY:rect.top+rect.height*.45,deltaY:-1800,bubbles:true,cancelable:true}));
  await wait(()=>Number(canvas.dataset.detail)>4&&!$('detailStatus').textContent.includes('refining'),'explored detail');
  const before={scale:Number(canvas.dataset.scale),x:Number(canvas.dataset.offsetX),y:Number(canvas.dataset.offsetY)};
  const path=performance.getEntriesByType('resource').findLast(e=>e.name.includes('/sessions/'+created.id+'/detail/')).name;
  const tile=await old.fetch(path).then(r=>r.text());
  $('saveProject').click();await wait(()=>archive&&!$('saveProject').disabled,'world ZIP save');assert(archive.size>10000,'ZIP has no world content');checks.push('Save world ZIP contains structured world and explored detail');
  // Delete the original session/cache before importing. There is no original
  // server-side project state for the import path to accidentally reuse.
  await old.fetch('/api/sessions/'+created.id,{method:'DELETE'});
  const count=creates,transfer=new DataTransfer();transfer.items.add(new File([archive],'KRIEMHILD.world.zip',{type:'application/zip'}));$('openProject').files=transfer.files;$('openProject').dispatchEvent(new Event('change',{bubbles:true}));
  await wait(()=>restored&&!$('openProject').disabled&&$('projectStatus').textContent.startsWith('World loaded'),'world ZIP import');
  assert(creates===count,'Import generated a replacement world');assert(JSON.stringify(restored.environment)===JSON.stringify(created.environment),'Stored geography changed');
  const replay=await old.fetch(path.replace(created.id,restored.id)).then(r=>r.text());assert(replay===tile,'Explored tile regenerated or changed');checks.push('Import restores identical world and tile bytes after deleting the original server session');
  await wait(()=>!$('detailStatus').textContent.includes('refining'),'restored view detail');
  assert(Math.abs(Number(canvas.dataset.scale)-before.scale)<1e-8&&Math.abs(Number(canvas.dataset.offsetX)-before.x)<1e-6&&Math.abs(Number(canvas.dataset.offsetY)-before.y)<1e-6,'Exploration camera not restored');checks.push('Saved zoom and geographic view are restored without regeneration');
  assert(!$('error').textContent,$('error').textContent);
  return {checks,bytes:archive.size,storedTiles:restored.storedDetailCount,status:$('projectStatus').textContent};
 }finally{window.requestAnimationFrame=old.raf;window.cancelAnimationFrame=old.caf;window.fetch=old.fetch;URL.createObjectURL=old.url;HTMLAnchorElement.prototype.click=old.click;}
})()

