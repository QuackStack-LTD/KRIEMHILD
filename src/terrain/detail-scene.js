import * as THREE from 'three';
import {DetailTiles,tileKey} from './detail-tiles.js';
import {makeDetailPalette,featureTexture} from './detail-render.js';
import {detailAtScale,detailName,smooth} from './map-camera.js';
import {clipWetTriangle} from './detail-water.js';

// A covering quadtree: parent quadrants disappear only after the corresponding
// child is available. Border vertices stitch to the actual parent edge.
export function createDetailScene(scene,camera,renderer,solver){
  let dirty=true,lastSignature='',lastLayer='',palette=null,disposed=false,revision=0;
  const store=new DetailTiles(solver,()=>{dirty=true;}),group=new THREE.Group(),meshes=new Map();scene.add(group);
  const W=solver.W,H=solver.H,sphere=solver.wrapX,radius=W/(2*Math.PI),ray=new THREE.Raycaster(),plane=new THREE.Plane(new THREE.Vector3(0,1,0),0),ball=new THREE.Sphere(new THREE.Vector3(),radius);
  function position(x,y,z){
    if(!sphere)return new THREE.Vector3((x/(W-1)-.5)*W,z,(y/(H-1)-.5)*H);
    const lon=x/(W-1)*Math.PI*2,lat=y/(H-1)*Math.PI,r=radius+z;
    return new THREE.Vector3(-r*Math.cos(lon)*Math.sin(lat),r*Math.cos(lat),r*Math.sin(lon)*Math.sin(lat));
  }
  function coordinates(p){
    if(!sphere)return {x:(p.x/W+.5)*(W-1),y:(p.z/H+.5)*(H-1)};
    return {x:((Math.atan2(p.z,-p.x)+Math.PI*2)%(Math.PI*2))/(Math.PI*2)*(W-1),y:Math.acos(Math.max(-1,Math.min(1,p.y/p.length())))/Math.PI*(H-1)};
  }
  function disposeMesh(m){group.remove(m);m.traverse(child=>{child.geometry?.dispose();child.material?.map?.dispose();child.material?.dispose();});}
  function update(scale,layer='terrain'){
    if(disposed)return;
    ray.setFromCamera(new THREE.Vector2(0,0),camera);
    const hit=sphere?ray.ray.intersectSphere(ball,new THREE.Vector3()):ray.ray.intersectPlane(plane,new THREE.Vector3());
    if(!hit)return;
    const center=coordinates(hit),distance=Math.max(.01,camera.position.distanceTo(hit)),height=renderer.domElement.clientHeight,width=renderer.domElement.clientWidth;
    const pixels=height/(2*distance*Math.tan(THREE.MathUtils.degToRad(camera.fov/2)));
    const detail=detailAtScale(pixels),latScale=sphere?Math.max(.12,Math.sin(center.y/(H-1)*Math.PI)):1;
    const bounds={x:center.x-width/pixels/latScale,y:center.y-height/pixels,width:2*width/pixels/latScale,height:2*height/pixels};
    let regions=bounds;
    if(sphere){
      if(bounds.width>=W-1)regions={...bounds,x:0,width:W-1};
      else if(bounds.x<0)regions=[{...bounds,x:0,width:bounds.x+bounds.width},{...bounds,x:W-1+bounds.x,width:-bounds.x}];
      else if(bounds.x+bounds.width>W-1)regions=[{...bounds,width:W-1-bounds.x},{...bounds,x:0,width:bounds.x+bounds.width-(W-1)}];
    }
    const signature=[Math.round(bounds.x*16),Math.round(bounds.y*16),Math.round(bounds.width*16),Math.round(bounds.height*16),Math.round(detail*64),scale,layer].join('/');
    if(signature!==lastSignature){store.request(regions,pixels);dirty=true;lastSignature=signature;}
    if(!dirty)return;dirty=false;
    store.request(regions,pixels);
    if(layer!==lastLayer||!palette){palette=makeDetailPalette(solver,layer);lastLayer=layer;}
    const active=new Map();
    for(const [key,tile]of store.cache){
      if(tile.level===0){active.set(key,tile);continue;}
      if(!store.wanted?.has(key)||tile.level>Math.ceil(detail))continue;
      let l=tile.level,x=tile.x,y=tile.y,valid=true;while(l>0){l--;x=Math.floor(x/2);y=Math.floor(y/2);if(!store.cache.has(tileKey(l,x,y)))valid=false;}
      if(valid)active.set(key,tile);
    }
    for(const [key,m]of meshes)if(!active.has(key)){disposeMesh(m);meshes.delete(key);}
    for(const [key,tile]of active){
      const arrival=smooth((performance.now()-tile.loaded)/180),blend=tile.level===0?1:smooth(detail-tile.level+1)*arrival;
      if(arrival<1)dirty=true;
      const children=[0,1,2,3].map(q=>active.has(tileKey(tile.level+1,tile.x*2+q%2,tile.y*2+Math.floor(q/2))));
      const neighborTiles=[[-1,0],[1,0],[0,-1],[0,1]].map(([x,y])=>active.get(tileKey(tile.level,tile.x+x,tile.y+y)));
      const neighbors=neighborTiles.map(Boolean);
      const edgeBlend=neighborTiles.map(t=>t?Math.min(blend,smooth(detail-t.level+1)*smooth((performance.now()-t.loaded)/180)):0);
      const stamp=[revision,tile.loaded,Math.round(detail*64),Math.round(blend*64),children.join(),neighbors.join(),edgeBlend.map(v=>Math.round(v*64)).join(),scale,layer].join('/');
      const previous=meshes.get(key);if(previous?.userData.stamp===stamp)continue;if(previous)disposeMesh(previous);
      const n=tile.size,verts=[],uv=[],indices=[],water=[];
      const f=solver.environment.fields;
      const north=f.elevation.slice(0,W).reduce((a,b)=>a+b,0)/W,south=f.elevation.slice((H-1)*W).reduce((a,b)=>a+b,0)/W;
      const elevation=(x,y)=>{const p=tile.points[y*n+x];return p.parent+(p.elevation-p.parent)*blend;};
      for(let y=0;y<n;y++)for(let x=0;x<n;x++){
        let z=elevation(x,y);
        if(tile.level>0){let edge=blend;if(x===0&&neighbors[0])edge=Math.min(edge,edgeBlend[0]);if(x===32&&neighbors[1])edge=Math.min(edge,edgeBlend[1]);if(y===0&&neighbors[2])edge=Math.min(edge,edgeBlend[2]);if(y===32&&neighbors[3])edge=Math.min(edge,edgeBlend[3]);const p=tile.points[y*n+x];z=p.parent+(p.elevation-p.parent)*edge;}
        if(tile.level>0){
          if((x===0&&!neighbors[0]||x===32&&!neighbors[1])&&y%2)z=(elevation(x,y-1)+elevation(x,y+1))/2;
          if((y===0&&!neighbors[2]||y===32&&!neighbors[3])&&x%2)z=(elevation(x-1,y)+elevation(x+1,y))/2;
        }
        const wx=Math.min(W-1,(tile.x*32+x)*tile.step),wy=Math.min(H-1,(tile.y*32+y)*tile.step);
        if(sphere){if(wy===0)z=north;else if(wy===H-1)z=south;else if(wx===0||wx===W-1){const row=Math.floor(wy),t=wy-row;z=f.elevation[row*W]*(1-t)+f.elevation[Math.min(H-1,row+1)*W]*t;}}
        const p=position(wx,wy,z/2000*scale);
        if(sphere&&tile.level>0){
          const x0=Math.floor(x/2)*2,y0=Math.floor(y/2)*2,a=(x-x0)/2,b=(y-y0)/2;
          const corners=a+b<=1?[[x0,y0,1-a-b],[x0+2,y0,a],[x0,y0+2,b]]:[[x0+2,y0+2,a+b-1],[x0+2,y0,1-b],[x0,y0+2,1-a]];
          const parent=new THREE.Vector3();
          for(const [cx,cy,weight]of corners){if(weight===0)continue;const ix=Math.min(32,cx),iy=Math.min(32,cy),px=Math.min(W-1,(tile.x*32+ix)*tile.step),py=Math.min(H-1,(tile.y*32+iy)*tile.step);let z=tile.points[iy*n+ix].elevation;
            if(py===0)z=north;else if(py===H-1)z=south;else if(px===0||px===W-1){const row=Math.floor(py),t=py-row;z=f.elevation[row*W]*(1-t)+f.elevation[Math.min(H-1,row+1)*W]*t;}
            parent.addScaledVector(position(px,py,z/2000*scale),weight);
          }
          // Interpolate Cartesian parent triangles as well as elevation: a
          // newly subdivided sphere must not pop outward from a coarse chord.
          let amount=blend;if(x===0)amount=Math.min(amount,edgeBlend[0]);if(x===32)amount=Math.min(amount,edgeBlend[1]);if(y===0)amount=Math.min(amount,edgeBlend[2]);if(y===32)amount=Math.min(amount,edgeBlend[3]);
          const fine=position(wx,wy,(tile.points[y*n+x].elevation)/2000*scale);
          if(wy===0||wy===H-1||wx===0||wx===W-1)fine.copy(p);
          p.copy(parent).lerp(fine,amount);
        }
        verts.push(p.x,p.y,p.z);uv.push(x/32,1-y/32);
      }
      for(let y=0;y<32;y++)for(let x=0;x<32;x++){
        if(children[(x>=16?1:0)+(y>=16?2:0)])continue;
        const a=y*n+x,b=a+1,c=a+n,d=c+1;indices.push(a,c,b,b,c,d);
        for(const triangle of [[a,c,b],[b,c,d]]){
          const wet=triangle.map(i=>tile.points[i]).find(p=>p.waterBody>0)||(tile.level>=3?triangle.map(i=>tile.points[i]).find(p=>p.riverDepth>0):null);if(!wet)continue;
          const river=!wet.waterBody;
          const polygon=clipWetTriangle(triangle.map(i=>{
            const p=tile.points[i],z=p.parent+(p.elevation-p.parent)*blend;
            if(river){const level=p.riverLevel??wet.riverLevel;return {x:Math.min(W-1,(tile.x*32+i%n)*tile.step),y:Math.min(H-1,(tile.y*32+Math.floor(i/n))*tile.step),level,d:p.riverDepth>0?z-level:Math.max(.00001,z-level)};}
            return {x:Math.min(W-1,(tile.x*32+i%n)*tile.step),y:Math.min(H-1,(tile.y*32+Math.floor(i/n))*tile.step),d:p.waterBody===wet.waterBody?Math.min(-.00001,z-wet.waterLevel):Math.max(.00001,z-wet.waterLevel)};
          }));
          for(let k=1;k<polygon.length-1;k++)for(const v of [polygon[0],polygon[k],polygon[k+1]]){const p=position(v.x,v.y,(river?v.level:wet.waterLevel)/2000*scale+.0005);water.push(p.x,p.y,p.z);}
        }
      }
      const geo=new THREE.BufferGeometry();geo.setAttribute('position',new THREE.Float32BufferAttribute(verts,3));geo.setAttribute('uv',new THREE.Float32BufferAttribute(uv,2));geo.setIndex(indices);geo.computeVertexNormals();
      const texture=new THREE.CanvasTexture(featureTexture(tile,solver,palette,layer,blend,detail));texture.colorSpace=THREE.SRGBColorSpace;
      const mesh=new THREE.Mesh(geo,new THREE.MeshStandardMaterial({map:texture,roughness:.95,side:THREE.DoubleSide}));mesh.userData.stamp=stamp;group.add(mesh);meshes.set(key,mesh);
      if(water.length){const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(water,3));geometry.computeVertexNormals();mesh.add(new THREE.Mesh(geometry,new THREE.MeshStandardMaterial({color:0x2a6fc9,transparent:true,opacity:.22,roughness:.3,side:THREE.DoubleSide,depthWrite:false})));}
    }
    const status=document.getElementById('detailStatus');if(status)status.textContent=`${detailName(detail)} · 3D${store.pending.size?' · refining…':''}${store.error?' · '+store.error:''}`;
    renderer.domElement.dataset.detail=String(detail);renderer.domElement.dataset.tiles=String(active.size);
  }
  return {update,refresh(){revision++;dirty=true;palette=null;lastSignature='';},meshes:()=>[...meshes.values()].flatMap(m=>[m,...m.children]),dispose(){disposed=true;store.dispose();for(const m of meshes.values())disposeMesh(m);meshes.clear();scene.remove(group);}};
}
