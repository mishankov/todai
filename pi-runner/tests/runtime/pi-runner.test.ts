import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type RunnerOutput,
} from "../../src/protocol/types.js";
import { PiRunner } from "../../src/runtime/pi-runner.js";

describe("Pi runner", () => {
  it("reports an unavailable configured model as one stable terminal event", async () => {
    const agentDir = await mkdtemp(join(tmpdir(), "todai-pi-test-"));
    const output: RunnerOutput[] = [];
    let resolveTerminal: () => void = () => undefined;
    const terminal = new Promise<void>((resolve) => {
      resolveTerminal = resolve;
    });
    const runner = new PiRunner((event) => {
      output.push(event);
      if (event.type === "run.failed") resolveTerminal();
    });

    try {
      runner.accept({
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.start",
        requestId: "request-1",
        sessionId: "session-1",
        runId: "run-1",
        message: "Plan my day",
        history: [],
        runtimeName: "pi",
        toolAccess: {
          baseUrl: "http://127.0.0.1:8080",
          token: "scoped-token",
          allowedTools: ["task_get"],
        },
        pi: { agentDir, provider: "missing", model: "missing" },
      });
      await Promise.race([
        terminal,
        new Promise((_, reject) =>
          setTimeout(
            () => reject(new Error("Pi runner did not terminate")),
            2_000,
          ),
        ),
      ]);

      expect(output).toHaveLength(1);
      expect(output[0]).toMatchObject({
        type: "run.failed",
        sequence: 1,
        error: { code: "pi_runtime_error", retryable: false },
      });
    } finally {
      await runner.close();
      await rm(agentDir, { recursive: true, force: true });
    }
  });
});
