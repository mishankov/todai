import { ProtocolError } from "../protocol/codec.js";
import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type ProtocolErrorEvent,
  type RunAbortCommand,
  type RunStartCommand,
  type RunnerCommand,
  type RunnerOutput,
} from "../protocol/types.js";

type Writer = (message: RunnerOutput) => void;

interface ActiveRun {
  command: RunStartCommand;
  nextSequence: number;
  timer: NodeJS.Timeout;
}

export class FakeRunner {
  readonly #write: Writer;
  readonly #responseDelayMs: number;
  #activeRun: ActiveRun | undefined;
  #started = false;

  constructor(write: Writer, responseDelayMs = 0) {
    this.#write = write;
    this.#responseDelayMs = responseDelayMs;
  }

  start(): void {
    if (this.#started) {
      return;
    }
    this.#started = true;
    this.#write({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "runner.ready",
      runtime: { name: "todai-runner", version: "0.3.0" },
    });
  }

  accept(command: RunnerCommand): void {
    switch (command.type) {
      case "run.start":
        this.#startRun(command);
        return;
      case "run.abort":
        this.#abortRun(command);
    }
  }

  reject(error: ProtocolError): void {
    const event: ProtocolErrorEvent = {
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "protocol.error",
      code: error.code,
      message: error.message,
    };
    if (error.requestId !== undefined) {
      event.requestId = error.requestId;
    }
    this.#write(event);
  }

  close(): void {
    if (this.#activeRun !== undefined) {
      clearTimeout(this.#activeRun.timer);
      this.#activeRun = undefined;
    }
  }

  #startRun(command: RunStartCommand): void {
    if (this.#activeRun !== undefined) {
      this.reject(
        new ProtocolError(
          "run_active",
          "a run is already active",
          command.requestId,
        ),
      );
      return;
    }

    this.#write({
      ...runEnvelope(command, 1),
      type: "run.started",
    });

    const activeRun: ActiveRun = {
      command,
      nextSequence: 2,
      timer: setTimeout(
        () => this.#completeRun(command.runId),
        this.#responseDelayMs,
      ),
    };
    this.#activeRun = activeRun;
  }

  #completeRun(runId: string): void {
    const activeRun = this.#activeRun;
    if (activeRun === undefined || activeRun.command.runId !== runId) {
      return;
    }

    const response = `Fake response to: ${activeRun.command.message}`;
    this.#write({
      ...runEnvelope(activeRun.command, activeRun.nextSequence++),
      type: "assistant.delta",
      messageId: `fake-message-${activeRun.command.runId}`,
      delta: response,
    });
    this.#write({
      ...runEnvelope(activeRun.command, activeRun.nextSequence++),
      type: "history.message",
      historyMessage: {
        role: "assistant",
        content: [{ type: "text", text: response }],
        timestamp: Date.now(),
      },
    });
    this.#write({
      ...runEnvelope(activeRun.command, activeRun.nextSequence),
      type: "run.completed",
    });
    this.#activeRun = undefined;
  }

  #abortRun(command: RunAbortCommand): void {
    const activeRun = this.#activeRun;
    if (activeRun === undefined || activeRun.command.runId !== command.runId) {
      this.reject(
        new ProtocolError(
          "run_not_active",
          "the requested run is not active",
          command.requestId,
        ),
      );
      return;
    }

    clearTimeout(activeRun.timer);
    this.#write({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "run.aborted",
      requestId: command.requestId,
      sessionId: activeRun.command.sessionId,
      runId: activeRun.command.runId,
      sequence: activeRun.nextSequence,
      reason: "requested",
    });
    this.#activeRun = undefined;
  }
}

function runEnvelope(command: RunStartCommand, sequence: number) {
  return {
    protocol: RUNNER_PROTOCOL,
    version: RUNNER_PROTOCOL_VERSION,
    requestId: command.requestId,
    sessionId: command.sessionId,
    runId: command.runId,
    sequence,
  } as const;
}
