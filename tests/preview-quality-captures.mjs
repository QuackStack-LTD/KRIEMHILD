import { chromium } from "@playwright/test";
import { mkdir,writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
const out='docs/preview-climate-investigation/matched-previews',records=[];
await mkdir(out,{recursive:true});
const browser=await chromium.launch({headless:true,executablePath:process.env.KRIEMHILD_BROWSER_EXECUTABLE});
try{
 for(const renderer of ['reference','balanced']){
  const page=await browser.newPage({viewport:{width:1280,height:800},deviceScaleFactor:1});
  await page.addInitScript(()=>localStorage.setItem('kriemhild.preview-quality.v1',JSON.stringify({preset:'balanced',customScale:1})));
  await page.route('**/api/**',async route=>{
   const u=new URL(route.request().url());u.port='8128';const headers={...route.request().headers(),origin:'http://127.0.0.1:8128',referer:'http://127.0.0.1:8128/'};delete headers.host;
   const response=await route.fetch({url:u.href,headers});
   if(u.pathname.endsWith('/open')){const state=await response.json();state.projectUI||={};state.projectUI.continuousView={camera:{x:0,y:0,w:2,h:1},style:'natural',hillshade:.55,mode:'map',showRivers:true};await route.fulfill({response,json:state});}else await route.fulfill({response});
  });
  await page.goto(`http://127.0.0.1:${renderer==='reference'?8128:8127}/world/977d134ee5aadc480e1410ade3c84587/build?tileDebug=1`);
  await page.getByLabel('Autosave accepted terrain after 30 seconds').uncheck();
  for(const [name,w]of[['global',2],['continental',.25],['regional',.03],['local',.003],['extreme',.00008]]){
   await page.evaluate(detail=>window.dispatchEvent(new CustomEvent('terrain-diagnostic-camera',{detail})),{x:w===2?0:1.3-w/2,y:w===2?0:.22-w/4,w,h:w/2});
   await page.waitForTimeout(150);
   await page.waitForFunction(()=>{const r=window.__terrainViewportLog?.findLast(e=>e.event==='request'),g=document.querySelector('[aria-label="Committed terrain viewport"]');return r&&g?.dataset.phase==='final'&&+g.dataset.generation===r.generation&&document.querySelector('footer')?.textContent.endsWith('Ready');},null,{timeout:180000});
   await page.screenshot({path:`${out}/${renderer}-${name}.png`});
   const state=await page.evaluate(()=>({camera:document.querySelector('.studio-stage svg').getAttribute('viewBox'),request:window.__terrainViewportLog.findLast(e=>e.event==='request'),frame:window.__terrainViewportLog.findLast(e=>e.event==='frame')}));
   records.push({renderer,name,...state});
  }
  await page.close();
 }
 for(const name of ['global','continental','regional','local','extreme']){
  const [a,b]=records.filter(r=>r.name===name);assert.equal(a.camera,b.camera);assert.deepEqual(a.request.bounds,b.request.bounds);assert.equal(a.request.revision,b.request.revision);
 }
 await writeFile(out+'/results.json',JSON.stringify(records,null,2));
}finally{await browser.close();}
