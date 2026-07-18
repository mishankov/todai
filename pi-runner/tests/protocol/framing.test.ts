import { describe, expect, it } from "vitest";

import { FramingError, JsonlFramer } from "../../src/protocol/framing.js";

describe("JSONL framing", () => {
  it("reassembles partial UTF-8 input and drains multiple lines", () => {
    const framer = new JsonlFramer();
    const input = Buffer.from('{"message":"Привет"}\n{"type":"next"}\r\n');
    const splitInsideUtf8Character = input.indexOf(Buffer.from("П")) + 1;

    expect(framer.push(input.subarray(0, splitInsideUtf8Character))).toEqual(
      [],
    );
    expect(framer.push(input.subarray(splitInsideUtf8Character))).toEqual([
      '{"message":"Привет"}',
      '{"type":"next"}',
    ]);
    expect(framer.finish()).toEqual([]);
  });

  it("returns a final record without a trailing newline", () => {
    const framer = new JsonlFramer();

    expect(framer.push(Buffer.from('{"type":"final"}'))).toEqual([]);
    expect(framer.finish()).toEqual(['{"type":"final"}']);
  });

  it("rejects a record larger than the configured limit", () => {
    const framer = new JsonlFramer(4);

    expect(() => framer.push(Buffer.from("12345"))).toThrow(FramingError);
  });
});
