import {biomeColor,coarseMaterial} from './biome-material.js';
import {riverStyle} from './feature-style.js';
import {fieldColor,riverSystemColor} from './environment-view.js';
import {smooth} from './map-camera.js';

const clamp=(x,a,b)=>Math.max(a,Math.min(b,x));
export function makeDetailPalette(solver,layer='terrain'){
  const ctx=document.createElement('canvas').getContext('2d',{willReadFrequently:true}),cache=new Map();
  const materials={};
  for(const type of solver.config.types){const key=type.environmentType||type.id;if(!['grass','forest','desert','mountain','tundra','snow','sand','swamp'].includes(key))continue;ctx.fillStyle=type.color;ctx.fillRect(0,0,1,1);materials[key]=[...ctx.getImageData(0,0,1,1).data].slice(0,3);}
  const palette=Array.from({length:solver.N},(_,i)=>{
    if(layer==='terrain'&&!solver.pinned?.[i]&&solver.environment?.fields.waterBody[i]===0)return biomeColor(coarseMaterial(solver.environment,i),materials);
    const type=solver.config.types[solver.typeAt(i)];const css=layer==='terrain'?(type?.color||'#7c9864'):(fieldColor(solver.environment,layer,i)||'#637b83');
    if(cache.has(css))return cache.get(css);ctx.fillStyle=css;ctx.fillRect(0,0,1,1);const rgb=[...ctx.getImageData(0,0,1,1).data].slice(0,3);cache.set(css,rgb);return rgb;
  });
  palette.materials=materials;return palette;
}

function baseColor(solver,palette,x,y,wet){
  const w=solver.W,h=solver.H;x=clamp(x,0,w-1);y=clamp(y,0,h-1);const x0=Math.floor(x),y0=Math.floor(y),a=x-x0,b=y-y0;
  const rgb=[0,0,0];let total=0;
  for(const [dx,dy,weight] of [[0,0,(1-a)*(1-b)],[1,0,a*(1-b)],[0,1,(1-a)*b],[1,1,a*b]]){
    const i=Math.min(h-1,y0+dy)*w+Math.min(w-1,x0+dx);
    if(wet!==null&&(solver.environment.fields.waterBody[i]>0)!==wet)continue;
    total+=weight;for(let k=0;k<3;k++)rgb[k]+=palette[i][k]*weight;
  }
  return total>0?rgb.map(c=>c/total):palette[Math.round(y)*w+Math.round(x)];
}

