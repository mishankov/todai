import { spawn } from "node:child_process";
import { once } from "node:events";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type RunnerOutput,
} from "../../src/protocol/types.js";

const componentDirectory = fileURLToPath(new URL("../..", import.meta.url));

describe("runner CLI", () => {
  it("keeps stdout as JSONL and completes a deterministic run", async () => {
    const result = await runCli(
      JSON.stringify({
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.start",
        requestId: "request-1",
        sessionId: "session-1",
        runId: "run-1",
        message: "Plan my day",
        history: [],
        runtimeName: "fake",
        toolAccess: {
          baseUrl: "http://127.0.0.1:8080",
          token: "token",
          allowedTools: ["task_get"],
        },
        pi: {},
      }),
      "run.completed",
    );

    expect(result.exitCode).toBe(0);
    expect(result.stdout.map((message) => message.type)).toEqual([
      "runner.ready",
      "run.started",
      "assistant.delta",
      "history.message",
      "run.completed",
    ]);
    expect(result.stderr).toContain("runner ready");
  });

  it("reports invalid input as a protocol event and logs to stderr", async () => {
    const result = await runCli("not-json", "protocol.error");

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toEqual([
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "runner.ready",
        runtime: { name: "todai-runner", version: "0.3.0" },
      },
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "protocol.error",
        code: "invalid_json",
        message: "command is not valid JSON",
      },
    ]);
    expect(result.stderr).toContain("invalid_json");
  });
});

async function runCli(input: string, terminalType: RunnerOutput["type"]) {
  const child = spawn(
    process.execPath,
    ["--import", "tsx", "src/cli/main.ts"],
    {
      cwd: componentDirectory,
      stdio: ["pipe", "pipe", "pipe"],
    },
  );
  const stdout: RunnerOutput[] = [];
  let stdoutBuffer = "";
  let stderr = "";
  let inputClosed = false;

  child.stdout.setEncoding("utf8");
  child.stdout.on("data", (chunk: string) => {
    stdoutBuffer += chunk;
    const lines = stdoutBuffer.split("\n");
    stdoutBuffer = lines.pop() ?? "";

    for (const line of lines) {
      if (line.length === 0) {
        continue;
      }
      const message = JSON.parse(line) as RunnerOutput;
      stdout.push(message);
      if (!inputClosed && message.type === terminalType) {
        inputClosed = true;
        child.stdin.end();
      }
    }
  });
  child.stderr.setEncoding("utf8");
  child.stderr.on("data", (chunk: string) => {
    stderr += chunk;
  });

  child.stdin.write(`${input}\n`);
  let timedOut = false;
  const timeout = setTimeout(() => {
    timedOut = true;
    child.kill("SIGKILL");
  }, 5_000);
  const [exitCode] = (await once(child, "exit")) as [
    number | null,
    NodeJS.Signals | null,
  ];
  clearTimeout(timeout);

  if (timedOut) {
    throw new Error("runner CLI did not terminate");
  }
  if (stdoutBuffer.length > 0) {
    throw new Error("runner CLI left an unterminated stdout record");
  }

  return { exitCode, stdout, stderr };
}
