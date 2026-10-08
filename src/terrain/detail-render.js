import {fieldColor} from './environment-view.js';
import {smooth} from './map-camera.js';

const clamp=(x,a,b)=>Math.max(a,Math.min(b,x));
export function makeDetailPalette(solver,layer='terrain'){
  const ctx=document.createElement('canvas').getContext('2d',{willReadFrequently:true}),cache=new Map();
  return Array.from({length:solver.N},(_,i)=>{
    const type=solver.config.types[solver.typeAt(i)];const css=layer==='terrain'?(type?.color||'#7c9864'):(fieldColor(solver.environment,layer,i)||'#637b83');
    if(cache.has(css))return cache.get(css);ctx.fillStyle=css;ctx.fillRect(0,0,1,1);const rgb=[...ctx.getImageData(0,0,1,1).data].slice(0,3);cache.set(css,rgb);return rgb;
  });
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
        // Material follows the sampled physical surface, not random colour spots.
        const mix=(color,amount)=>{rgb=rgb.map((c,k)=>c+(color[k]-c)*clamp(amount,0,1));};
        const parentMaterial=smooth((tile.level-2)/3),local=parentMaterial+(smooth((tile.level-1)/3)-parentMaterial)*blend;
        const vegetation=p.vegetation??0;
        if(p.temperature>0)mix([132-68*vegetation,122-18*vegetation,83-34*vegetation],local*.5);
        mix([190,169,120],(p.sand||0)*local*.8);
        mix([110,107,95],(p.rock||0)*local*.65);
        mix([80,108,67],(p.floodplain||0)*local*.45);
        if(p.riverDepth>0)mix([53,115,139],local);
      }
      let dx=(height(j*n+Math.min(n-1,i+1))-height(j*n+Math.max(0,i-1)))/(tile.step*2);
      let dy=(height(Math.min(n-1,j+1)*n+i)-height(Math.max(0,j-1)*n+i))/(tile.step*2);
      if(p.gradient){dx=p.gradient[2]+(p.gradient[0]-p.gradient[2])*blend;dy=p.gradient[3]+(p.gradient[1]-p.gradient[3])*blend;}
      // A bounded normal response keeps deep relief readable without clipping
      // every ridge to black/white at local scales.
      const normal=Math.sqrt(1+(dx*dx+dy*dy)/1600000);
      const shade=clamp(.55+(.65+(dx-dy)/2400)/normal,.42,1.24);
      rgb=rgb.map(c=>c*shade);
    }else if(layer==='elevation'||layer==='bathymetry'||layer==='waterDepth'){
      const value=layer==='elevation'?(height(at)+12000)/20000:p.waterDepth/12000,t=clamp(value,0,1);
      rgb=[40+200*t,80+100*(1-Math.abs(t-.5)*2),200-160*t];
    }
    const index=at*4;for(let k=0;k<3;k++)image.data[index+k]=rgb[k];image.data[index+3]=255;
  }
  ctx.putImageData(image,0,0);return canvas;
}

export function drawDetailFeatures(ctx,tiles,camera,detail,layer){
  if(!['terrain','river','accumulation'].includes(layer))return;
  const seen=new Set();ctx.save();
  for(const {tile,arrival} of tiles)for(const f of tile.features){
    if(seen.has(f.id))continue;seen.add(f.id);
    const alpha=smooth(detail-f.level+1)*arrival;if(alpha<=0)continue;
    if(f.kind!=='river'||f.path.length<2)continue;
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
    if(detail>2&&f.discharge>=2){ctx.globalAlpha=alpha*smooth(detail-2)*.28;ribbon(1.35,'#a1a183',0);}
    ctx.globalAlpha=alpha;ribbon(1,'#397f9e',Math.min(1.6,.8+Math.sqrt(f.discharge||0)*.03));
    if(detail>4){ctx.globalAlpha=alpha*.2;ribbon(.55,'#72b6c5',0);}
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
