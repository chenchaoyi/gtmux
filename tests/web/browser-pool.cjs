// One process per Node test worker, never a shared BrowserContext.
function browserPool(launch) {
  let pending,
    browser,
    launches = 0;
  return {
    async get() {
      if (browser?.isConnected()) return browser;
      if (!pending) {
        launches++;
        pending = Promise.resolve()
          .then(launch)
          .then(value => {
            browser = value;
            return value;
          })
          .finally(() => {
            pending = undefined;
          });
      }
      return pending;
    },
    async close() {
      const value = pending ? await pending.catch(() => undefined) : browser;
      browser = undefined;
      await value?.close();
    },
    get launches() {
      return launches;
    },
  };
}
module.exports = { browserPool };
