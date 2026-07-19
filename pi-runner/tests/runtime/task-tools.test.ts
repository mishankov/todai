import { createServer } from "node:http";
import type { AddressInfo } from "node:net";

import { describe, expect, it } from "vitest";

import { callTaskTool, createTaskTools } from "../../src/runtime/task-tools.js";

describe("task tool client", () => {
  it("exposes project_get as a read-only project and section lookup", () => {
    const [tool] = createTaskTools({
      baseUrl: "http://127.0.0.1:8080",
      token: "secret-token",
      allowedTools: ["project_get"],
    });

    expect(tool?.name).toBe("project_get");
    expect(tool?.description).toContain("ordered sections");
    expect(tool?.executionMode).toBe("parallel");
  });

  it("sends the scoped bearer token only to the selected internal tool", async () => {
    let requestPath = "";
    let authorization = "";
    let requestBody = "";
    const server = createServer((request, response) => {
      requestPath = request.url ?? "";
      authorization = request.headers.authorization ?? "";
      request.setEncoding("utf8");
      request.on("data", (chunk: string) => {
        requestBody += chunk;
      });
      request.on("end", () => {
        response.writeHead(200, { "content-type": "application/json" });
        response.end('{"task":{"id":"task-1"}}');
      });
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    const address = server.address() as AddressInfo;

    try {
      const result = await callTaskTool(
        {
          baseUrl: `http://127.0.0.1:${address.port}`,
          token: "secret-token",
          allowedTools: ["task_get"],
        },
        "task_get",
        { taskId: "task-1" },
      );

      expect(requestPath).toBe("/internal/tools/task_get");
      expect(authorization).toBe("Bearer secret-token");
      expect(JSON.parse(requestBody)).toEqual({ taskId: "task-1" });
      expect(result.content[0]?.text).toContain("task-1");
    } finally {
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    }
  });

  it("does not include the bearer token in HTTP errors", async () => {
    const server = createServer((_request, response) => {
      response.writeHead(409);
      response.end("version conflict");
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    const address = server.address() as AddressInfo;

    try {
      const result = callTaskTool(
        {
          baseUrl: `http://127.0.0.1:${address.port}`,
          token: "secret-token",
          allowedTools: ["task_update"],
        },
        "task_update",
        { taskId: "task-1", version: 1 },
      );
      await expect(result).rejects.toThrow("HTTP 409");
      await expect(result).rejects.not.toThrow("secret-token");
    } finally {
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    }
  });
});
