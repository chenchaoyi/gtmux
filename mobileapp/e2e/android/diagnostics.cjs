/* eslint-env node */
// Only used by the owned-emulator runner, never bundled into the product.
function logTail(limit = 2 * 1024 * 1024) {
  if(!Number.isSafeInteger(limit)||limit<1)throw new RangeError('Positive log byte budget required');
  let chunks = [], bytes = 0;
  return {
    push(value) {
      const chunk = Buffer.from(value);
      if (chunk.length >= limit) { chunks = [Buffer.from(chunk.subarray(-limit))]; bytes = limit; return; }
      chunks.push(chunk); bytes += chunk.length;
      while (bytes > limit) {
        const excess = bytes - limit;
        if (chunks[0].length <= excess) bytes -= chunks.shift().length;
        else { chunks[0] = chunks[0].subarray(excess); bytes -= excess; }
      }
    },
    text() { return Buffer.concat(chunks, bytes).toString('utf8'); },
    get bytes() { return bytes; },
  };
}
function automationDisconnected(error) {
  const message = String(error?.message || error);
  return /instrumentation process.*not running|device offline|socket hang up|invalid session id|session.*(?:deleted|terminated)/i.test(message);
}
// Preserve ADB diagnostics even when WebDriver screenshots/source are unavailable.
async function captureIndependently(actions) {
  let first;
  for (const action of actions) {
    try { await action(); } catch (error) { first ||= error; }
  }
  if (first) throw first;
}
module.exports = {logTail, automationDisconnected, captureIndependently};
