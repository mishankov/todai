import { describe, expect, it } from "vitest";

import {
  ProtocolError,
  decodeCommand,
  encodeMessage,
} from "../../src/protocol/codec.js";
import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
} from "../../src/protocol/types.js";

describe("protocol codec", () => {
  it("decodes a versioned run.start command", () => {
    const command = decodeCommand(
      JSON.stringify({
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.start",
        requestId: "request-1",
        sessionId: "session-1",
        runId: "run-1",
        message: "Plan my day",
        history: [
          {
            role: "assistant",
            content: [
              {
                type: "toolCall",
                id: "call-1",
                name: "task_create",
                arguments: { title: "Review" },
              },
            ],
            timestamp: 1,
          },
          {
            role: "toolResult",
            toolCallId: "call-1",
            toolName: "task_create",
            content: [{ type: "text", text: '{"id":"task-1"}' }],
            details: { status: 200 },
            isError: false,
            timestamp: 2,
          },
        ],
        runtimeName: "fake",
        toolAccess: {
          baseUrl: "http://127.0.0.1:8080",
          token: "token",
          allowedTools: ["task_get"],
        },
        pi: { timezone: "Europe/Moscow", thinkingEffort: "high" },
      }),
    );

    expect(command).toEqual({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "run.start",
      requestId: "request-1",
      sessionId: "session-1",
      runId: "run-1",
      message: "Plan my day",
      history: [
        {
          role: "assistant",
          content: [
            {
              type: "toolCall",
              id: "call-1",
              name: "task_create",
              arguments: { title: "Review" },
            },
          ],
          timestamp: 1,
        },
        {
          role: "toolResult",
          toolCallId: "call-1",
          toolName: "task_create",
          content: [{ type: "text", text: '{"id":"task-1"}' }],
          details: { status: 200 },
          isError: false,
          timestamp: 2,
        },
      ],
      runtimeName: "fake",
      toolAccess: {
        baseUrl: "http://127.0.0.1:8080",
        token: "token",
        allowedTools: ["task_get"],
      },
      pi: { timezone: "Europe/Moscow", thinkingEffort: "high" },
    });
  });

  it("rejects an unknown thinking effort", () => {
    expect(() =>
      decodeCommand(
        JSON.stringify({
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
            token: "token",
            allowedTools: [],
          },
          pi: { thinkingEffort: "extreme" },
        }),
      ),
    ).toThrowError(expect.objectContaining({ code: "invalid_command" }));
  });

  it.each([
    ["invalid_json", "{"],
    [
      "unsupported_protocol",
      JSON.stringify({
        protocol: "other",
        version: 1,
        type: "run.abort",
        requestId: "r",
        runId: "x",
      }),
    ],
    [
      "unsupported_version",
      JSON.stringify({
        protocol: RUNNER_PROTOCOL,
        version: 999,
        type: "run.abort",
        requestId: "r",
        runId: "x",
      }),
    ],
    [
      "unknown_command",
      JSON.stringify({
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "task.delete",
        requestId: "r",
      }),
    ],
  ])("rejects an invalid command with %s", (code, line) => {
    expect.assertions(1);

    try {
      decodeCommand(line);
    } catch (error) {
      expect(error).toMatchObject<Partial<ProtocolError>>({ code });
    }
  });

  it("encodes exactly one JSONL record", () => {
    const encoded = encodeMessage({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "runner.ready",
      runtime: { name: "todai-runner", version: "0.3.0" },
    });

    expect(encoded.endsWith("\n")).toBe(true);
    expect(encoded.slice(0, -1)).not.toContain("\n");
    expect(JSON.parse(encoded)).toMatchObject({ type: "runner.ready" });
  });
});
