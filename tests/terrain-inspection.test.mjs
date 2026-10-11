import {test} from 'node:test';
import assert from 'node:assert/strict';
import {biomeName,biomeColor,naturalMaterials} from '../src/terrain/biome-material.js';
import {geographicLocation,formatMapLocation,terrainInspection,renderedRiverAt,sampleTerrainTile} from '../src/terrain/terrain-inspection.js';

const solver=()=>({W:360,H:180,config:{types:[{name:'Old incorrect desert'}]},typeAt:()=>0,environment:{options:{seed:'inspection'},geography:{planetary_radius:6371,latitude_north:90,latitude_south:-90,longitude_west:-180,longitude_east:180},fields:{}}});
test('Readout uses the renderer material instead of the old terrain cell label',()=>{
 const s=solver(),point={x:100,y:50};
 for(const sample of [{vegetation:.95,moisture:.9,temperature:20},{rock:.9,temperature:15},{sand:.9,temperature:25},{wetland:1,moisture:1,temperature:20}]){
  const inspected=terrainInspection(s,point,sample);
  assert.notEqual(inspected.terrain,'Old incorrect desert');
  assert.deepEqual(biomeColor(sample,undefined,{...point,seed:0}),naturalMaterials[biomeName(sample,{...point,seed:0})]);
 }
 assert.equal(terrainInspection(s,point,{waterBody:1}).terrain,'Ocean / sea');
 assert.equal(terrainInspection(s,point,{waterBody:2}).terrain,'Lake');
 assert.equal(terrainInspection(s,point,{riverVisible:true}).terrain,'River');
 s.pinned={[50*360+100]:true};assert.equal(terrainInspection(s,point,{rock:1}).terrain,'Old incorrect desert');
});
test('Coordinates use spherical geographic scale and cell centres',()=>{
 const s=solver(),equator=geographicLocation(s,{x:179.5,y:89.5}),north=geographicLocation(s,{x:179.5,y:29.5});
 assert.equal(equator.latitude,0);assert.equal(equator.longitude,0);
 assert.ok(Math.abs(equator.xKm-Math.PI*6371)<1e-8);
 assert.ok(Math.abs(north.xKm/equator.xKm-.5)<1e-8);
 assert.match(formatMapLocation(s,{x:179.5,y:89.5}),/20015.1 km/);
 s.environment=null;assert.match(formatMapLocation(s,{x:1,y:2}),/unscaled/);
});
test('River readout follows visible geometry and zoom-dependent channel width',()=>{
 const feature={id:'river',kind:'river',level:0,path:[[1,1],[5,1]],width:.1,widths:[.001,.2],physicalWidthMetres:50};
 assert.equal(renderedRiverAt([feature],{x:4,y:1},100,2),true);
 assert.equal(renderedRiverAt([feature],{x:4,y:2},100,2),false);
 assert.equal(renderedRiverAt([{...feature,level:5}],{x:4,y:1},100,2),false);
});

test('Refined inspection interpolates physical fields and preserves dry depressions',()=>{
 const tile={x:0,y:0,step:1,level:1,points:Array.from({length:33*33},(_,i)=>({parent:100,elevation:100+(i%33)*10,rock:.8,temperature:12,waterBody:0}))};
 const sample=sampleTerrainTile(tile,{x:2.5,y:3.5},.5);
 assert.equal(sample.elevation,112.5);assert.equal(sample.rock,.8);assert.equal(sample.waterBody,0);
 assert.equal(terrainInspection(solver(),{x:2.5,y:3.5},sample).terrain,'Mountain terrain');
 tile.points.forEach(p=>{p.parent=-30;p.elevation=-30;});
 assert.equal(sampleTerrainTile(tile,{x:2.5,y:3.5},1).waterDepth,0);
 tile.points.forEach(p=>{p.waterBody=2;p.waterLevel=20;});
 const lake=sampleTerrainTile(tile,{x:2.5,y:3.5},1);
 assert.equal(lake.waterDepth,50);assert.equal(terrainInspection(solver(),{x:2.5,y:3.5},lake).terrain,'Lake');
});
