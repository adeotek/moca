// graphify OpenCode plugin
// Injects a knowledge graph reminder before bash tool calls when the graph exists.
//
// IMPORTANT: keep the reminder string free of backticks and $(...) constructs.
// The hook prepends `echo "<reminder>" && <cmd>` to the user's bash command;
// backticks inside the double-quoted echo trigger bash command substitution,
// which both corrupts tool output and silently executes the very graphify
// command we are only suggesting. Plain words render fine in opencode's TUI.
//
// OpenCode V2 plugin API: default export { id, setup(ctx) }, registered via
// ctx.tool.hook("execute.before"). Behaviour is unchanged from the V1 port.
import { existsSync } from "fs";
import { join } from "path";

export default {
  id: "graphify",
  async setup(ctx) {
    const directory = ctx.location.directory;
    let reminded = false;

    await ctx.tool.hook("execute.before", (event) => {
      if (reminded) return;
      if (!existsSync(join(directory, "graphify-out", "graph.json"))) return;

      const tool = String(event?.tool ?? "").toLowerCase();
      if (tool !== "bash" && tool !== "shell") return;
      const args = event?.input;
      if (!args || typeof args !== "object") return;
      if (typeof args.command !== "string" || !args.command) return;

      // ';' not '&&' - Windows PowerShell 5.1 rejects '&&' as a statement
      // separator, breaking the first bash command of the session (#1646).
      args.command =
        'echo "[graphify] knowledge graph at graphify-out/. For focused questions, run graphify query with your question (scoped subgraph, usually much smaller than GRAPH_REPORT.md) instead of grepping raw files. Read GRAPH_REPORT.md only for broad architecture context." ; ' +
        args.command;
      reminded = true;
    });
  },
};
