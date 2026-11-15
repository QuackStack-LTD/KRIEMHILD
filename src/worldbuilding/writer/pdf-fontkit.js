import { create as createFont } from "fontkit";

// pdf-lib 1.x consumes the older encodeStream contract. Fontkit's modular
// browser build returns the same subset bytes from encode(), without pulling
// Node stream polyfills and a prebundled font engine into the PDF chunk.
export default {
  create(bytes) {
    const font = createFont(bytes);
    const createSubset = font.createSubset.bind(font);
    font.createSubset = () => {
      const subset = createSubset();
      subset.encodeStream = () => {
        const handlers = new Map();
        const stream = {
          on(event, handler) {
            handlers.set(event, handler);
            return stream;
          },
        };
        queueMicrotask(() => {
          try {
            handlers.get("data")?.(subset.encode());
            handlers.get("end")?.();
          } catch (error) {
            handlers.get("error")?.(error);
          }
        });
        return stream;
      };
      return subset;
    };
    return font;
  },
};
