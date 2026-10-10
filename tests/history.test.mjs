import {test} from 'node:test';
import assert from 'node:assert/strict';
import {timelineWindow,graphLayout,edgePath,mergeConflicts,ageActionSource} from '../src/history.js';
test('Timeline window centers the selected Age with at most two neighbors each way',()=>{
 const t={ages:Array.from({length:12},(_,i)=>String(i))};for(let i=0;i<12;i++){const w=timelineWindow(t,String(i));assert.ok(w.ages.length<=5);assert.deepEqual(w.ages,t.ages.slice(Math.max(0,i-2),i+3));}assert.equal(timelineWindow(t,'11').after,false);assert.equal(timelineWindow(t,'5').before,true);
});
test('Merge review reports geometry and deletion conflicts but not unique objects',()=>{
 const doc={maps:[],entities:[{id:'city',ageId:'a',name:'City'},{id:'city',ageId:'b',name:'City',deleted:true},{id:'keep',ageId:'b',name:'Keep'}],representations:[{entityId:'city',ageId:'a',mapId:'map',shape:{points:[[1,2],[2,3],[3,1]]}}]};
 assert.deepEqual(mergeConflicts(doc,'a','b').map(c=>c.kind),['Entity deletion']);
 doc.entities[1].deleted=false;
 doc.representations.push({...doc.representations[0],ageId:'b'});
 assert.deepEqual(mergeConflicts(doc,'a','b'),[]);
 doc.representations[1]={...doc.representations[1],shape:{points:[[1,2],[2,4],[3,1]]}};
 assert.equal(mergeConflicts(doc,'a','b')[0].kind,'Entity properties / geometry');
});
test('Graph layout retains independent lanes and forward branch/merge dependencies',()=>{
 const d={ages:['a','b','c','d','e'].map(id=>({id})),timelines:[{ages:['a','b']},{ages:['c','d']},{ages:['e']}],edges:[{source:'a',destination:'b'},{source:'a',destination:'c'},{source:'c',destination:'d'},{source:'b',destination:'e'},{source:'d',destination:'e'}]};const l=graphLayout(d);assert.equal(l.positions.get('a').y,l.positions.get('b').y);assert.notEqual(l.positions.get('a').y,l.positions.get('c').y);for(const e of d.edges)assert.ok(l.positions.get(e.destination).x>l.positions.get(e.source).x);assert.match(edgePath(l.positions.get('d'),l.positions.get('e')),/ C /);
});

test('New Age targets the selected graph Age after switching away from a new branch',()=>{
 const doc={world:{currentAge:'new-branch'},ages:[{id:'original',timelineId:'first'},{id:'new-branch',timelineId:'second'}]};
 assert.equal(ageActionSource(doc,'graph','original'),'original');
 assert.equal(ageActionSource(doc,'graph','new-branch'),'new-branch');
 assert.equal(ageActionSource(doc,'world','original'),'new-branch');
});

test('Chronological loops and self-connections have finite stable layouts and curved return paths',()=>{
 const doc={ages:['a','b','c'].map(id=>({id})),timelines:[{ages:['a','b']},{ages:['c']}],edges:[{source:'a',destination:'b'},{source:'b',destination:'c'},{source:'c',destination:'a'},{source:'a',destination:'a'}]};
 const layout=graphLayout(doc);assert.deepEqual(layout,graphLayout(doc));
 for(const p of layout.positions.values())assert.ok(Number.isFinite(p.x)&&Number.isFinite(p.y));
 assert.ok(layout.positions.get('b').x>layout.positions.get('a').x);
 assert.match(edgePath(layout.positions.get('b'),layout.positions.get('a')),/ C /);
 assert.match(edgePath(layout.positions.get('a'),layout.positions.get('a')),/ C /);
});
