import {test} from 'node:test';
import assert from 'node:assert/strict';
import {setTimeout as wait} from 'node:timers/promises';
import {createNaturalHover,naturalGoodLabel,uniqueGoods} from '../src/terrain/natural-hover.js';

class Element {
 constructor(){this.children=[];this.text='';this.listeners={};this.classList={add(){}};}
 set textContent(value){this.text=value;this.children=[];}
 get textContent(){return this.text+this.children.map(c=>c.textContent||'').join(' ');}
 append(...children){this.children.push(...children);}
 replaceChildren(...children){this.text='';this.children=children;}
 setAttribute(){}
 addEventListener(type,fn){this.listeners[type]=fn;}
 removeEventListener(type){delete this.listeners[type];}
}

test('Pinned climate queries refresh after terrain revisions without moving the pin',async t=>{
 const original={document:globalThis.document,fetch:globalThis.fetch,localStorage:globalThis.localStorage};t.after(()=>Object.assign(globalThis,original));
 globalThis.document={createElement:()=>new Element()};globalThis.localStorage={getItem:()=>null,setItem(){}};
 const requests=[];globalThis.fetch=async url=>{requests.push(url);return {ok:true,json:async()=>({terrain:'Hill',x:4,y:5,available:true,geology:[],goods:[]})}};
 const host=new Element(),hover=createNaturalHover(host);t.after(()=>hover.dispose());
 const solver={id:'map',project:{revision:1}};hover.pin(solver,{x:4,y:5});await wait(180);
 solver.project={revision:2};hover.update(solver,{x:12,y:10});await wait(180);
 assert.equal(requests.length,2);assert.deepEqual(hover.pinned(),{x:4,y:5});assert.equal(requests[0],requests[1]);
});
test('Hover distinguishes a deposit beneath the cursor from a nearby occurrence',()=>{
 assert.equal(naturalGoodLabel({name:'Iron ore',location:'here',rating:'Abundant'}),'Iron ore — Abundant');
 assert.equal(naturalGoodLabel({name:'Gold',location:'nearby',distance:1.24}),'Gold — Nearby (1.2 cells)');
});
test('Hover debounces movement, ignores stale replies, changes radius and disposes requests',async t=>{
 const original={document:globalThis.document,fetch:globalThis.fetch,localStorage:globalThis.localStorage};
 t.after(()=>Object.assign(globalThis,original));
 globalThis.document={createElement:()=>new Element()};
 globalThis.localStorage={getItem:()=>null,setItem(){}};
 const pending=[];
 globalThis.fetch=(url,options)=>new Promise(resolve=>pending.push({url,options,resolve}));
 const host=new Element(),hover=createNaturalHover(host);
 t.after(()=>hover.dispose());
 hover.update({id:'world'}, {x:1,y:2});
 hover.update({id:'world'}, {x:3,y:4});
 await wait(180);
 assert.equal(pending.length,1);
 assert.match(pending[0].url,/x=3&y=4&radius=2/);
 hover.update({id:'world'}, {x:5,y:6});
 assert.equal(pending[0].options.signal.aborted,true);
 await wait(180);
 const data=terrain=>({terrain,x:5,y:6,available:true,geology:[],goods:[{name:'Timber',location:'here',rating:'Abundant',description:'Forest biomass supports timber.'}]});
 pending[1].resolve({ok:true,json:async()=>data('Forest')});
 await wait(0);
 pending[0].resolve({ok:true,json:async()=>data('Stale lake')});
 await wait(0);
 assert.match(host.textContent,/Terrain: Forest/);
 assert.doesNotMatch(host.textContent,/Stale lake/);
 assert.match(host.textContent,/Timber/);
 const radius=host.children[1].children[0];
 radius.value='0';radius.listeners.change();
 await wait(180);
 assert.match(pending[2].url,/radius=0/);
 hover.dispose();
 assert.equal(pending[2].options.signal.aborted,true);
 pending[2].resolve({ok:true,json:async()=>data('Disposed')});
 await wait(0);
 assert.equal(host.children.length,0);
});

test('Repeated goods keep local / nearest matches and preserve distinct variants',()=>{
 const base={type:'gold',name:'Gold',description:'River sediments concentrate gold.',location:'nearby',distance:2,abundance:.2};
 const local={...base,id:'local',location:'here',distance:0};
 const items=[base,{...base,distance:1},local,{...base,variant:'primary',description:'Gold in veins.'},{...local,id:'duplicate',description:'  river sediments   concentrate gold.  '}];
 const result=uniqueGoods(items);
 assert.equal(result.length,2);assert.equal(result[0].id,'local');assert.equal(result[1].variant,'primary');
 assert.equal(uniqueGoods([base,{...base,distance:1}])[0].distance,1);
 assert.equal(items.length,5);
});

test('Pin locks the description, repins, supports radius, clears and resets between worlds',async t=>{
 const original={document:globalThis.document,fetch:globalThis.fetch,localStorage:globalThis.localStorage};
 t.after(()=>Object.assign(globalThis,original));
 globalThis.document={createElement:()=>new Element()};globalThis.localStorage={getItem:()=>null,setItem(){}};
 const requests=[],changes=[];globalThis.fetch=(url,options)=>new Promise(resolve=>requests.push({url,options,resolve}));
 const host=new Element(),hover=createNaturalHover(host,p=>changes.push(p));t.after(()=>hover.dispose());
 hover.update({id:'a'},{x:1,y:1});await wait(170);
 hover.pin({id:'a'},{x:8,y:9});hover.update({id:'a'},{x:20,y:30});hover.update({id:'a'},null);await wait(170);
 assert.equal(requests.length,2);assert.match(requests[1].url,/x=8&y=9/);assert.ok(requests[0].options.signal.aborted);
 const data={terrain:'Pinned forest',x:8,y:9,available:true,geology:[{kind:'basin',rock:'shale',ageMa:30,process:'Deposition'},{kind:'basin',rock:'shale',ageMa:30,process:'Deposition'}],goods:[]};
 requests[1].resolve({ok:true,json:async()=>data});await wait(0);requests[0].resolve({ok:true,json:async()=>({...data,terrain:'Stale'})});await wait(0);
 assert.match(host.textContent,/Pinned forest/);assert.doesNotMatch(host.textContent,/Stale/);assert.equal(host.textContent.match(/Deposition/g).length,1);
 const radius=host.children[1].children[0];radius.value='5';radius.listeners.change();await wait(170);assert.match(requests[2].url,/x=8&y=9&radius=5/);
 hover.pin({id:'a'},{x:4,y:5});await wait(170);assert.ok(requests[2].options.signal.aborted);assert.deepEqual(hover.pinned(),{x:4,y:5});
 host.children[3].listeners.click();assert.equal(hover.pinned(),null);assert.equal(changes.at(-1),null);
 hover.update({id:'a'},{x:6,y:7});await wait(170);assert.match(requests.at(-1).url,/x=6&y=7/);
 hover.pin({id:'a'},{x:4,y:5});hover.update({id:'b'},null);assert.equal(hover.pinned(),null);assert.doesNotMatch(host.textContent,/Pinned forest/);
});
