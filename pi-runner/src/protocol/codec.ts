import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type HistoryTextContent,
  type RunStartCommand,
  type RunnerCommand,
  type RunnerOutput,
} from "./types.js";

export class ProtocolError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly requestId?: string,
  ) {
    super(message);
    this.name = "ProtocolError";
  }
}

export function decodeCommand(line: string): RunnerCommand {
  let value: unknown;

  try {
    value = JSON.parse(line);
  } catch {
    throw new ProtocolError("invalid_json", "command is not valid JSON");
  }

  if (!isRecord(value)) {
    throw new ProtocolError("invalid_command", "command must be a JSON object");
  }

  const requestId = optionalString(value, "requestId");

  if (value.protocol !== RUNNER_PROTOCOL) {
    throw new ProtocolError(
      "unsupported_protocol",
      `protocol must be ${RUNNER_PROTOCOL}`,
      requestId,
    );
  }
  if (value.version !== RUNNER_PROTOCOL_VERSION) {
    throw new ProtocolError(
      "unsupported_version",
      `version must be ${RUNNER_PROTOCOL_VERSION}`,
      requestId,
    );
  }

  switch (value.type) {
    case "run.start": {
      const runtimeName = requiredString(value, "runtimeName", requestId);
      if (runtimeName !== "fake" && runtimeName !== "pi") {
        throw new ProtocolError(
          "invalid_command",
          "runtimeName must be fake or pi",
          requestId,
        );
      }
      return {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.start",
        requestId: requiredString(value, "requestId", requestId),
        sessionId: requiredString(value, "sessionId", requestId),
        runId: requiredString(value, "runId", requestId),
        message: requiredString(value, "message", requestId),
        history: requiredHistory(value, requestId),
        runtimeName,
        toolAccess: requiredToolAccess(value, requestId),
        pi: optionalPiConfig(value, requestId),
      };
    }
    case "run.abort":
      return {
        protocol: RUNNER_PROTOCOL,
        version: RUNNER_PROTOCOL_VERSION,
        type: "run.abort",
        requestId: requiredString(value, "requestId", requestId),
        runId: requiredString(value, "runId", requestId),
      };
    default:
      throw new ProtocolError(
        "unknown_command",
        "command type is not supported",
        requestId,
      );
  }
}

function requiredHistory(
  value: Record<string, unknown>,
  requestId?: string,
): RunStartCommand["history"] {
  if (!Array.isArray(value.history)) {
    throw new ProtocolError(
      "invalid_command",
      "history must be an array",
      requestId,
    );
  }
  return value.history.map((message) =>
    decodeHistoryMessage(message, requestId),
  );
}

function decodeHistoryMessage(
  value: unknown,
  requestId?: string,
): RunStartCommand["history"][number] {
  if (!isRecord(value)) {
    throw new ProtocolError(
      "invalid_command",
      "history message must be an object",
      requestId,
    );
  }
  const role = requiredString(value, "role", requestId);
  const timestamp = value.timestamp;
  if (!Number.isSafeInteger(timestamp) || (timestamp as number) < 1) {
    throw new ProtocolError(
      "invalid_command",
      "history message timestamp must be a positive integer",
      requestId,
    );
  }
  if (!Array.isArray(value.content)) {
    throw new ProtocolError(
      "invalid_command",
      "history message content must be an array",
      requestId,
    );
  }
  const content = value.content.map((item) =>
    decodeHistoryContent(item, requestId),
  );
  if (role === "user") {
    if (content.length !== 1 || content[0]?.type !== "text") {
      throw new ProtocolError(
        "invalid_command",
        "user history must contain one text block",
        requestId,
      );
    }
    return { role, content: [content[0]], timestamp: timestamp as number };
  }
  if (role === "assistant") {
    if (content.length === 0) {
      throw new ProtocolError(
        "invalid_command",
        "assistant history content must not be empty",
        requestId,
      );
    }
    return { role, content, timestamp: timestamp as number };
  }
  if (role === "toolResult") {
    const textContent = content.filter(
      (item): item is HistoryTextContent => item.type === "text",
    );
    if (textContent.length !== content.length) {
      throw new ProtocolError(
        "invalid_command",
        "tool result history may contain only text",
        requestId,
      );
    }
    return {
      role,
      content: textContent,
      toolCallId: requiredString(value, "toolCallId", requestId),
      toolName: requiredString(value, "toolName", requestId),
      ...(value.details === undefined ? {} : { details: value.details }),
      isError: value.isError === true,
      timestamp: timestamp as number,
    };
  }
  throw new ProtocolError(
    "invalid_command",
    "history message role is not supported",
    requestId,
  );
}