export function tileImage(tile,solver,palette,layer,blend=1){
  const canvas=document.createElement('canvas');canvas.width=canvas.height=tile.size;
  const ctx=canvas.getContext('2d'),image=ctx.createImageData(tile.size,tile.size),n=tile.size;
  const height=i=>tile.points[i].parent+(tile.points[i].elevation-tile.points[i].parent)*blend;
  for(let j=0;j<n;j++)for(let i=0;i<n;i++){
    const at=j*n+i,p=tile.points[at],x=(tile.x*32+i)*tile.step,y=(tile.y*32+j)*tile.step;
    let rgb=baseColor(solver,palette,x,y,layer==='terrain'?p.waterBody>0:null);
    if(layer==='terrain'){
      if(p.waterBody>0){const deep=clamp(Math.log1p(p.waterDepth)/9,0,1);rgb=[45-25*deep,140-96*deep,175-91*deep];}
      else {
        const physical=biomeColor(p,palette.materials);
        // The same material function is used at every scale. Parent blending
        // controls geometry; there is no hard biome-dependent height reset.
        rgb=physical;
        // Explicit terrain painting stays an override, feathered against the
        // surrounding physical materials instead of becoming a polygon edge.
        if(solver.pinned){const x0=Math.floor(x),y0=Math.floor(y),a=x-x0,b=y-y0;let weight=0,paint=[0,0,0];for(const [dx,dy,t]of [[0,0,(1-a)*(1-b)],[1,0,a*(1-b)],[0,1,(1-a)*b],[1,1,a*b]]){const cell=Math.max(0,Math.min(solver.H-1,y0+dy))*solver.W+Math.max(0,Math.min(solver.W-1,x0+dx));if(!solver.pinned[cell])continue;weight+=t;for(let k=0;k<3;k++)paint[k]+=palette[cell][k]*t;}if(weight>0)rgb=rgb.map((c,k)=>c*(1-weight)+paint[k]);}
        if(p.riverDepth>0)rgb=[57,127,158];
      }
      let dx=(height(j*n+Math.min(n-1,i+1))-height(j*n+Math.max(0,i-1)))/(tile.step*2);
      let dy=(height(Math.min(n-1,j+1)*n+i)-height(Math.max(0,j-1)*n+i))/(tile.step*2);
      if(p.gradient){dx=p.gradient[2]+(p.gradient[0]-p.gradient[2])*blend;dy=p.gradient[3]+(p.gradient[1]-p.gradient[3])*blend;}
      // A bounded normal response keeps deep relief readable without clipping
      // every ridge to black/white at local scales.
      const normal=Math.sqrt(1+(dx*dx+dy*dy)/1600000);
      const shade=clamp(.55+(.65+(dx-dy)/2400)/normal,.42,1.24);
      rgb=rgb.map(c=>c*shade);
    }else if(layer==='riverClass'||layer==='riverSystem'){
      // Draw diagnostic channels as connected geometry over a quiet terrain
      // silhouette, not as enlarged colored raster cells.
      const high=clamp(p.elevation/4000,0,1)*20;
      rgb=p.waterBody>0?[19,43,58]:[34+high,49+high,43+high];
    }else if(layer==='elevation'||layer==='bathymetry'||layer==='waterDepth'){
      const value=layer==='elevation'?(height(at)+12000)/20000:p.waterDepth/12000,t=clamp(value,0,1);
      rgb=[40+200*t,80+100*(1-Math.abs(t-.5)*2),200-160*t];
    }
    const index=at*4;for(let k=0;k<3;k++)image.data[index+k]=rgb[k];image.data[index+3]=255;
  }
  ctx.putImageData(image,0,0);return canvas;
}

export function drawDetailFeatures(ctx,tiles,camera,detail,layer){
  if(!['terrain','river','accumulation','watershed','riverClass','riverSystem'].includes(layer))return;
  const debug=['riverClass','riverSystem'].includes(layer);
  const seen=new Set();ctx.save();
  for(const {tile,arrival} of tiles)for(const f of tile.features){
    if(seen.has(f.id))continue;seen.add(f.id);
    const alpha=(debug?1:smooth(detail-f.level+1))*arrival;if(alpha<=0)continue;
    if(!['river','canal'].includes(f.kind)||f.path.length<2)continue;
    ctx.globalAlpha=alpha;const path=f.path.map(p=>[camera.x+p[0]*camera.scale,camera.y+p[1]*camera.scale]);
    // Variable-width ribbons meet at shared hydraulic junction widths. Banks
    // become readable at regional scale; tiny streams remain surface lines.
    const ribbon=(factor,color,minWidth)=>{
      const left=[],right=[];
      for(let i=0;i<path.length;i++){
        const a=path[Math.max(0,i-1)],b=path[Math.min(path.length-1,i+1)],len=Math.hypot(b[0]-a[0],b[1]-a[1])||1;
        const radius=Math.max(minWidth,(f.widths?.[i]??f.width)*camera.scale*factor)/2;
        const nx=-(b[1]-a[1])/len*radius,ny=(b[0]-a[0])/len*radius;
        left.push([path[i][0]+nx,path[i][1]+ny]);right.push([path[i][0]-nx,path[i][1]-ny]);
      }
      ctx.fillStyle=color;ctx.beginPath();[...left,...right.reverse()].forEach((p,i)=>i?ctx.lineTo(...p):ctx.moveTo(...p));ctx.closePath();ctx.fill();
    };
    if(debug){
      let color=f.class==='major'?'#ffcd78':f.class==='regional'?'#64d4ad':'#609bc1';
      if(layer==='riverSystem')color=riverSystemColor(f.riverId||f.id);
      ribbon(1,color,f.class==='major'?2.4:1.2);
      const a=path[0],b=path[path.length-1],length=Math.hypot(b[0]-a[0],b[1]-a[1]);
      if(length>14){const k=Math.floor(path.length*.65),p=path[k],q=path[Math.max(0,k-2)],angle=Math.atan2(p[1]-q[1],p[0]-q[0]);ctx.fillStyle=color;ctx.beginPath();ctx.moveTo(...p);ctx.lineTo(p[0]-Math.cos(angle-.5)*5,p[1]-Math.sin(angle-.5)*5);ctx.lineTo(p[0]-Math.cos(angle+.5)*5,p[1]-Math.sin(angle+.5)*5);ctx.closePath();ctx.fill();}
      continue;
    }
    if(detail>2&&f.discharge>=2){ctx.globalAlpha=alpha*smooth(detail-2)*.28;ribbon(1.35,riverStyle.bank,0);}
    ctx.globalAlpha=alpha*(f.regime==='seasonal'?.75:1);ribbon(1,f.regime==='ephemeral'?'#978569':riverStyle.water,Math.min(1.6,.8+Math.sqrt(f.discharge||0)*.03));
    if(detail>4){ctx.globalAlpha=alpha*.2;ribbon(.55,riverStyle.highlight,0);}
  }
  ctx.restore();
}

