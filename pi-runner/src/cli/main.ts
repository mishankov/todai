import {
  ProtocolError,
  decodeCommand,
  encodeMessage,
} from "../protocol/codec.js";
import { registerBunOAuthFlows } from "@earendil-works/pi-ai/bun-oauth";
import { FramingError, JsonlFramer } from "../protocol/framing.js";
import { Runner } from "../runtime/runner.js";

registerBunOAuthFlows();

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

let shuttingDown = false;

process.on("SIGTERM", () => void shutdown(0));
process.on("SIGINT", () => void shutdown(0));

async function shutdown(exitCode: number): Promise<void> {
  if (shuttingDown) return;
  shuttingDown = true;
  process.stdin.destroy();
  await runner.close();
  process.exitCode = exitCode;
}

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
  process.stdin.pause();
  void shutdown(1);
}
