// CLI adapter for the same expressions used in the collaborative preview.
// Point at a disposable test server, not a world containing authored work.
import {chromium} from '@playwright/test';
import {readFile,writeFile} from 'node:fs/promises';

export async function runTerrainAudit(file){
 const browser=await chromium.launch({headless:true,...(process.env.KRIEMHILD_BROWSER_CHANNEL?{channel:process.env.KRIEMHILD_BROWSER_CHANNEL}:{})});
 const page=await browser.newPage({viewport:{width:1280,height:900}}),errors=[];
 page.on('pageerror',e=>{errors.push(e.message);console.error('Browser exception:',e.stack||e.message)});
 try{
  await page.goto(process.env.KRIEMHILD_AUDIT_URL||'http://127.0.0.1:8124');
  // Create a disposable legacy terrain via the API; opening it exercises the
  // actual application routes and persisted state before entering the generator.
  await page.evaluate(async()=>{const r=await fetch('/api/sessions',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({environment:{columns:32,rows:24,seed:'browser-audit',realism:true}})});if(!r.ok)throw new Error(await r.text());const state=await r.json();for(const action of ['step','save']){const saved=await fetch(`/api/sessions/${state.id}/${action}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(action==='step'?{count:10000}:{})});if(!saved.ok)throw new Error(await saved.text());}});
  await page.reload();
  await page.getByRole('button',{name:'Open legacy map'}).first().click();
  await page.getByRole('button',{name:'Terrain Generator',exact:true}).click();
  await page.waitForSelector('#majorContinents');
  await page.waitForFunction(()=>document.querySelector('#statusLine')?.textContent.startsWith('Done')&&!document.querySelector('#generate')?.disabled);
  // Confirm only fixture regeneration/preset dialogs in this disposable server.
  await page.evaluate(()=>{const accept=()=>{for(const b of document.querySelectorAll('button'))if(b.textContent.trim()==='Confirm'&&!b.disabled)b.click()};const observer=new MutationObserver(accept);observer.observe(document.body,{childList:true,subtree:true});window.__fixtureDialogs=observer;});
  await page.evaluate(await readFile(file,'utf8'));
  const result=await page.evaluate(()=>({realism:window.__realismAudit,detail:window.__detailAudit,biome:window.__biomeAudit}));
  if(errors.length)throw new Error(errors.join('\n'));
  console.log(JSON.stringify(result,null,2));
  if(process.env.KRIEMHILD_AUDIT_REPORT)await writeFile(process.env.KRIEMHILD_AUDIT_REPORT,JSON.stringify(result,null,2));
  if(process.env.KRIEMHILD_AUDIT_SCREENSHOT)await page.screenshot({path:process.env.KRIEMHILD_AUDIT_SCREENSHOT});
 }catch(error){console.error({errors,page:await page.evaluate(()=>({preset:document.querySelector('#preset')?.value,status:document.querySelector('#statusLine')?.textContent,error:document.querySelector('#error')?.textContent}))});throw error}
 finally{await browser.close()}
}