export function featureTexture(tile,solver,palette,layer,blend,detail){
  const base=tileImage(tile,solver,palette,layer,blend),canvas=document.createElement('canvas');canvas.width=canvas.height=256;
  const ctx=canvas.getContext('2d');ctx.drawImage(base,.5,.5,32,32,0,0,256,256);
  const span=32*tile.step;
  drawDetailFeatures(ctx,[{tile,arrival:1}],{x:-tile.x*256,y:-tile.y*256,scale:256/span},detail,layer);
  return canvas;
}

export function drawDetailOverlay(ctx,solver,camera,layer,width,height){
  const f=solver.environment.fields;
  ctx.save();
  if(layer==='wind'){
    const step=Math.max(1,Math.ceil(32/camera.scale));ctx.strokeStyle='#e7f1f5';ctx.lineWidth=1;
    const minX=Math.max(0,Math.floor(-camera.x/camera.scale/step)*step),minY=Math.max(0,Math.floor(-camera.y/camera.scale/step)*step);
    for(let y=minY;y<solver.H&&camera.y+y*camera.scale<height;y+=step)for(let x=minX;x<solver.W&&camera.x+x*camera.scale<width;x+=step){
      const i=y*solver.W+x,px=camera.x+x*camera.scale,py=camera.y+y*camera.scale,a=Math.atan2(f.windY[i],f.windX[i]),length=Math.min(22,camera.scale*step*.7),ex=px+Math.cos(a)*length,ey=py+Math.sin(a)*length;
      ctx.beginPath();ctx.moveTo(px,py);ctx.lineTo(ex,ey);ctx.lineTo(ex-Math.cos(a-.6)*4,ey-Math.sin(a-.6)*4);ctx.moveTo(ex,ey);ctx.lineTo(ex-Math.cos(a+.6)*4,ey-Math.sin(a+.6)*4);ctx.stroke();
    }
  }
  if(document.getElementById('contShow')?.checked)for(const p of solver.contPoints||[]){const x=camera.x+p.x*camera.scale,y=camera.y+p.y*camera.scale;ctx.fillStyle=p.kind?.color||'#e3b978';ctx.beginPath();ctx.arc(x,y,5,0,Math.PI*2);ctx.fill();}
  ctx.restore();
}
