import {
  defineTool,
  type ToolDefinition,
} from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";

import type { RunStartCommand } from "../protocol/types.js";

const nullableString = Type.Union([Type.String(), Type.Null()]);
const version = Type.Integer({ minimum: 1 });

const definitions = {
  task_get: ["Get one task by ID", Type.Object({ taskId: Type.String() })],
  task_search: [
    "Search tasks",
    Type.Object({
      query: Type.String(),
      projectId: Type.Optional(Type.String()),
      status: Type.Optional(Type.String()),
      limit: Type.Optional(Type.Integer({ minimum: 1 })),
    }),
  ],
  project_list: [
    "List projects",
    Type.Object({ includeArchived: Type.Optional(Type.Boolean()) }),
  ],
  view_query: [
    "List tasks from inbox, all, project, or today view",
    Type.Object({
      view: Type.String(),
      projectId: Type.Optional(Type.String()),
      timezone: Type.Optional(Type.String()),
      includeCompleted: Type.Optional(Type.Boolean()),
    }),
  ],
  task_create: [
    "Create a task",
    Type.Object({
      title: Type.String(),
      projectId: Type.Optional(nullableString),
      sectionId: Type.Optional(nullableString),
      parentId: Type.Optional(Type.String()),
    }),
  ],
  task_update: [
    "Update task fields using its current version",
    Type.Object({
      taskId: Type.String(),
      version,
      title: Type.Optional(Type.String()),
      description: Type.Optional(nullableString),
      priority: Type.Optional(Type.Integer({ minimum: 0, maximum: 4 })),
      dueDate: Type.Optional(nullableString),
      dueTime: Type.Optional(nullableString),
      dueTimezone: Type.Optional(nullableString),
    }),
  ],
  task_complete: [
    "Complete a task",
    Type.Object({ taskId: Type.String(), version }),
  ],
  task_reopen: [
    "Reopen a completed task",
    Type.Object({ taskId: Type.String(), version }),
  ],
  task_move: [
    "Move a task to a project or section",
    Type.Object({
      taskId: Type.String(),
      version,
      projectId: Type.Optional(nullableString),
      sectionId: Type.Optional(nullableString),
    }),
  ],
  task_reorder: [
    "Reorder a task within a section",
    Type.Object({
      taskId: Type.String(),
      version,
      sectionId: Type.Optional(nullableString),
      beforeTaskId: Type.Optional(nullableString),
    }),
  ],
} as const;

const mutationTools = new Set([
  "task_create",
  "task_update",
  "task_complete",
  "task_reopen",
  "task_move",
  "task_reorder",
]);

export function createTaskTools(
  access: RunStartCommand["toolAccess"],
): ToolDefinition[] {
  return access.allowedTools.map((name) => {
    const definition = definitions[name as keyof typeof definitions];
    if (definition === undefined)
      throw new Error(`unsupported allowed tool: ${name}`);
    const [description, parameters] = definition;
    return defineTool({
      name,
      label: name,
      description,
      parameters,
      executionMode: mutationTools.has(name) ? "sequential" : "parallel",
      execute: async (_toolCallId, input, signal) =>
        callTaskTool(access, name, input, signal),
    });
  });
}

export async function callTaskTool(
  access: RunStartCommand["toolAccess"],
  name: string,
  input: unknown,
  signal?: AbortSignal,
) {
  const response = await fetch(`${access.baseUrl}/internal/tools/${name}`, {
    method: "POST",
    headers: {
      authorization: `Bearer ${access.token}`,
      "content-type": "application/json",
    },
    body: JSON.stringify(input),
    ...(signal === undefined ? {} : { signal }),
  });
  const body = await response.text();
  if (!response.ok)
    throw new Error(
      `tool ${name} failed with HTTP ${response.status}: ${body.slice(0, 500)}`,
    );
  return {
    content: [{ type: "text" as const, text: body }],
    details: { status: response.status },
  };
}
