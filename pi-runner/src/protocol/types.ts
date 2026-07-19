export const RUNNER_PROTOCOL = "todai.runner" as const;
export const RUNNER_PROTOCOL_VERSION = 3 as const;

export type ThinkingEffort =
  "off" | "minimal" | "low" | "medium" | "high" | "xhigh" | "max";

export interface HistoryTextContent {
  type: "text";
  text: string;
}

export interface HistoryToolCallContent {
  type: "toolCall";
  id: string;
  name: string;
  arguments: Record<string, unknown>;
}

export type HistoryContent = HistoryTextContent | HistoryToolCallContent;

export type HistoryMessage =
  | {
      role: "user";
      content: [HistoryTextContent];
      timestamp: number;
    }
  | {
      role: "assistant";
      content: HistoryContent[];
      timestamp: number;
    }
  | {
      role: "toolResult";
      content: HistoryTextContent[];
      toolCallId: string;
      toolName: string;
      details?: unknown;
      isError: boolean;
      timestamp: number;
    };

export interface RunnerEnvelope {
  protocol: typeof RUNNER_PROTOCOL;
  version: typeof RUNNER_PROTOCOL_VERSION;
  type: string;
}

export interface RunStartCommand extends RunnerEnvelope {
  type: "run.start";
  requestId: string;
  sessionId: string;
  runId: string;
  message: string;
  history: HistoryMessage[];
  runtimeName: "fake" | "pi";
  toolAccess: {
    baseUrl: string;
    token: string;
    allowedTools: string[];
  };
  pi: {
    agentDir?: string;
    provider?: string;
    model?: string;
    timezone?: string;
    thinkingEffort?: ThinkingEffort;
  };
}

export interface RunAbortCommand extends RunnerEnvelope {
  type: "run.abort";
  requestId: string;
  runId: string;
}

export type RunnerCommand = RunStartCommand | RunAbortCommand;

export interface RunnerReadyEvent extends RunnerEnvelope {
  type: "runner.ready";
  runtime: {
    name: "todai-runner";
    version: "0.3.0";
  };
}

interface RunEvent extends RunnerEnvelope {
  requestId: string;
  sessionId: string;
  runId: string;
  sequence: number;
}

export interface RunStartedEvent extends RunEvent {
  type: "run.started";
  model: string;
  thinkingEffort: ThinkingEffort;
}

export interface AssistantDeltaEvent extends RunEvent {
  type: "assistant.delta";
  messageId: string;
  delta: string;
}

export interface RunCompletedEvent extends RunEvent {
  type: "run.completed";
}

export interface ToolStartedEvent extends RunEvent {
  type: "tool.started";
  toolCallId: string;
  toolName: string;
  arguments: Record<string, unknown>;
}

export interface ToolCompletedEvent extends RunEvent {
  type: "tool.completed";
  toolCallId: string;
  toolName: string;
  result: {
    content: HistoryTextContent[];
    details?: unknown;
  };
  isError: boolean;
}

export interface HistoryMessageEvent extends RunEvent {
  type: "history.message";
  historyMessage: Exclude<HistoryMessage, { role: "user" }>;
}

export interface RunFailedEvent extends RunEvent {
  type: "run.failed";
  error: { code: string; message: string; retryable: boolean };
}

export interface RunAbortedEvent extends RunEvent {
  type: "run.aborted";
  reason: "requested";
}

export interface ProtocolErrorEvent extends RunnerEnvelope {
  type: "protocol.error";
  code: string;
  message: string;
  requestId?: string;
}

export type RunnerOutput =
  | RunnerReadyEvent
  | RunStartedEvent
  | AssistantDeltaEvent
  | ToolStartedEvent
  | ToolCompletedEvent
  | HistoryMessageEvent
  | RunCompletedEvent
  | RunFailedEvent
  | RunAbortedEvent
  | ProtocolErrorEvent;
