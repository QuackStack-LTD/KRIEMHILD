// Draw water only inside the hydrological footprint. An infinite sea plane
// incorrectly covers dry below-sea-level basins and misses elevated lakes.
export function waterSurfacePositions(fields,w,h,scale,sphere=false){
  const wet=[];for(let i=0;i<w*h;i++)if(fields.waterBody[i]>0&&fields.waterDepth[i]>0)wet.push(i);
  const positions=new Float32Array(wet.length*18);
  let at=0;
  const radius=w/(2*Math.PI);
  const vertex=(x,y,z)=>{
    if(sphere){const phi=y/h*Math.PI,theta=x/w*Math.PI*2,r=radius+z;positions[at++]=-r*Math.cos(theta)*Math.sin(phi);positions[at++]=r*Math.cos(phi);positions[at++]=r*Math.sin(theta)*Math.sin(phi)}
    else{positions[at++]=x-w/2;positions[at++]=z;positions[at++]=y-h/2}
  };
  for(const i of wet){const x=i%w,y=Math.floor(i/w),z=fields.waterLevel[i]/2000*scale+.002;
    vertex(x,y,z);vertex(x,y+1,z);vertex(x+1,y,z);
    vertex(x+1,y,z);vertex(x,y+1,z);vertex(x+1,y+1,z);
  }
  return positions;
}
