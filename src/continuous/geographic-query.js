import { request } from "./api.js";

// The interactive map spans [0,2] x [0,1]. Longitude wraps; polar margins
// are outside the world. This conversion never depends on preview pixels.
export function mapGeography(point) {
  if (
    !Array.isArray(point) ||
    point.length !== 2 ||
    !point.every(Number.isFinite) ||
    point[1] < 0 ||
    point[1] > 1
  )
    return null;
  const longitude = ((((point[0] * 180) % 360) + 360) % 360) - 180;
  return { longitude, latitude: 90 - point[1] * 180 };
}

export class GeographicQuery {
  constructor({
    fetchSample = request,
    budget = 1024 * 1024,
    limit = 256,
  } = {}) {
    this.fetchSample = fetchSample;
    this.budget = budget;
    this.limit = limit;
    this.cache = new Map();
    this.bytes = 0;
    this.serial = 0;
  }
  source(identity, revision) {
    const source = JSON.stringify([identity, revision]);
    if (source !== this.currentSource) {
      this.cancel();
      this.currentSource = source;
    }
  }
  cancel() {
    this.serial++;
    this.controller?.abort();
  }
  async select({ session, identity, revision, point }, onResult) {
    this.source(identity, revision);
    this.cancel();
    const coordinates = mapGeography(point);
    if (!coordinates) return;
    const serial = this.serial;
    const controller = (this.controller = new AbortController());
    const key = JSON.stringify([identity, revision, coordinates]);
    const started = performance.now();
    let entry = this.cache.get(key);
    try {
      const value =
        entry?.value ||
        (await this.fetchSample(
          `sessions/${session}/terrain/sample?lon=${coordinates.longitude}&lat=${coordinates.latitude}&revision=${revision}`,
          undefined,
          "GET",
          controller.signal,
        ));
      if (serial !== this.serial || controller.signal.aborted) return;
      if (value.revision !== undefined && value.revision !== revision) return;
      const result = {
        ...value,
        query: {
          identity,
          revision,
          ...coordinates,
          durationMs: performance.now() - started,
          cache: entry ? "hit" : "miss",
        },
      };
      if (!entry) {
        entry = {
          value,
          bytes: JSON.stringify(value).length * 2 + key.length * 2,
        };
        if (entry.bytes <= this.budget) {
          this.cache.set(key, entry);
          this.bytes += entry.bytes;
        }
      } else {
        this.cache.delete(key);
        this.cache.set(key, entry);
      }
      while (this.bytes > this.budget || this.cache.size > this.limit) {
        const oldest = this.cache.keys().next().value;
        this.bytes -= this.cache.get(oldest).bytes;
        this.cache.delete(oldest);
      }
      onResult(result);
      return result;
    } catch (error) {
      if (serial === this.serial && !controller.signal.aborted) throw error;
    }
  }
}
