import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type HistoryMessage,
  type RunnerOutput,
} from "../../src/protocol/types.js";

const harness = vi.hoisted(() => ({
  restoredMessages: [] as unknown[],
  listener: undefined as ((event: unknown) => void) | undefined,
  systemPrompt: "",
  thinkingLevel: "",
  prompt: "",
}));

vi.mock("@earendil-works/pi-coding-agent", () => ({
  DefaultResourceLoader: class {
    constructor(options: { systemPromptOverride: () => string }) {
      harness.systemPrompt = options.systemPromptOverride();
    }
    async reload() {
      return undefined;
    }
  },
  ModelRuntime: {
    create: async () => ({
      getModel: () => ({
        api: "openai-responses",
        provider: "openai",
        id: "test-model",
      }),
    }),
  },
  SessionManager: { inMemory: () => ({}) },
  createAgentSession: async (options: { thinkingLevel?: string }) => {
    harness.thinkingLevel = options.thinkingLevel ?? "";
    const state = {
      messages: [] as unknown[],
      model: {
        api: "openai-responses",
        provider: "openai",
        id: "test-model",
      },
    };
    return {
      session: {
        agent: { state },
        thinkingLevel: options.thinkingLevel ?? "off",
        subscribe: (listener: (event: unknown) => void) => {
          harness.listener = listener;
          return () => undefined;
        },
        prompt: async (prompt: string) => {
          harness.prompt = prompt;
          harness.restoredMessages = structuredClone(state.messages);
          emitRunEvents(harness.listener);
        },
        abort: async () => undefined,
        dispose: () => undefined,
      },
    };
  },
  defineTool: (definition: unknown) => definition,
  getAgentDir: () => "/tmp/pi",
}));

const { PiRunner } = await import("../../src/runtime/pi-runner.js");

describe("Pi runner history", () => {
  beforeEach(() => {
    harness.restoredMessages = [];
    harness.listener = undefined;
    harness.systemPrompt = "";
    harness.thinkingLevel = "";
    harness.prompt = "";
  });

  it("restores messages and emits persistable tool arguments and results", async () => {
    const output: RunnerOutput[] = [];
    const terminal = new Promise<void>((resolve) => {
      const runner = new PiRunner((event) => {
        output.push(event);
        if (event.type === "run.completed" || event.type === "run.failed")
          resolve();
      });
      runner.accept({
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.start",
        requestId: "request-2",
        sessionId: "session-1",
        runId: "run-2",
        message: "Move them",
        context: {
          type: "task",
          taskId: "11111111-1111-4111-8111-111111111111",
          action: "decompose",
        },
        history: testHistory(),
        runtimeName: "pi",
        toolAccess: {
          baseUrl: "http://127.0.0.1:8080",
          token: "scoped-token",
          allowedTools: ["task_get"],
        },
        pi: {
          provider: "openai",
          model: "test-model",
          timezone: "Europe/Moscow",
          thinkingEffort: "high",
        },
      });
    });
    await terminal;

    expect(output.find((event) => event.type === "run.failed")).toBeUndefined();
    expect(harness.systemPrompt).toContain("Europe/Moscow");
    expect(harness.systemPrompt).toContain(
      "/projects/<URL-encoded-projectId>/tasks/<URL-encoded-taskId>",
    );
    expect(harness.thinkingLevel).toBe("high");
    expect(harness.prompt).toBe(
      '<todai-context>{"type":"task","taskId":"11111111-1111-4111-8111-111111111111","action":"decompose"}</todai-context>\n\nMove them',
    );
    expect(output).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          type: "run.started",
          model: "test-model",
          thinkingEffort: "high",
        }),
      ]),
    );

    expect(harness.restoredMessages).toMatchObject([
      { role: "user", content: "Create a task" },
      {
        role: "assistant",
        content: [
          {
            type: "toolCall",
            id: "call-1",
            name: "task_get",
            arguments: { taskId: "task-1" },
          },
        ],
      },
      {
        role: "toolResult",
        toolCallId: "call-1",
        content: [{ type: "text", text: '{"id":"task-1"}' }],
        details: { status: 200 },
      },
      { role: "assistant", content: [{ type: "text", text: "Created." }] },
    ]);
    expect(output).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          type: "tool.started",
          arguments: { taskId: "task-1" },
        }),
        expect.objectContaining({
          type: "tool.completed",
          result: {
            content: [{ type: "text", text: '{"id":"task-1"}' }],
            details: { status: 200 },
          },
        }),
        expect.objectContaining({
          type: "history.message",
          historyMessage: expect.objectContaining({
            role: "toolResult",
            toolCallId: "call-2",
          }),
        }),
      ]),
    );
  });
});

function testHistory(): HistoryMessage[] {
  return [
    {
      role: "user",
      content: [{ type: "text", text: "Create a task" }],
      timestamp: 1,
    },
    {
      role: "assistant",
      content: [
        {
          type: "toolCall",
          id: "call-1",
          name: "task_get",
          arguments: { taskId: "task-1" },
        },
      ],
      timestamp: 2,
    },
    {
      role: "toolResult",
      toolCallId: "call-1",
      toolName: "task_get",
      content: [{ type: "text", text: '{"id":"task-1"}' }],
      details: { status: 200 },
      isError: false,
      timestamp: 3,
    },
    {
      role: "assistant",
      content: [{ type: "text", text: "Created." }],
      timestamp: 4,
    },
  ];
}

function emitRunEvents(listener: ((event: unknown) => void) | undefined) {
  if (!listener) throw new Error("Pi runner did not subscribe to the session");
  listener({ type: "agent_start" });
  listener({
    type: "message_end",
    message: {
      role: "assistant",
      content: [
        {
          type: "toolCall",
          id: "call-2",
          name: "task_get",
          arguments: { taskId: "task-1" },
        },
      ],
      timestamp: 5,
    },
  });
  listener({
    type: "tool_execution_start",
    toolCallId: "call-2",
    toolName: "task_get",
    args: { taskId: "task-1" },
  });
  listener({
    type: "tool_execution_end",
    toolCallId: "call-2",
    toolName: "task_get",
    result: {
      content: [{ type: "text", text: '{"id":"task-1"}' }],
      details: { status: 200 },
    },
    isError: false,
  });
  listener({
    type: "message_end",
    message: {
      role: "toolResult",
      toolCallId: "call-2",
      toolName: "task_get",
      content: [{ type: "text", text: '{"id":"task-1"}' }],
      details: { status: 200 },
      isError: false,
      timestamp: 6,
    },
  });
}
