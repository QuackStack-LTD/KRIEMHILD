import test from 'node:test';
import assert from 'node:assert/strict';
import {unzipSync,strFromU8} from 'fflate';
import * as XLSX from 'xlsx';
import {PDFDocument} from 'pdf-lib';
import {newDocx,newLibraryFile,editorKind,writeWorkbook,editCell} from '../src/worldbuilding/library-files.js';

test('new DOCX is a real document archive with escaped text and package relationships',()=>{
 const files=unzipSync(newDocx('Archive & law\n<decree>'));
 assert.ok(files['[Content_Types].xml']&&files['_rels/.rels']);
 const xml=strFromU8(files['word/document.xml']);assert.match(xml,/Archive &amp; law/);assert.match(xml,/&lt;decree&gt;/);assert.equal((xml.match(/<w:p>/g)||[]).length,2);
});
test('spreadsheet edits survive XLS/XLSX/ODS file round-trips without formula execution',async()=>{
 for(const format of ['xls','xlsx','ods']){
  const initial=await newLibraryFile(format),book=XLSX.read(initial,{type:'array'}),sheet=book.Sheets.Sheet1;
  editCell(XLSX,sheet,0,0,'Revenue');editCell(XLSX,sheet,1,0,'120');editCell(XLSX,sheet,1,1,'=A2*2');
  const saved=writeWorkbook(XLSX,book,'xlsx'),loaded=XLSX.read(saved,{type:'array',cellFormula:true});
  assert.equal(loaded.Sheets.Sheet1.A1.v,'Revenue');assert.equal(loaded.Sheets.Sheet1.A2.v,120);assert.equal(loaded.Sheets.Sheet1.B2.f,'A2*2');
  editCell(XLSX,sheet,1,0,'');assert.equal(sheet.A2,undefined);
 }
});
test('PDF creation and updates produce real PDF bytes; unsupported files stay opaque',async()=>{
 const bytes=await newLibraryFile('pdf');assert.equal(new TextDecoder().decode(bytes.slice(0,4)),'%PDF');
 const pdf=await PDFDocument.load(bytes);pdf.getPage(0).drawText('Royal decree');const revised=await pdf.save();assert.equal((await PDFDocument.load(revised)).getPageCount(),1);
 assert.equal(editorKind('archive.bin'),'binary');assert.equal(editorKind('dogma.md'),'text');assert.equal(editorKind('ledger.XLS'),'sheet');
});
