import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { once } from "node:events";
import { copyFile, mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterAll, beforeAll, describe, expect, it } from "vitest";

import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type RunStartCommand,
  type RunnerOutput,
} from "../../src/protocol/types.js";

const smokeEnabled = process.env.TODAI_PI_SMOKE === "1";
const sourceAgentDir = process.env.TODAI_PI_AGENT_DIR ?? "";
const provider = process.env.TODAI_PI_PROVIDER ?? "";
const model = process.env.TODAI_PI_MODEL ?? "";

describe.runIf(smokeEnabled)("real Pi smoke", () => {
  let agentDir: string;
  let toolServer: ReturnType<typeof createServer>;
  let toolBaseUrl: string;
  let runner: RunnerProcess | undefined;

  beforeAll(async () => {
    if (!sourceAgentDir || !provider || !model) {
      throw new Error(
        "TODAI_PI_AGENT_DIR, TODAI_PI_PROVIDER, and TODAI_PI_MODEL are required",
      );
    }
    agentDir = await mkdtemp(join(tmpdir(), "todai-pi-smoke-"));
    await copyFile(
      join(sourceAgentDir, "auth.json"),
      join(agentDir, "auth.json"),
    );
    try {
      await copyFile(
        join(sourceAgentDir, "models.json"),
        join(agentDir, "models.json"),
      );
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
    }

    toolServer = createServer((request, response) => {
      if (
        request.method !== "POST" ||
        request.url !== "/internal/tools/task_get"
      ) {
        response.writeHead(404).end();
        return;
      }
      if (request.headers.authorization === "Bearer smoke-cancel") {
        request.once("close", () => response.destroy());
        return;
      }
      response.writeHead(200, { "content-type": "application/json" }).end(
        JSON.stringify({
          id: "smoke-task",
          title: "Bun compatibility smoke",
          version: 1,
          subtasks: [],
          comments: [],
        }),
      );
    });
    toolServer.listen(0, "127.0.0.1");
    await once(toolServer, "listening");
    const address = toolServer.address();
    if (address === null || typeof address === "string") {
      throw new Error("tool smoke server did not bind a TCP port");
    }
    toolBaseUrl = `http://127.0.0.1:${address.port}`;
    runner = await RunnerProcess.start();
  }, 30_000);

  afterAll(async () => {
    await runner?.close();
    toolServer?.close();
    if (toolServer?.listening) await once(toolServer, "close");
    if (agentDir) await rm(agentDir, { recursive: true, force: true });
  });

  it("authenticates, streams, calls a Todai tool, and persists valid auth state", async () => {
    const authPath = join(agentDir, "auth.json");
    const before = await stat(authPath);
    const events = await runner!.run(
      command(
        "real",
        "smoke-token",
        [
          "Call task_get exactly once with taskId smoke-task.",
          "Then answer with the exact marker TODAI_BUN_SMOKE and a short summary.",
        ].join(" "),
      ),
      "run.completed",
    );

    expect(events.some((event) => event.type === "run.started")).toBe(true);
    expect(events.some((event) => event.type === "assistant.delta")).toBe(true);
    expect(
      events.some(
        (event) =>
          event.type === "tool.started" && event.toolName === "task_get",
      ),
    ).toBe(true);
    expect(
      events.some(
        (event) =>
          event.type === "tool.completed" &&
          event.toolName === "task_get" &&
          !event.isError,
      ),
    ).toBe(true);
    expect(
      events
        .filter((event) => event.type === "assistant.delta")
        .map((event) => event.delta)
        .join(""),
    ).toContain("TODAI_BUN_SMOKE");

    const persisted = JSON.parse(await readFile(authPath, "utf8")) as unknown;
    expect(persisted).toBeTypeOf("object");
    expect((await stat(authPath)).mtimeMs).toBeGreaterThanOrEqual(
      before.mtimeMs,
    );
  }, 120_000);

  it("cancels an authenticated tool run and remains ready for clean shutdown", async () => {
    const run = command(
      "cancel",
      "smoke-cancel",
      "Call task_get with taskId smoke-task and wait for its result.",
    );
    runner!.write(run);
    await runner!.waitFor(
      (event) =>
        "runId" in event &&
        event.runId === run.runId &&
        event.type === "tool.started" &&
        event.toolName === "task_get",
    );
    runner!.write({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "run.abort",
      requestId: "request-cancel-abort",
      runId: run.runId,
    });
    const aborted = await runner!.waitFor(
      (event) =>
        "runId" in event &&
        event.runId === run.runId &&
        event.type === "run.aborted",
    );
    expect(aborted).toMatchObject({ type: "run.aborted", reason: "requested" });
  }, 120_000);

  function command(
    suffix: string,
    token: string,
    message: string,
  ): RunStartCommand {
    return {
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "run.start",
      requestId: `request-${suffix}`,
      sessionId: `session-${suffix}`,
      runId: `run-${suffix}`,
      message,
      history: [],
      runtimeName: "pi",
      toolAccess: {
        baseUrl: toolBaseUrl,
        token,
        allowedTools: ["task_get"],
      },
      pi: { agentDir, provider, model, thinkingEffort: "off" },
    };
  }
});

