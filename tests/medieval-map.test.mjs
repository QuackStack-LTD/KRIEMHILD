import test from 'node:test';
import assert from 'node:assert/strict';
import {renderMedievalMap,riverMetrics,waterGeometry,featureAnchor,wrapPlaceName,illustrationEntities} from '../src/builder/medieval-map-export.js';
const geo={world_scale:100,planetary_radius:6371,latitude_north:80,latitude_south:-80,longitude_west:-180,longitude_east:180};
const context={geography:geo,terrainWidth:100,terrainHeight:60,scale:10,bounds:{x:0,y:0,width:99,height:59}};
const stream={id:'stream',kind:'river',class:'stream',width:.00003,physicalWidthMetres:2,dischargeM3s:.5,path:[[10,10,40],[11,11,30]]};
const major={...stream,id:'major',riverId:'continental',class:'major',width:.004,physicalWidthMetres:400,dischargeM3s:5000,widths:[.002,.004],dischargesM3s:[2000,5000]};
function fixture(){return {name:'The Marches',revision:17,bounds:{x:0,y:0,width:10,height:6},terrainWidth:11,terrainHeight:7,geography:geo,columns:11,rows:7,samples:Array.from({length:77},(_,i)=>[i%11<2||i%11>8?1:0,2000,i%3?.8:0,.8]),entities:[{id:'capital',name:'A & B <Keep>',kind:'capital',layer:'settlements',geometry:'polygon',points:[[4,2],[5,2],[5,3],[4,3]]}],features:[{...major,path:[[3,1,100],[4,3,50],[7,5,0]]}],hydrology:{reaches:[],rivers:[]}}}

function politicalFixture(){const d=fixture();d.entities.push({id:'country-shape',logicalId:'country',name:'Old name',kind:'country',layer:'countries',geometry:'polygon',points:[[2,1],[8,1],[8,5],[2,5]]},{id:'village',kind:'village',layer:'settlements',name:'Willow Hamlet',geometry:'point',points:[[6,4]]},{id:'independent',kind:'city',layer:'settlements',name:'Free Harbour',geometry:'point',points:[[9,3]]});d.logicalEntities=[{id:'country',name:'Kingdom of the Silver Coast',kind:'country'}];return d;}

test('continental illustrations show enclosing states and retain independent settlements',()=>{
 const d=politicalFixture(),a=renderMedievalMap(d);assert.match(a.svg,/data-label-for="country-shape"/);assert.match(a.svg,/Kingdom of the Silver Coast/);assert.doesNotMatch(a.svg,/data-label-for="capital"|data-label-for="village"/);assert.match(a.svg,/data-label-for="independent"/);
 d.entities.push({id:'empire',name:'The High Empire',kind:'empire',layer:'countries',geometry:'polygon',points:[[1,.5],[8.5,.5],[8.5,5.5],[1,5.5]]});const empire=renderMedievalMap(d);assert.match(empire.svg,/data-label-for="empire"/);assert.doesNotMatch(empire.svg,/data-label-for="country-shape"|data-label-for="capital"|data-label-for="village"/);
});

test('country illustrations always retain city and village names, even on broad country extents',()=>{
 const d=politicalFixture();d.historicalMap={entityId:'country',ageId:'a',parentId:'main'};const out=renderMedievalMap(d);for(const id of ['country-shape','capital','village'])assert.match(out.svg,new RegExp(`data-label-for="${id}"`));
 // Crowded names remain associated with their own settlement rather than disappearing.
 d.entities.find(e=>e.id==='village').points=[[4.5,2.5]];assert.match(renderMedievalMap(d).svg,/data-label-for="village"/);
});

test('illustration hierarchy follows saved identity and Age relationships without hiding unrelated places',()=>{
 const d=politicalFixture();d.entities.find(e=>e.id==='village').points=[[9,4]];d.relations=[{source:'village',destination:'country',kind:'part-of',ageId:'a'},{source:'independent',destination:'country',kind:'part-of',ageId:'old'}];d.historicalMap={ageId:'a'};
 const selected=illustrationEntities(d).entities.map(e=>e.id);assert.ok(!selected.includes('village'));assert.ok(selected.includes('independent'));
 d.entities.push({...d.entities.find(e=>e.id==='country-shape'),id:'country-fragment',points:[[2,1],[3,1],[3,2],[2,2]]});assert.equal((renderMedievalMap(d).svg.match(/aria-label="Kingdom of the Silver Coast"/g)||[]).length,1);
});

