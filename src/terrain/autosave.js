// Serialize view writes, coalesce continuous gestures, and retain failed saves.
// Engine mutations and explored tiles are committed independently by the server.
export class AutosaveQueue {
  constructor(send, report=()=>{}, delay=300) {
    this.send=send;this.report=report;this.delay=delay;this.pending=null;this.running=null;this.timer=0;this.closed=false;
  }
  schedule(value) {
    if(this.closed)return;
    this.pending=value;this.report('pending');
    // A fixed deadline prevents a continuous drag from postponing saves forever.
    if(!this.timer&&!this.running)this.timer=setTimeout(()=>{this.timer=0;void this.flush().catch(()=>{});},this.delay);
  }
  async flush() {
    clearTimeout(this.timer);this.timer=0;
    if(this.running){await this.running;return this.flush();}
    if(!this.pending||this.closed)return;
    const value=this.pending;this.pending=null;this.report('saving');
    this.running=this.send(value);
    try {await this.running;this.report('saved');}
    catch(error){this.pending??=value;this.report('error',error);throw error;}
    finally {this.running=null;if(this.pending&&!this.closed)this.timer=setTimeout(()=>{this.timer=0;void this.flush().catch(()=>{});},2000);}
    if(this.pending)return this.flush();
  }
  discard(){this.pending=null;clearTimeout(this.timer);this.timer=0;}
  dispose(){this.closed=true;this.discard();}
}
