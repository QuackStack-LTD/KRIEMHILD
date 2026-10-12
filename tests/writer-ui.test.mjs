import test from 'node:test';
import assert from 'node:assert/strict';
import { JSDOM } from 'jsdom';
import { build } from 'esbuild';
import { mkdtemp, rm } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';
import { resolve, join } from 'node:path';
import React,{act} from 'react';
const dom=new JSDOM('<div id="root"></div>',{url:'http://localhost'});
for(const k of ['window','document','HTMLElement','Element','Node','DocumentFragment','MutationObserver','DOMParser','KeyboardEvent','MouseEvent','Event','localStorage','getComputedStyle'])globalThis[k]=dom.window[k];
Object.defineProperty(globalThis,'navigator',{value:dom.window.navigator,configurable:true});
globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.requestAnimationFrame=fn=>setTimeout(fn,0);globalThis.cancelAnimationFrame=clearTimeout;
dom.window.HTMLElement.prototype.getClientRects=()=>[{width:100,height:40}];dom.window.Range.prototype.getClientRects=()=>[];dom.window.Range.prototype.getBoundingClientRect=()=>({top:0,left:0,bottom:0,right:0});
const {createRoot}=await import('react-dom/client');
const temp=await mkdtemp(resolve('.tools/writer-ui-'));const output=join(temp,'components.mjs');
await build({stdin:{contents:`export {default as Writing} from './src/worldbuilding/Writing.jsx';export {default as useWriter} from './src/worldbuilding/writer/useWriter.js';export {default as UIProvider} from './src/worldbuilding/ui/UIProvider.jsx';`,resolveDir:process.cwd(),loader:'jsx'},outfile:output,bundle:true,format:'esm',platform:'node',packages:'external',jsx:'automatic',loader:{'.css':'empty'}});
const {Writing,useWriter,UIProvider}=await import(pathToFileURL(output));
let root,hook,server,requests,mode,release;
const p={id:'project',worldId:'world',name:'Test novel',revision:1,settings:{defaultAge:'age'}};
const scene={id:'scene',projectId:p.id,parentId:'',title:'Opening',type:'scene',position:0,status:'Drafting',contentSchema:1,content:{type:'doc',content:[{type:'paragraph',content:[{type:'text',text:'Opening words'}]}]},text:'Opening words',revision:1,references:[],storyAgeId:'age'};
function Harness(){hook=useWriter('world');return React.createElement('div',null,hook.status)}
async function mount(component=React.createElement(Harness)){
 server=structuredClone(scene);requests=[];mode='ok';localStorage.clear();
 globalThis.fetch=async(path,opts={})=>{let data;if(path.includes('/nodes')){const n=JSON.parse(opts.body).node;requests.push(n);if(mode==='defer'){await new Promise(r=>release=r);mode='ok'}if(mode==='fail')return {ok:false,status:503,json:async()=>({error:'Server offline'})};if(mode==='conflict'||n.revision!==server.revision)return {ok:false,status:409,json:async()=>({error:'Newer server revision'})};server={...n,revision:n.revision+1};data=server}else if(path.includes('/projects?'))data=[p];else data={project:p,nodes:[server]};return {ok:true,status:200,json:async()=>structuredClone(data)}};
 root=createRoot(document.getElementById('root'));await act(async()=>{root.render(component);await new Promise(r=>setTimeout(r,40))});await act(()=>new Promise(r=>setTimeout(r,20)));
}
test.afterEach(async()=>{if(root)await act(()=>root.unmount());root=null;hook=null});
test.after(async()=>{await rm(temp,{recursive:true,force:true});dom.window.close()});
test('debounced autosave waits for server confirmation and retains failed edits',async()=>{await mount();await act(()=>hook.edit({title:'Unsaved title'}));assert.equal(hook.status,'Unsaved changes');assert.equal(requests.length,0);await act(()=>new Promise(r=>setTimeout(r,1200)));assert.equal(hook.status,'Saved');assert.equal(server.title,'Unsaved title');mode='fail';await act(()=>hook.edit({title:'Offline edit'}));await act(()=>hook.flush());assert.equal(hook.status,'Offline / save failed');assert.equal(hook.dirty,true);assert.match(localStorage.getItem('kriemhild.writer.recovery.project.scene'),/Offline edit/);mode='ok';await act(()=>hook.flush());assert.equal(hook.status,'Saved');assert.equal(localStorage.getItem('kriemhild.writer.recovery.project.scene'),null)});
test('edits during an in-flight save are flushed before navigation returns',async()=>{await mount();await act(()=>hook.edit({title:'First'}));mode='defer';let pending;await act(()=>{pending=hook.flush()});await act(()=>hook.edit({title:'While saving'}));await act(async()=>{release();await pending});assert.equal(requests.length,2);assert.equal(server.title,'While saving');assert.equal(server.revision,3);assert.equal(hook.dirty,false)});
test('stale conflicts block switching and preserve local and server copies',async()=>{await mount();mode='conflict';await act(()=>hook.edit({title:'Local copy'}));let chosen;await act(async()=>{chosen=await hook.choose({...scene,id:'other'})});assert.equal(chosen,false);assert.equal(hook.draft.id,'scene');assert.equal(hook.status,'Conflict needs attention');assert.equal(server.title,'Opening');assert.match(localStorage.getItem('kriemhild.writer.recovery.project.scene'),/Local copy/)});
test('workspace renders semantic editor, planning views and focus controls',async()=>{
 const doc={world:{id:'world',name:'Test World',currentAge:'age'},ages:[{id:'age',name:'First Age'}],entities:[],relations:[]};
 await mount(React.createElement(UIProvider,null,React.createElement(Writing,{doc,onBack:()=>{}})));
 assert.ok(document.querySelector('[role=textbox][aria-label=Manuscript]'));
 assert.equal(document.querySelector('textarea.writing-editor'),null);
 const click=async text=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent===text);assert.ok(b,text);await act(()=>b.click())};
 await click('outline');assert.ok(document.querySelector('.writer-outline'));await click('corkboard');assert.ok(document.querySelector('.writer-corkboard'));await click('write');await click('Context & notes');assert.ok(document.querySelector('.writer-context'));await click('Focus');assert.equal(document.querySelector('.writer-context'),null);assert.equal(document.querySelector('.writer-navigator'),null);await click('Exit focus');assert.ok(document.querySelector('.writer-context'));
});