function decodeHistoryContent(
  value: unknown,
  requestId?: string,
): RunStartCommand["history"][number]["content"][number] {
  if (!isRecord(value)) {
    throw new ProtocolError(
      "invalid_command",
      "history content must be an object",
      requestId,
    );
  }
  if (value.type === "text") {
    return { type: "text", text: requiredString(value, "text", requestId) };
  }
  if (value.type === "toolCall") {
    if (!isRecord(value.arguments)) {
      throw new ProtocolError(
        "invalid_command",
        "tool call arguments must be an object",
        requestId,
      );
    }
    return {
      type: "toolCall",
      id: requiredString(value, "id", requestId),
      name: requiredString(value, "name", requestId),
      arguments: value.arguments,
    };
  }
  throw new ProtocolError(
    "invalid_command",
    "history content type is not supported",
    requestId,
  );
}

function requiredToolAccess(
  value: Record<string, unknown>,
  requestId?: string,
): RunStartCommand["toolAccess"] {
  const access = value.toolAccess;
  if (!isRecord(access)) {
    throw new ProtocolError(
      "invalid_command",
      "toolAccess must be an object",
      requestId,
    );
  }
  const baseUrl = requiredString(access, "baseUrl", requestId);
  try {
    const parsed = new URL(baseUrl);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:")
      throw new Error();
  } catch {
    throw new ProtocolError(
      "invalid_command",
      "toolAccess.baseUrl must be an HTTP(S) URL",
      requestId,
    );
  }
  const allowedTools = access.allowedTools;
  if (
    !Array.isArray(allowedTools) ||
    allowedTools.length === 0 ||
    allowedTools.some((tool) => typeof tool !== "string" || tool.length === 0)
  ) {
    throw new ProtocolError(
      "invalid_command",
      "toolAccess.allowedTools must contain tool names",
      requestId,
    );
  }
  return {
    baseUrl,
    token: requiredString(access, "token", requestId),
    allowedTools,
  };
}

function optionalPiConfig(
  value: Record<string, unknown>,
  requestId?: string,
): RunStartCommand["pi"] {
  const pi = value.pi;
  if (pi === undefined) return {};
  if (!isRecord(pi)) {
    throw new ProtocolError(
      "invalid_command",
      "pi must be an object",
      requestId,
    );
  }
  const agentDir = optionalString(pi, "agentDir");
  const provider = optionalString(pi, "provider");
  const model = optionalString(pi, "model");
  const timezone = optionalString(pi, "timezone");
  return {
    ...(agentDir === undefined ? {} : { agentDir }),
    ...(provider === undefined ? {} : { provider }),
    ...(model === undefined ? {} : { model }),
    ...(timezone === undefined ? {} : { timezone }),
  };
}

export function encodeMessage(message: RunnerOutput): string {
  return `${JSON.stringify(message)}\n`;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function requiredString(
  value: Record<string, unknown>,
  field: string,
  requestId?: string,
): string {
  const result = optionalString(value, field);
  if (result === undefined || result.length === 0) {
    throw new ProtocolError(
      "invalid_command",
      `${field} must be a non-empty string`,
      requestId,
    );
  }
  return result;
}

function optionalString(
  value: Record<string, unknown>,
  field: string,
): string | undefined {
  const result = value[field];
  return typeof result === "string" ? result : undefined;
}
