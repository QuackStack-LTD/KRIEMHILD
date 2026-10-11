import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const registry=await readFile(new URL('../internal/history/registry.json',import.meta.url),'utf8');
const source=(await readFile(new URL('../src/worldbuilding/model.js',import.meta.url),'utf8')).replace("import registry from '../../internal/history/registry.json';",`const registry=${registry};`);
const modelURL='data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const graphSource=(await readFile(new URL('../src/worldbuilding/connections.js',import.meta.url),'utf8')).replace("'./model.js'",JSON.stringify(modelURL));
const {connectionKinds,peopleGraph,reconnectRelations}=await import('data:text/javascript;base64,'+Buffer.from(graphSource).toString('base64'));
const e=(id,kind='character',properties={})=>({id,kind,name:id,ageId:'a',properties});
const a=e('a'),b=e('b'),city=e('city','city'),org=e('org','organization');
test('connection choices distinguish personal, familial, location and institutional endpoints',()=>{
 const kinds=connectionKinds(a,b);assert.ok(kinds.includes('child-of')&&kinds.includes('friend'));for(const k of ['born-in','resides-in','part-of','member-of','controls'])assert.ok(!kinds.includes(k));
 assert.ok(connectionKinds(a,b,'family').includes('child-of'));assert.ok(!connectionKinds(a,b,'family').includes('friend'));assert.ok(!connectionKinds(a,b,'social').includes('child-of'));
 assert.ok(connectionKinds(a,city).includes('born-in'));assert.ok(!connectionKinds(a,city).includes('friend'));
 assert.ok(connectionKinds(a,org).includes('member-of'));assert.deepEqual(connectionKinds(city,org),[]);
});
test('combined graph shares identities, positions, context edges and isolated characters; focused graphs separate families and social links',()=>{
 const d={world:{},entities:[{...a,properties:{graphPosition:{x:300,y:220}}},b,e('isolated'),city,org],relations:[{id:'family',ageId:'a',source:'a',destination:'b',kind:'spouse'},{id:'social',ageId:'a',source:'a',destination:'b',kind:'friend'},{id:'home',ageId:'a',source:'a',destination:'city',kind:'resides-in'},{id:'member',ageId:'a',source:'a',destination:'org',kind:'member-of'}]};
 const g=peopleGraph(d,'a');assert.equal(g.nodes.length,5);assert.equal(g.edges.length,4);assert.equal(g.positions.get('a').x,300);assert.equal(g.positions.get('a').width,160);assert.equal(g.positions.get('city').width,76);assert.equal(g.positions.get('city').height,76);assert.equal(g.positions.get('org').width,76);
 assert.deepEqual(peopleGraph(d,'a',{mode:'family'}).edges.map(r=>r.id),['family']);assert.deepEqual(peopleGraph(d,'a',{mode:'social'}).edges.map(r=>r.id),['social']);assert.deepEqual(peopleGraph(d,'a',{mode:'places'}).edges.map(r=>r.id),['home','member']);
 assert.ok(peopleGraph(d,'a',{center:'a',extras:['isolated']}).nodes.some(e=>e.id==='isolated'));
 assert.ok(!peopleGraph(d,'a',{hidden:['b']}).edges.some(r=>r.destination==='b'));assert.equal(d.relations.length,4);
});

test('reconnecting one endpoint preserves identity, metadata and the other endpoint',()=>{
 const original={id:'friendship',source:'a',destination:'b',sourcePort:'e',destinationPort:'w',kind:'friend',description:'Old companions',validFrom:'42'};
 const [changed]=reconnectRelations([original],'b',{id:'c',port:'n'});
 assert.deepEqual(changed,{...original,destination:'c',destinationPort:'n'});assert.equal(original.destination,'b');
 assert.throws(()=>reconnectRelations([original],'missing',{id:'c',port:'n'}));
 const parents=[{id:'p1',source:'child',destination:'a',kind:'child-of'},{id:'p2',source:'child',destination:'b',kind:'child-of'}];
 const moved=reconnectRelations(parents,'child',{id:'new-child',port:'nw'});assert.deepEqual(moved.map(r=>r.destination),['a','b']);assert.ok(moved.every(r=>r.source==='new-child'&&r.sourcePort==='nw'));
});
test('personal graph supports concept beliefs without adding spatial ownership edges',()=>{
 assert.ok(connectionKinds(a,e('belief','doctrine')).includes('affirms'));
 assert.ok(!connectionKinds(a,e('belief','doctrine')).includes('has-component'));
});
