// Code loads this plugin into each OpenCode 2 it starts, through the `plugins`
// directory Code names in OPENCODE_CONFIG_CONTENT, so nothing is written to the
// user's configuration. OpenCode 2 replaced the plugin API, so this is a
// separate file from the OpenCode 1 plugin beside it, and it reports the
// session's progress to Code with `cde event`.
import { spawn } from "node:child_process"
import { Plugin } from "@opencode/plugin"

const detailLimit = 200

function summary(text) {
  const line = String(text ?? "").replace(/\s+/g, " ").trim()
  return line.length > detailLimit ? line.slice(0, detailLimit - 1) + "…" : line
}

export default Plugin.define({
  id: "code.terminal",
  async setup(ctx) {
    const cde = process.env.CODE_CDE
    if (!cde) return

    // Subagents run in child sessions, and would make their parent's state
    // flap while it waits on them.
    const children = new Set()
    const live = new Set()
    // Code orders events by when `cde` sends them, so each waits for the one before.
    let sent = Promise.resolve()

    function send(kind, sessionID, detail) {
      if (!sessionID || children.has(sessionID)) return
      live.add(sessionID)
      const args = ["event", kind, "--agent", "opencode", "--session", sessionID]
      if (detail) args.push("--detail", summary(detail))
      sent = sent.then(() => new Promise((resolve) => {
        try {
          const child = spawn(cde, args, { stdio: "ignore" })
          child.on("error", resolve)
          child.on("close", resolve)
        } catch {
          resolve()
        }
      }))
    }

    function sessionID(event) {
      return event.data?.sessionID ?? event.data?.info?.id ?? event.data?.session?.id
    }

    function handle(event) {
      const id = sessionID(event)
      switch (event.type) {
        case "session.created":
          if (event.data?.info?.parentID) children.add(id)
          else send("sessionStarted", id)
          break
        // A turn is an execution: it starts, then succeeds, fails or is
        // interrupted. The step events inside it are not the turn's end, so
        // compaction and retries do not flap the status.
        case "session.execution.started":
          send("turnStarted", id)
          break
        case "session.execution.succeeded":
          send("turnEnded", id)
          break
        case "session.execution.failed":
        case "session.execution.interrupted": {
          const message = event.data?.error?.message ?? event.data?.error?.name
          send("turnEnded", id, message)
          break
        }
        case "session.tool.called":
          send("working", id, event.data?.tool ?? event.data?.name)
          break
        case "permission.asked":
        case "session.form.created":
          send("needsInput", id, event.data?.permission ?? event.data?.title ?? "input")
          break
        case "permission.replied":
        case "permission.rejected":
        case "session.form.replied":
        case "session.form.cancelled":
          send("working", id)
          break
        case "session.deleted":
          send("sessionEnded", id)
          live.delete(id)
          children.delete(id)
          break
      }
    }

    const controller = new AbortController()
    void (async () => {
      try {
        for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
          handle(event)
        }
      } catch {
        // The stream ends on unload, which the cleanup below handles.
      }
    })()

    // Quitting OpenCode deletes no sessions, so the ones it had open end here.
    return () => {
      controller.abort()
      for (const id of live) send("sessionEnded", id)
      live.clear()
      return Promise.race([sent, new Promise((resolve) => setTimeout(resolve, 1000))])
    }
  },
})
