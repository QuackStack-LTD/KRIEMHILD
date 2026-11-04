import test from "node:test";
import assert from "node:assert/strict";
import net from "node:net";
import { spawn } from "node:child_process";
import fs from "node:fs/promises";

test("a second launch refuses before altering the running server's frontend",async()=>{
 const server=net.createServer(socket=>socket.end());
 await new Promise(resolve=>server.listen(0,"127.0.0.1",resolve));
 try{
  const before=await fs.readFile("dist/index.html");
  const child=spawn(process.execPath,["scripts/start.mjs"],{env:{...process.env,KRIEMHILD_ADDR:`127.0.0.1:${server.address().port}`},windowsHide:true});
  let output="";child.stderr.on("data",data=>output+=data);
  const exit=await new Promise(resolve=>child.on("exit",resolve));
  assert.equal(exit,1);assert.match(output,/already running/);assert.match(output,/No build files were changed/);
  assert.deepEqual(await fs.readFile("dist/index.html"),before);
 }finally{await new Promise(resolve=>server.close(resolve))}
});
