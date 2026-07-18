import {
  ProtocolError,
  decodeCommand,
  encodeMessage,
} from "../protocol/codec.js";
import { FramingError, JsonlFramer } from "../protocol/framing.js";
import { Runner } from "../runtime/runner.js";

const framer = new JsonlFramer();
const runner = new Runner((message) =>
  process.stdout.write(encodeMessage(message)),
);

runner.start();
console.error("runner ready");

process.stdin.on("data", (chunk: Buffer) => {
  try {
    for (const line of framer.push(chunk)) {
      acceptLine(line);
    }
  } catch (error) {
    failFraming(error);
  }
});

process.stdin.on("end", () => {
  try {
    for (const line of framer.finish()) {
      acceptLine(line);
    }
  } catch (error) {
    failFraming(error);
  }
});

process.on("SIGTERM", () => {
  runner.close();
  process.stdin.destroy();
  process.exitCode = 0;
});

function acceptLine(line: string): void {
  try {
    runner.accept(decodeCommand(line));
  } catch (error) {
    const protocolError =
      error instanceof ProtocolError
        ? error
        : new ProtocolError("internal_error", "failed to process command");
    console.error(`${protocolError.code}: ${protocolError.message}`);
    runner.reject(protocolError);
  }
}

function failFraming(error: unknown): void {
  const framingError =
    error instanceof FramingError
      ? error
      : new FramingError("failed to read JSONL input");
  console.error(`invalid_frame: ${framingError.message}`);
  runner.reject(new ProtocolError("invalid_frame", framingError.message));
  runner.close();
  process.stdin.pause();
  process.exitCode = 1;
}