class RunnerProcess {
  readonly #child: ChildProcessWithoutNullStreams;
  readonly #events: RunnerOutput[] = [];
  readonly #waiters = new Set<{
    predicate: (event: RunnerOutput) => boolean;
    resolve: (event: RunnerOutput) => void;
  }>();
  #stdout = "";
  #stderr = "";

  private constructor(child: ChildProcessWithoutNullStreams) {
    this.#child = child;
    child.stdout.setEncoding("utf8");
    child.stdout.on("data", (chunk: string) => this.#accept(chunk));
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (chunk: string) => {
      this.#stderr += chunk;
    });
  }

  static async start(): Promise<RunnerProcess> {
    const standalone = process.env.TODAI_RUNNER_EXECUTABLE?.trim();
    const executable = standalone || process.execPath;
    const args = standalone ? [] : ["src/cli/main.ts"];
    const processRunner = new RunnerProcess(
      spawn(executable, args, { stdio: ["pipe", "pipe", "pipe"] }),
    );
    await processRunner.waitFor((event) => event.type === "runner.ready");
    return processRunner;
  }

  write(command: object): void {
    this.#child.stdin.write(`${JSON.stringify(command)}\n`);
  }

  async run(
    command: RunStartCommand,
    terminal: RunnerOutput["type"],
  ): Promise<RunnerOutput[]> {
    const start = this.#events.length;
    this.write(command);
    await this.waitFor(
      (event) =>
        "runId" in event &&
        event.runId === command.runId &&
        event.type === terminal,
    );
    return this.#events.slice(start);
  }

  waitFor(predicate: (event: RunnerOutput) => boolean): Promise<RunnerOutput> {
    const existing = this.#events.findLast(predicate);
    if (existing) return Promise.resolve(existing);
    return new Promise((resolve, reject) => {
      const waiter = { predicate, resolve };
      this.#waiters.add(waiter);
      const timeout = setTimeout(() => {
        this.#waiters.delete(waiter);
        reject(new Error(`runner event timed out; stderr: ${this.#stderr}`));
      }, 110_000);
      waiter.resolve = (event) => {
        clearTimeout(timeout);
        resolve(event);
      };
    });
  }

  async close(): Promise<void> {
    this.#child.stdin.end();
    const exit = Promise.race([
      once(this.#child, "exit"),
      new Promise<never>((_, reject) =>
        setTimeout(
          () => reject(new Error("runner did not exit cleanly")),
          5_000,
        ),
      ),
    ]);
    try {
      await exit;
    } catch (error) {
      this.#child.kill("SIGKILL");
      throw error;
    }
  }

  #accept(chunk: string): void {
    this.#stdout += chunk;
    const lines = this.#stdout.split("\n");
    this.#stdout = lines.pop() ?? "";
    for (const line of lines) {
      if (!line) continue;
      const event = JSON.parse(line) as RunnerOutput;
      this.#events.push(event);
      for (const waiter of this.#waiters) {
        if (!waiter.predicate(event)) continue;
        this.#waiters.delete(waiter);
        waiter.resolve(event);
      }
    }
  }
}
