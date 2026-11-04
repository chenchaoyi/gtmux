// V2 uses a shared service and domain hooks. This definition is also a V1 module:
// old loaders select server(), while V2 selects setup(); neither runs both.
export default {
  id: "gtmux",
  server: GtmuxHook,
  setup: async (ctx) => {
    const { spawn } = await import("node:child_process");
    const BIN = "__GTMUX_BIN__";
    const roots = new Map();
    const names = new Map();
    const recorded = new Set();
    const active = new Set();
    let pending = Promise.resolve();
    const privateServer =
      process.argv.includes("--stdio") && !process.argv.includes("--service");
    const send = (event, obj) =>
      new Promise((resolve) => {
        const env = { ...process.env };
        delete env.TMUX_PANE;
        if (!privateServer) delete env.TMUX;
        const child = spawn(BIN, ["hook", "--agent", "opencode", event], {
          env,
          stdio: ["pipe", "ignore", "ignore"],
        });
        const timer = setTimeout(() => {
          child.kill();
          resolve();
        }, 3000);
        child.once("error", () => {
          clearTimeout(timer);
          resolve();
        });
        child.once("close", () => {
          clearTimeout(timer);
          resolve();
        });
        child.stdin.on("error", () => {});
        child.stdin.end(
          JSON.stringify({
            ...obj,
            opencode_v2: true,
            opencode_private: privateServer,
          })
        );
      });
    const root = async (sid, created) => {
      if (!sid) return;
      if (created) roots.set(sid, created);
      if (!roots.has(sid)) {
        try {
          const result = await ctx.session.get({ sessionID: sid });
          roots.set(sid, result.data || result);
        } catch {
          return;
        }
      }
      if (roots.size > 512) roots.delete(roots.keys().next().value);
      const info = roots.get(sid);
      return info && !info.parentID ? info : undefined;
    };
    const payload = (sid, info) => ({
      session_id: sid,
      cwd: info.location?.directory || ctx.location.directory,
    });
    await ctx.session.hook("prompt", async (input) => {
      const info = await root(input.sessionID);
      if (!info || !input.prompt.text?.trim()) return;
      await send("Profile", {
        ...payload(input.sessionID, info),
        gtmux_profile: {
          model: info.model?.id || "",
          provider: info.model?.providerID || "",
          role: info.agent || "",
          variant: info.model?.variant || "",
        },
      });
      active.add(input.sessionID);
      await send("UserPromptSubmit", {
        ...payload(input.sessionID, info),
        prompt: input.prompt.text,
      });
    });
    const tool = async (sid, id, name, input) => {
      const info = await root(sid);
      const key = sid + "/" + id;
      if (!info || recorded.has(key)) return;
      recorded.add(key);
      if (recorded.size > 2048) recorded.delete(recorded.values().next().value);
      await send("Transcript", {
        ...payload(sid, info),
        transcript: { kind: "tool", id, name, input },
      });
    };
    await ctx.tool.hook("execute.before", async (input) => {
      await new Promise((resolve) => setImmediate(resolve));
      await pending;
      await tool(input.sessionID, input.id, input.tool, input.input);
      const info = await root(input.sessionID);
      if (info && input.tool === "question")
        await send("Question", payload(input.sessionID, info));
    });
    await ctx.tool.hook("execute.after", async (input) => {
      const info = await root(input.sessionID);
      if (info && input.tool === "question")
        await send("Resumed", payload(input.sessionID, info));
    });
    await ctx.session.hook("experimental.idle", async (input) => {
      // Drain durable text/tool events before the CLI closes its server lease.
      await new Promise((resolve) => setImmediate(resolve));
      await pending;
      const info = await root(input.sessionID);
      if (info && active.delete(input.sessionID))
        await send("Stop", payload(input.sessionID, info));
    });
    const types = [
      "session.created",
      "session.deleted",
      "session.execution.started",
      "session.execution.succeeded",
      "session.execution.failed",
      "session.execution.interrupted",
      "session.text.ended",
      "session.tool.input.started",
      "session.tool.called",
      "session.compaction.started",
      "session.compaction.ended",
      "permission.asked",
      "permission.replied",
    ];
    const stream = ctx.event.subscribe({ types })[Symbol.asyncIterator]();
    let closed = false;
    const consume = async () => {
      while (!closed) {
        const next = await stream.next();
        if (next.done) break;
        const event = next.value;
        pending = pending
          .then(async () => {
            const data = event.data || {};
            const sid = data.sessionID;
            const info = await root(
              sid,
              event.type === "session.created" ? data : undefined
            );
            if (!info) return;
            const body = payload(sid, info);
            if (event.type === "session.tool.input.started") {
              names.set(sid + "/" + data.id, data.name);
              if (names.size > 2048) names.delete(names.keys().next().value);
              return;
            }
            if (event.type === "session.tool.called") {
              const key = sid + "/" + data.id;
              const name = names.get(key);
              names.delete(key);
              if (name) await tool(sid, data.id, name, data.input);
              return;
            }
            if (event.type === "session.text.ended") {
              await send("Transcript", {
                ...body,
                transcript: { kind: "text", text: data.text },
              });
              return;
            }
            const tokens = {
              "session.created": "SessionStart",
              "session.deleted": "SessionEnd",
              "session.execution.started": "TurnStarted",
              "session.execution.succeeded": "Stop",
              "session.execution.failed": "StopFailure",
              "session.execution.interrupted": "Interrupt",
              "session.compaction.started": "PreCompact",
              "session.compaction.ended": "PostCompact",
              "permission.asked": "PermissionRequest",
              "permission.replied": "Resumed",
            };
            if (event.type === "session.execution.started") active.add(sid);
            if (
              event.type === "session.execution.succeeded" &&
              !active.delete(sid)
            )
              return;
            if (
              event.type === "session.execution.failed" ||
              event.type === "session.execution.interrupted"
            )
              active.delete(sid);
            if (tokens[event.type])
              await send(tokens[event.type], {
                ...body,
                error: data.error?.message || data.error?.name || "",
              });
            if (event.type === "session.deleted") roots.delete(sid);
          })
          .catch(() => {});
      }
    };
    void consume().catch(() => {}); // observation failures must never fail the agent's execution
    return () => {
      closed = true;
      void stream.return?.().catch(() => {});
    };
  },
};
