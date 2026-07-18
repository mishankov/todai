import { afterEach, describe, expect, it, vi } from "vitest";

import { ProtocolError } from "../../src/protocol/codec.js";
import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type RunnerOutput,
} from "../../src/protocol/types.js";
import { FakeRunner } from "../../src/runtime/fake-runner.js";

describe("fake runner", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("emits the deterministic happy path in protocol order", () => {
    vi.useFakeTimers();
    const output: RunnerOutput[] = [];
    const runner = new FakeRunner((message) => output.push(message));

    runner.start();
    runner.accept({
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
    });
    vi.runAllTimers();

    expect(output).toEqual([
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "runner.ready",
        runtime: { name: "todai-runner", version: "0.3.0" },
      },
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.started",
        requestId: "request-1",
        sessionId: "session-1",
        runId: "run-1",
        sequence: 1,
      },
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "assistant.delta",
        requestId: "request-1",
        sessionId: "session-1",
        runId: "run-1",
        sequence: 2,
        messageId: "fake-message-run-1",
        delta: "Fake response to: Plan my day",
      },
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "history.message",
        requestId: "request-1",
        sessionId: "session-1",
        runId: "run-1",
        sequence: 3,
        historyMessage: {
          role: "assistant",
          content: [{ type: "text", text: "Fake response to: Plan my day" }],
          timestamp: expect.any(Number),
        },
      },
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.completed",
        requestId: "request-1",
        sessionId: "session-1",
        runId: "run-1",
        sequence: 4,
      },
    ]);
  });

  it("turns an invalid command into a protocol event", () => {
    const output: RunnerOutput[] = [];
    const runner = new FakeRunner((message) => output.push(message));

    runner.reject(
      new ProtocolError(
        "unknown_command",
        "command type is not supported",
        "request-2",
      ),
    );

    expect(output).toEqual([
      {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "protocol.error",
        code: "unknown_command",
        message: "command type is not supported",
        requestId: "request-2",
      },
    ]);
  });

  it("aborts an active run before it completes", () => {
    vi.useFakeTimers();
    const output: RunnerOutput[] = [];
    const runner = new FakeRunner((message) => output.push(message));

    runner.accept({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "run.start",
      requestId: "request-start",
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
    });
    runner.accept({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "run.abort",
      requestId: "request-abort",
      runId: "run-1",
    });
    vi.runAllTimers();

    expect(output.at(-1)).toEqual({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "run.aborted",
      requestId: "request-abort",
      sessionId: "session-1",
      runId: "run-1",
      sequence: 2,
      reason: "requested",
    });
    expect(output.some((message) => message.type === "run.completed")).toBe(
      false,
    );
  });
});
