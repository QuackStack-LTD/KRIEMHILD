import test from 'node:test';
import assert from 'node:assert/strict';
import {parseRoute} from '../src/project.js';
import {builderPreferences} from '../src/builder/preferences.js';
import {allowedGeometry,drapeSegment} from '../src/builder/geometry.js';
import {meshSurface} from '../src/terrain/mesh-surface.js';

test('project routes retain stable identity across both editors',()=>{
 const id='0123456789abcdef0123456789abcdef';
 assert.deepEqual(parseRoute('/'),{view:'home'});
 assert.deepEqual(parseRoute('/world/new'),{view:'generate',fresh:true});
 for(const view of ['generate','build'])assert.deepEqual(parseRoute(`/world/${id}/${view}`),{id,view});
 for(const url of ['/world/unknown/build',`/world/${id}/delete`,'/assets/missing.js'])assert.equal(parseRoute(url).view,'missing');
});
test('object geometry choices and black defaults match Builder rules',()=>{
 assert.equal(builderPreferences({}).color,'#000000');
 for(const k of ['village','town','city','capital','custom-settlement'])assert.deepEqual(allowedGeometry('settlements',k),['polygon']);
 for(const k of ['road','bridge','railway'])assert.deepEqual(allowedGeometry('roads',k),['line']);
 assert.deepEqual(allowedGeometry('buildings','house'),['point','polygon']);
 for(const k of ['factory','landmark'])assert.deepEqual(allowedGeometry('buildings',k),['point']);
 assert.deepEqual(allowedGeometry('buildings','custom-structure'),['point','line','polygon']);
 assert.equal(builderPreferences({tools:{layer:'settlements',kind:'village',tool:'line'}}).tool,'polygon');
});
test('draped edges follow ridges and depressions along terrain triangles',()=>{
 // A ridge at x=1 and depression at x=2. Endpoints alone would bridge both.
 const vertices=[];for(let y=0;y<2;y++)for(let x=0;x<4;x++)vertices.push(x,[0,10,-5,0][x]+y*2,y);
 const sample=(x,y)=>meshSurface(vertices,4,2,x,y);
 const path=drapeSegment([0,.25],[3,.75],1,sample,{x:0,y:0,width:3,height:1});
 assert.ok(path.length>4);assert.ok(path.some(p=>p[1]>9));assert.ok(path.some(p=>p[1]<-3));
 for(let i=1;i<path.length;i++){
  const a=path[i-1],b=path[i],middle=a.map((v,k)=>(v+b[k])/2),ground=sample(middle[0],middle[2]);
  assert.ok(Math.abs(middle[1]-ground[1])<1e-9,'line cuts through or bridges a terrain triangle');
 }
 assert.deepEqual(drapeSegment([-4,0],[-1,0],1,sample,{x:0,y:0,width:3,height:1}),[]);
});
test('Builder preferences survive storage and constrain malformed imports',()=>{
 const tools={tool:'crater',layer:'roads',kind:'bridge',name:'Stone Bridge',color:'#aabbcc',radius:4,amount:150,overlay:'elevation'};
 assert.deepEqual(builderPreferences(JSON.parse(JSON.stringify({tools}))),tools);
 const invalid=builderPreferences({tools:{tool:'execute',layer:'missing',overlay:'nonsense',radius:-99,amount:99999,color:'javascript:bad'}});
 assert.equal(invalid.tool,'select');assert.equal(invalid.layer,'settlements');assert.equal(invalid.overlay,'terrain');assert.equal(invalid.radius,.01);assert.equal(invalid.amount,12000);assert.match(invalid.color,/^#[0-9a-f]{6}$/);
});