test('cartographic widths retain physical hierarchy without widening long low-discharge streams',()=>{
 const narrow=riverMetrics(stream,0,context),broad=riverMetrics(major,1,context);assert.equal(narrow.visible,false);assert.ok(broad.visible);assert.ok(broad.width>1);
 const long=riverMetrics(stream,0,context,{}, {lengthKm:4000});assert.ok(long.visible);assert.ok(broad.width>long.width*4);assert.equal(long.metres,narrow.metres);
 const upstream=riverMetrics(major,0,context);assert.ok(broad.width>upstream.width);
 const regional=riverMetrics({...major,class:'regional',physicalWidthMetres:60,dischargeM3s:120,widths:undefined,dischargesM3s:undefined,width:.0006},1,context);assert.ok(regional.width>long.width);assert.ok(regional.width<broad.width);
});
test('local maps expose streams and physical width, independent of channel incision',()=>{
 const local={...context,bounds:{x:10,y:10,width:1,height:1},scale:900};assert.ok(riverMetrics(stream,0,local).visible);
 const deep=riverMetrics({...major,depth:12},0,local,{incisionMetres:0});assert.equal(deep.canyon,false);
 assert.ok(riverMetrics(major,0,local,{incisionMetres:180,valleyWidthMetres:600}).canyon);
 assert.equal(riverMetrics({...major,morphology:'floodplain'},0,local,{incisionMetres:180,valleyWidthMetres:600}).canyon,false);
 assert.ok(riverMetrics(major,0,local).width>riverMetrics(major,0,context).width);
});
test('illustration is deterministic, inert escaped SVG with actual channel and settlement geometry',()=>{
 const d=fixture(),a=renderMedievalMap(d),b=renderMedievalMap(d);assert.deepEqual(a,b);assert.match(a.svg,/fill="#FFFFFF"/);assert.match(a.svg,/A &amp; B &lt;Keep&gt;/);assert.match(a.svg,/data-river="major"/);assert.match(a.svg,/data-river-system="continental"/);assert.match(a.svg,/href="#castle"/);assert.doesNotMatch(a.svg,/NaN|undefined|<image|<script/);assert.equal(a.sourceRevision,17);assert.ok(a.statistics.symbols>0);
});
test('water boundary follows land-water data, contains lake holes, no border coast invented',()=>{
 const d=fixture();d.samples=d.samples.map(()=>[0,100,0,0]);const dry=waterGeometry(d,p=>p);assert.equal(dry.water,'');assert.equal(dry.coast,'');d.samples[3*11+5][0]=1;const lake=waterGeometry(d,p=>p);assert.ok(lake.water.endsWith('Z'));assert.ok(lake.coast);d.samples=d.samples.map(()=>[1,0,0,0]);const sea=waterGeometry(d,p=>p);assert.ok(sea.water);assert.equal(sea.coast,'');
});
test('child-map extent, all objects, genuine distributaries, and local width arrays survive rendering',()=>{
 const d=fixture();d.bounds={x:3,y:1,width:4,height:4};d.features.push({...major,id:'delta-branch',widths:[.0005,.001],path:[[4,3,50],[5,4,20],[7,5,0]]});for(let i=0;i<2002;i++)d.entities.push({id:'house'+i,kind:'house',layer:'buildings',geometry:'point',points:[[4,2]]});const a=renderMedievalMap(d);assert.deepEqual(a.bounds,d.bounds);assert.equal(a.statistics.entities,2003);assert.match(a.svg,/data-river="delta-branch"/);assert.match(a.svg,/clip-path="url\(#map-extent\)"/);assert.equal(a.statistics.rivers,2);
});

test('place labels are compact, wrapped and anchored over the settlement; houses are unnamed',()=>{const d=fixture();d.entities[0].name='The Ancient Citadel of Seven Rivers';d.entities.push({id:'house',name:'Old generated house name',kind:'house',layer:'buildings',geometry:'point',points:[[6,2]]});const out=renderMedievalMap(d);assert.match(out.svg,/data-label-for="capital"/);assert.match(out.svg,/text-anchor="middle" font-size="12"/);assert.match(out.svg,/<tspan[^>]*>The Ancient Citadel<\/tspan>/);assert.doesNotMatch(out.svg,/Old generated house name/);assert.deepEqual(wrapPlaceName('White Harbour'),['White Harbour']);const anchor=featureAnchor([[0,0],[4,0],[4,1],[1,1],[1,4],[0,4]],'polygon');assert.ok(anchor[0]<=1||anchor[1]<=1);assert.deepEqual(featureAnchor([[0,0],[2,0],[2,8]],'line'),[2,3])});

test('unnamed map objects retain illustration geometry without placeholder labels',()=>{const d=fixture();d.entities[0].name='example-a81bd';d.entities[0].labelName='example-label';const out=renderMedievalMap(d);assert.equal(out.statistics.entities,1);assert.match(out.svg,/href="#castle"/);assert.doesNotMatch(out.svg,/example-a81bd|example-label|data-label-for="capital"/)});
