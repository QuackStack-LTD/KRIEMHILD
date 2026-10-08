// Clip a terrain triangle at its local water surface. Coordinates and signed
// distances are interpolated together; dry below-sea-level vertices stay dry.
export function clipWetTriangle(vertices){
  const out=[];
  for(let i=0;i<vertices.length;i++){
    const a=vertices[i],b=vertices[(i+1)%vertices.length],inside=a.d<0,next=b.d<0;
    if(inside)out.push(a);
    if(inside!==next){const t=a.d/(a.d-b.d),p={x:a.x+(b.x-a.x)*t,y:a.y+(b.y-a.y)*t,d:0};if(a.level!==undefined&&b.level!==undefined)p.level=a.level+(b.level-a.level)*t;out.push(p);}
  }
  return out;
}
