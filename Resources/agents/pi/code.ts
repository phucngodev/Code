import { spawn } from "node:child_process"

// Pi loads this extension into each run it starts, through `--extension`, so
// nothing is written to the user's configuration. It reports the session to
// Code with `cde event`, so Code never reads Pi's files.
export default function (pi) {
  const cde = process.env.CODE_CDE
  if (!cde) return

  const detailLimit = 200

  // Code orders events by when `cde` starts, and Pi fires several in a burst,
  // so each send waits for the one before it. A hook that gave up is still
  // fine: `cde event` never fails the agent.
  let sent = Promise.resolve()
  let lastText = ""
  let lastError = ""

  function summary(text) {
    const line = String(text).split(/\s+/).filter(Boolean).join(" ")
    return line.length > detailLimit ? line.slice(0, detailLimit - 1) + "…" : line
  }

  function send(kind, sessionID, detail) {
    const args = ["event", kind, "--agent", "pi"]
    if (sessionID) args.push("--session", sessionID)
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

  function assistantText(message) {
    const blocks = Array.isArray(message?.content) ? message.content : []
    return blocks.filter((block) => block.type === "text").map((block) => block.text).join(" ").trim()
  }

  pi.on("session_start", (_event, ctx) => {
    lastText = ""
    lastError = ""
    send("sessionStarted", ctx.sessionManager.getSessionId())
  })

  pi.on("before_agent_start", (event, ctx) => {
    // A run started by a custom message is not a turn the user asked for.
    if (event.messageType !== "user") return
    send("turnStarted", ctx.sessionManager.getSessionId())
  })

  pi.on("tool_execution_start", (event, ctx) => {
    send("working", ctx.sessionManager.getSessionId(), event.toolName)
  })

  // Extensions put questions to the user through the UI, and Pi reports those
  // prompts, so a terminal can tell waiting on the user from working.
  pi.on("ui_prompt_start", (event, ctx) => {
    send("needsInput", ctx.sessionManager.getSessionId(), event.title || event.kind)
  })

  pi.on("ui_prompt_end", (_event, ctx) => {
    send("working", ctx.sessionManager.getSessionId())
  })

  // Pi may retry, compact or continue after a low-level run, so only
  // agent_settled means the turn is over. The last assistant message is the
  // turn's detail, or the error that ended it.
  pi.on("message_end", (event) => {
    const message = event.message
    if (!message || message.role !== "assistant") return
    if (message.stopReason === "error") {
      lastError = message.errorMessage || "error"
      return
    }
    const text = assistantText(message)
    if (text) {
      lastText = text
      lastError = ""
    }
  })

  pi.on("agent_settled", async (_event, ctx) => {
    send("turnEnded", ctx.sessionManager.getSessionId(), lastError || lastText)
    lastText = ""
    lastError = ""
    await sent
  })

  pi.on("session_shutdown", async (event, ctx) => {
    if (event.reason !== "quit") return
    send("sessionEnded", ctx.sessionManager.getSessionId())
    await sent
  })
}
