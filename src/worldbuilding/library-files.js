import {unzipSync,zipSync,strToU8,strFromU8} from 'fflate';
export const extension=name=>name.split('.').pop().toLowerCase();
export function editorKind(name){const ext=extension(name);return ['txt','md','markdown','json','xml','yaml','yml','html','css','js','log','tex'].includes(ext)?'text':ext==='docx'?'docx':['xls','xlsx','ods','csv'].includes(ext)?'sheet':ext==='pdf'?'pdf':'binary'}
const ns='http://schemas.openxmlformats.org/wordprocessingml/2006/main';
const escape=s=>s.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;');
export function newDocx(text=''){
 return zipSync(Object.fromEntries(Object.entries({
 '[Content_Types].xml':'<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>',
 '_rels/.rels':'<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>',
 'word/document.xml':`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="${ns}"><w:body>${text.split('\n').map(t=>`<w:p><w:r><w:t xml:space="preserve">${escape(t)}</w:t></w:r></w:p>`).join('')}<w:sectPr/></w:body></w:document>`,
 }).map(([k,v])=>[k,strToU8(v)])));
}
export function openDocx(bytes){
 let expanded=0;const files=unzipSync(bytes,{filter:f=>{expanded+=f.originalSize;if(expanded>128*1024*1024)throw Error('Expanded document exceeds 128 MiB. Download to edit externally.');return true}});
 if(!files['word/document.xml'])throw Error('Invalid DOCX document');
 const xml=new DOMParser().parseFromString(strFromU8(files['word/document.xml']),'application/xml');
 if(xml.querySelector('parsererror'))throw Error('Invalid document XML');
 const nodes=[...xml.getElementsByTagNameNS(ns,'p')];
 const paragraphs=nodes.map(p=>[...p.getElementsByTagNameNS(ns,'t')].map(t=>t.textContent).join(''));
 return {files,xml,nodes,paragraphs};
}
export function writeDocx(model,paragraphs){
 // Work on a fresh XML tree so failed saves do not mutate the editor's baseline.
 const xml=model.xml.cloneNode(true),nodes=[...xml.getElementsByTagNameNS(ns,'p')];
 paragraphs.forEach((value,i)=>{if(value===model.paragraphs[i])return;let p=nodes[i];if(!p){p=xml.createElementNS(ns,'w:p');const body=xml.getElementsByTagNameNS(ns,'body')[0];body.insertBefore(p,body.getElementsByTagNameNS(ns,'sectPr')[0]||null)}
  const texts=[...p.getElementsByTagNameNS(ns,'t')];if(texts.length){texts[0].textContent=value;texts[0].setAttributeNS('http://www.w3.org/XML/1998/namespace','xml:space','preserve');texts.slice(1).forEach(t=>t.textContent='')}else{const r=xml.createElementNS(ns,'w:r'),t=xml.createElementNS(ns,'w:t');t.textContent=value;t.setAttributeNS('http://www.w3.org/XML/1998/namespace','xml:space','preserve');r.append(t);p.append(r)}
 });
 return zipSync({...model.files,'word/document.xml':strToU8(new XMLSerializer().serializeToString(xml))});
}
export async function newLibraryFile(format){
 if(format==='docx')return newDocx();
 if(format==='pdf'){const {PDFDocument}=await import('pdf-lib');const pdf=await PDFDocument.create();pdf.addPage();return pdf.save()}
 if(['xlsx','xls','ods','csv'].includes(format)){const x=await import('xlsx');const book=x.utils.book_new();x.utils.book_append_sheet(book,x.utils.aoa_to_sheet([['']]),'Sheet1');return writeWorkbook(x,book,format)}
 return new TextEncoder().encode('');
}
export function writeWorkbook(x,book,ext){if(ext!=='xlsx'&&book.SheetNames.some(n=>Object.values(book.Sheets[n]).some(c=>c&&typeof c==='object'&&c.f)))throw Error('Save this workbook as XLSX to preserve formulas.');book.Workbook={...book.Workbook,CalcPr:{...book.Workbook?.CalcPr,fullCalcOnLoad:true,forceFullCalc:true}};return new Uint8Array(x.write(book,{type:'array',bookType:ext==='xls'?'biff8':ext,cellStyles:true}))}
export function editCell(x,sheet,row,col,value){const key=x.utils.encode_cell({r:row,c:col});const old=sheet[key]||{};if(!value){delete sheet[key]}else{const cell={...old};delete cell.f;delete cell.w;delete cell.v;if(value.startsWith('=')){cell.f=value.slice(1);cell.t='n'}else if(/^-?(?:\d+\.?\d*|\.\d+)$/.test(value)){cell.v=Number(value);cell.t='n'}else{cell.v=value;cell.t='s'}sheet[key]=cell}const range=x.utils.decode_range(sheet['!ref']||'A1');range.e.r=Math.max(range.e.r,row);range.e.c=Math.max(range.e.c,col);sheet['!ref']=x.utils.encode_range(range)}
