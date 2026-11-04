import test from "node:test";
import assert from "node:assert/strict";
import {resourceDistance,resourceRing,resourceQuery} from "../src/continuous/terrain-resources.js";

test("observed circles keep physical radii across the seam and near the poles",()=>{
 for(const center of [[1,.5],[1.999,.4],[1,.001],[1,.999]]){
  const ring=resourceRing(center,50,6371);
  for(const point of ring)assert.ok(Math.abs(resourceDistance(center,point,6371)-50)<1e-6);
  for(let i=1;i<ring.length;i++)assert.ok(Math.abs(ring[i][0]-ring[i-1][0])<1,"seam was not unwrapped");
 }
});
test("radius query uses geographical coordinates and preserves kilometre units",()=>{
 const q=new URLSearchParams(resourceQuery([2.001,.25],.25));
 assert.ok(Math.abs(Number(q.get("longitude"))+179.82)<1e-8);
 assert.equal(q.get("latitude"),"45");assert.equal(q.get("radiusKm"),"0.25");
 assert.equal(resourceDistance([0,.5],[2,.5],6371),0);
});
