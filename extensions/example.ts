/**
 * Reference example extension for weave. Demonstrates a minimal pi extension
 * and how weave resolves a tag to an absolute path. Registers a harmless
 * /weave-example command and greets on session start. Safe to delete once
 * you add real extensions.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("session_start", async (_event, ctx) => {
    ctx.ui.notify("weave example extension loaded", "info");
  });

  pi.registerCommand("weave-example", {
    description: "Prove the weave example extension is loaded",
    handler: async (_args, ctx) => {
      ctx.ui.notify("Hello from the weave example extension!", "info");
    },
  });
}
