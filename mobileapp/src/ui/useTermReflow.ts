import {useCallback, useEffect, useLayoutEffect, useRef, useState} from 'react';
import type {RefObject} from 'react';
import type {TermAnchor} from './termViewport';

export interface TermReadingPosition {anchor: TermAnchor | null; y: number; following: boolean}
export interface TermReflowViewport {
  capture: () => TermReadingPosition;
  restore: (position: TermReadingPosition, original: boolean) => void;
  freeze: () => void;
  release: () => void;
}
interface Reflow {original: boolean; position: TermReadingPosition; applied: boolean}

/** Paint feedback before row generation; native size events are not guaranteed for equal-height layouts. */
export function useTermReflow(requested: boolean, selectionActive: boolean, viewport: RefObject<TermReflowViewport | null>) {
  const [original, setOriginal] = useState(requested);
  const [loading, setLoading] = useState(false);
  const applied = useRef(original);
  const active = useRef<Reflow | null>(null);
  const frames = useRef(new Set<number>());
  const nextFrame = useCallback((action: () => void) => {
    const id = requestAnimationFrame(() => { frames.current.delete(id); action(); });
    frames.current.add(id);
  }, []);
  const cancelFrames = useCallback(() => {
    for (const id of frames.current) cancelAnimationFrame(id);
    frames.current.clear();
  }, []);

  useEffect(() => {
    if (selectionActive || requested === applied.current) {
      const old = active.current;
      if (old) {
        viewport.current?.restore(old.position, applied.current);
        active.current = null;
        setLoading(false);
        viewport.current?.release();
      }
      return cancelFrames;
    }
    const view = viewport.current;
    if (!view) return cancelFrames;
    // Keep the original anchor if another request interrupts an unsettled reflow.
    const job: Reflow = {original: requested, position: active.current?.position ?? view.capture(), applied: false};
    active.current = job;
    view.freeze();
    setLoading(true);
    nextFrame(() => nextFrame(() => {
      if (active.current === job) setOriginal(job.original);
    }));
    return cancelFrames;
  }, [requested, selectionActive, viewport, nextFrame, cancelFrames]);

  useLayoutEffect(() => {
    applied.current = original;
    const job = active.current;
    if (!job || job.original !== original) return;
    job.applied = true;
    // The native commit must receive the new grid before we restore. A second frame
    // also covers equal-size content that does not emit onContentSizeChange.
    nextFrame(() => {
      if (active.current !== job) return;
      viewport.current?.restore(job.position, original);
      nextFrame(() => {
        if (active.current !== job) return;
        viewport.current?.restore(job.position, original);
        active.current = null;
        setLoading(false);
        viewport.current?.release();
      });
    });
  }, [original, viewport, nextFrame]);

  useEffect(() => () => { cancelFrames(); active.current = null; }, [cancelFrames]);
  const restoreAfterResize = () => {
    const job = active.current;
    if (!job) return false;
    if (job.applied) viewport.current?.restore(job.position, job.original);
    return true;
  };
  return {original, loading, active, restoreAfterResize};
}
