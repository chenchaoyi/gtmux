/** Keep a timed-out device journey from racing the next test's cleanup or setup. */
export function createJourneyGuard() {
  let active: Promise<void> | undefined;
  return {
    run(work: () => Promise<void>): Promise<void> {
      const pending = work();
      active = pending;
      void pending.finally(() => { if (active === pending) active = undefined; }).catch(() => {});
      return pending;
    },
    async wait(): Promise<void> {
      if (active) await active.catch(() => {});
    },
  };
}
