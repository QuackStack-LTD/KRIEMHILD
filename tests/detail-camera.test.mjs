import {test} from 'node:test';
import assert from 'node:assert/strict';
import {MapCamera,detailAtScale,smooth} from '../src/terrain/map-camera.js';
import {visibleTiles} from '../src/terrain/detail-tiles.js';

test('Cursor geographic position is invariant through repeated zoom and pan',()=>{
  const camera=new MapCamera();camera.fit(1100,700,159,99);
  for(let i=0;i<30;i++){
    camera.x+=13;camera.y-=7;const x=31+i*23,y=41+i*11,before=camera.point(x,y);
    camera.zoomAt(x,y,1.25);const after=camera.point(x,y);assert.ok(Math.abs(before.x-after.x)<1e-10);assert.ok(Math.abs(before.y-after.y)<1e-10);
    camera.zoomAt(x,y,.8);const back=camera.point(x,y);assert.ok(Math.abs(before.x-back.x)<1e-10);assert.ok(Math.abs(before.y-back.y)<1e-10);
  }
});
test('Detail depends on scale, with continuous octave weights and bounded visible work',()=>{
  for(let k=1;k<8;k++){
    const a=detailAtScale(6*2**k*(1-1e-7)),b=detailAtScale(6*2**k*(1+1e-7));assert.ok(Math.abs(a-b)<1e-5);assert.ok(smooth(a-(k-1))>.99999);assert.ok(smooth(b-k)<1e-10);
  }
  for(let level=0;level<=8;level++){
    const scale=6*2**level,bounds={x:10,y:10,width:1100/scale,height:700/scale};
    const tiles=visibleTiles(bounds,level,159,99);assert.ok(tiles.length<=35);assert.deepEqual(tiles,visibleTiles({...bounds},level,159,99));
  }
});


test('Detail water clips shore triangles and excludes dry depressions',async()=>{
  const {clipWetTriangle}=await import('../src/terrain/detail-water.js');
  assert.deepEqual(clipWetTriangle([{x:0,y:0,d:1},{x:1,y:0,d:2},{x:0,y:1,d:3}]),[]);
  const vertices=clipWetTriangle([{x:0,y:0,d:-10},{x:1,y:0,d:10},{x:0,y:1,d:10}]);
  assert.equal(vertices.length,3);assert.ok(vertices.some(p=>p.x===.5&&p.y===0));assert.ok(vertices.some(p=>p.x===0&&p.y===.5));
  assert.ok(vertices.every(p=>Number.isFinite(p.x)&&Number.isFinite(p.y)));
});

test('River surfaces interpolate downhill water levels at clipped banks',async()=>{
  const {clipWetTriangle}=await import('../src/terrain/detail-water.js');
  const polygon=clipWetTriangle([{x:0,y:0,d:-4,level:100},{x:1,y:0,d:4,level:80},{x:0,y:1,d:4,level:100}]);
  assert.equal(polygon.find(p=>p.x===.5).level,90);
  assert.ok(polygon.every(p=>p.level>=80&&p.level<=100));
});

test('Detail features draw widening river ribbons and never placeholder dots',async()=>{
  const {drawDetailFeatures}=await import('../src/terrain/detail-render.js');
  const polygons=[];let path=[];
  const ctx={save(){},restore(){},beginPath(){path=[];},moveTo(x,y){path.push([x,y]);},lineTo(x,y){path.push([x,y]);},closePath(){},fill(){polygons.push(path);}};
  const features=[{id:'river',kind:'river',level:0,width:.02,widths:[.02,.08,.14],discharge:20,path:[[0,0,100],[1,0,90],[2,0,80]]},{id:'old-dot',kind:'rock',level:0,width:1,path:[[1,1,100]]}];
  drawDetailFeatures(ctx,[{tile:{features},arrival:1}],{x:0,y:0,scale:100},5,'terrain');
  assert.equal(polygons.length,3);
  const water=polygons[1];assert.ok(Math.abs(water[2][1])>Math.abs(water[0][1])*5);
});


test('Authored and generated river identities render identical water and banks at every scale',async()=>{
 const {drawDetailFeatures}=await import('../src/terrain/detail-render.js');
 const render=(id,detail)=>{const calls=[];let points=[];const ctx={save(){},restore(){},beginPath(){points=[];},moveTo(...p){points.push(p);},lineTo(...p){points.push(p);},closePath(){},fill(){calls.push({color:this.fillStyle,alpha:this.globalAlpha,points});}};
 const feature={id,kind:'river',level:1,width:.1,widths:[.1,.12,.17],discharge:35,path:[[0,0,300],[1,.2,200],[2,0,100]]};
 drawDetailFeatures(ctx,[{tile:{features:[feature]},arrival:1}],{x:12,y:10,scale:6*2**detail},detail,'terrain');return calls;};
 for(const detail of [0,1,2.5,4,6,8])assert.deepEqual(render('generated/drainage/7',detail),render('authored/river-a',detail));
 const {entityColor,entityStyle}=await import('../src/terrain/feature-style.js');
 assert.equal(entityColor({}),'#000000');assert.equal(entityColor({color:'#ab1234'}),'#ab1234');assert.equal(entityColor({},true),entityStyle.selected);
});


test('Terrain materials use physical fields identically for generated and edited samples',async()=>{
 const {tileImage}=await import('../src/terrain/detail-render.js');const previous=globalThis.document;
 globalThis.document={createElement(){const canvas={};canvas.getContext=()=>({createImageData:(w,h)=>({data:new Uint8ClampedArray(w*h*4)}),putImageData:image=>{canvas.pixels=[...image.data];}});return canvas;}};
 try{
  const solver={W:2,H:2,environment:{fields:{waterBody:[0,0,0,0]}}},palette=Array.from({length:4},()=>[90,140,80]);
  for(const level of [0,3,7])for(const material of [{vegetation:.9},{rock:.9},{sand:.9},{floodplain:1,riverDepth:3},{waterBody:1,waterDepth:40}]){
   const point={elevation:120,parent:110,temperature:15,waterBody:0,vegetation:0,gradient:[2,3,2,3],...material};
   const tile={level,x:0,y:0,step:1,size:2,points:Array.from({length:4},()=>point)};
   const natural=tileImage(tile,solver,palette,'terrain',.5).pixels;
   const authored=tileImage({...tile,points:tile.points.map(p=>({...p,authored:true}))},solver,palette,'terrain',.5).pixels;
   assert.deepEqual(authored,natural);
  }
 }finally{if(previous===undefined)delete globalThis.document;else globalThis.document=previous;}
});
