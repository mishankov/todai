import { ProtocolError } from "../protocol/codec.js";
import {
  RUNNER_PROTOCOL,
  RUNNER_PROTOCOL_VERSION,
  type RunnerCommand,
  type RunnerOutput,
} from "../protocol/types.js";
import { FakeRunner } from "./fake-runner.js";
import { PiRunner } from "./pi-runner.js";

type Writer = (message: RunnerOutput) => void;

export class Runner {
  readonly #write: Writer;
  readonly #fake: FakeRunner;
  readonly #pi: PiRunner;
  #runtime: "fake" | "pi" = "fake";
  #started = false;

  constructor(write: Writer) {
    this.#write = write;
    this.#fake = new FakeRunner(write);
    this.#pi = new PiRunner(write);
  }

  start(): void {
    if (this.#started) return;
    this.#started = true;
    this.#write({
      protocol: RUNNER_PROTOCOL,
      version: RUNNER_PROTOCOL_VERSION,
      type: "runner.ready",
      runtime: { name: "todai-runner", version: "0.3.0" },
    });
  }

  accept(command: RunnerCommand): void {
    if (command.type === "run.start") this.#runtime = command.runtimeName;
    if (this.#runtime === "pi") this.#pi.accept(command);
    else this.#fake.accept(command);
  }

  reject(error: ProtocolError): void {
    if (this.#runtime === "pi") this.#pi.reject(error);
    else this.#fake.reject(error);
  }
  async close(): Promise<void> {
    this.#fake.close();
    await this.#pi.close();
  }
}
