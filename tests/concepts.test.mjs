import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const registry=await readFile(new URL('../internal/history/registry.json',import.meta.url),'utf8');
const source=(await readFile(new URL('../src/worldbuilding/model.js',import.meta.url),'utf8')).replace("import registry from '../../internal/history/registry.json';",`const registry=${registry};`);
const modelURL='data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const code=(await readFile(new URL('../src/worldbuilding/concepts.js',import.meta.url),'utf8')).replace("'./model.js'",JSON.stringify(modelURL));
const {conceptSearch,conceptCollection,compareConcept,conceptContexts,isConcept}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
test('directory searches structured content and contexts without cloning concepts',()=>{
 const e={id:'r',kind:'religion',name:'River',names:[{name:'Old Waters'}],content:{components:[{id:'c',title:'Ritual',claim:{statement:'Water remembers'},fields:[{definition:{name:'Food'},value:'Pears'}],contexts:[{contextId:'city',stance:'metaphorical'}]}]}};
 for(const q of ['old waters','water remembers','pears','metaphorical'])assert.ok(conceptSearch(e).includes(q));
 assert.deepEqual(conceptContexts(e),['city']);assert.equal(conceptCollection(e),'traditions');assert.ok(isConcept(e));assert.ok(!isConcept({kind:'organization'}));
});
test('component history reports stable identity across renaming, addition and removal',()=>{
 const previous={content:{components:[{id:'a',title:'Old name'},{id:'b',title:'Removed'}]}};
 const current={content:{components:[{id:'a',title:'New name'},{id:'c',title:'Added'}]}};
 assert.deepEqual(compareConcept(current,previous).map(x=>x.status),['modified','added','removed']);
});
