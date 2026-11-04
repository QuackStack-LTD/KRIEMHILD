// One pending camera, one animation frame. Incoming input composes against the
// latest pending transform; there is no queue of old viewport requests.
export class FrameNavigator {
  constructor(
    read,
    commit,
    schedule = (callback) => globalThis.requestAnimationFrame(callback),
    unschedule = (id) => globalThis.cancelAnimationFrame(id),
  ) {
    this.read = read;
    this.commit = commit;
    this.schedule = schedule;
    this.unschedule = unschedule;
  }
  transform(update) {
    this.pending = update(this.pending || this.read());
    if (this.frame === undefined)
      this.frame = this.schedule(() => this.flush());
  }
  flush() {
    this.frame = undefined;
    const next = this.pending;
    this.pending = null;
    if (next) this.commit(next);
  }
  cancel() {
    if (this.frame !== undefined) this.unschedule(this.frame);
    this.frame = undefined;
    this.pending = null;
  }
}
