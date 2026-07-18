import { StringDecoder } from "node:string_decoder";

const DEFAULT_MAX_LINE_BYTES = 1024 * 1024;

export class FramingError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "FramingError";
  }
}

export class JsonlFramer {
  readonly #decoder = new StringDecoder("utf8");
  readonly #maxLineBytes: number;
  #buffer = "";

  constructor(maxLineBytes = DEFAULT_MAX_LINE_BYTES) {
    this.#maxLineBytes = maxLineBytes;
  }

  push(chunk: Uint8Array): string[] {
    this.#buffer += this.#decoder.write(Buffer.from(chunk));
    return this.#drainCompleteLines();
  }

  finish(): string[] {
    this.#buffer += this.#decoder.end();
    const lines = this.#drainCompleteLines();

    if (this.#buffer.length > 0) {
      this.#assertLineSize(this.#buffer);
      lines.push(stripCarriageReturn(this.#buffer));
      this.#buffer = "";
    }

    return lines;
  }

  #drainCompleteLines(): string[] {
    const lines: string[] = [];
    let newlineIndex = this.#buffer.indexOf("\n");

    while (newlineIndex >= 0) {
      const line = this.#buffer.slice(0, newlineIndex);
      this.#assertLineSize(line);
      lines.push(stripCarriageReturn(line));
      this.#buffer = this.#buffer.slice(newlineIndex + 1);
      newlineIndex = this.#buffer.indexOf("\n");
    }

    this.#assertLineSize(this.#buffer);
    return lines;
  }

  #assertLineSize(line: string): void {
    if (Buffer.byteLength(line, "utf8") > this.#maxLineBytes) {
      throw new FramingError(`JSONL line exceeds ${this.#maxLineBytes} bytes`);
    }
  }
}

function stripCarriageReturn(line: string): string {
  return line.endsWith("\r") ? line.slice(0, -1) : line;
}
