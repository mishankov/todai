export type TaskStatus = 'active' | 'completed';

export interface Task {
	id: string;
	projectId: string;
	sectionId: string | null;
	parentId: string | null;
	title: string;
	description: string | null;
	status: TaskStatus;
	priority: number;
	dueDate: string | null;
	dueTime: string | null;
	dueTimezone: string | null;
	position: number;
	version: number;
	completedAt: string | null;
	createdAt: string;
	updatedAt: string;
	lastModifiedBy: string;
}

export interface TaskSummary extends Task {
	subtaskCount: number;
	completedSubtaskCount: number;
}

export interface TaskUpdate {
	version: number;
	title?: string;
	description?: string | null;
	projectId?: string;
	sectionId?: string | null;
	priority?: number;
	dueDate?: string | null;
	dueTime?: string | null;
	dueTimezone?: string | null;
}

export interface TaskCreateDraft {
	title: string;
	description?: string | null;
	projectId: string;
	sectionId: string | null;
	priority: number;
	dueDate: string | null;
	dueTime: string | null;
	dueTimezone: string | null;
}

export interface TaskComment {
	id: string;
	taskId: string;
	authorId: string;
	body: string;
	version: number;
	createdAt: string;
	updatedAt: string;
	lastModifiedBy: string;
}

export class TaskRequestError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'TaskRequestError';
	}
}

export class TaskConflictError extends TaskRequestError {
	constructor() {
		super('The task changed after it was opened.');
		this.name = 'TaskConflictError';
	}
}

export async function getTask(fetcher: typeof fetch, taskId: string): Promise<Task> {
	const response = await fetcher(`/api/tasks/${encodeURIComponent(taskId)}`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new TaskRequestError('Could not load the task.');
	return (await response.json()) as Task;
}

export async function getInbox(
	fetcher: typeof fetch,
	projectId: string,
	includeCompleted = false
): Promise<TaskSummary[]> {
	const query = new URLSearchParams({ include_completed: String(includeCompleted) });
	return getTaskView(
		fetcher,
		`/api/views/projects/${encodeURIComponent(projectId)}/inbox?${query}`,
		'Could not load Inbox.'
	);
}

export async function getAllTasks(
	fetcher: typeof fetch,
	projectId: string,
	includeCompleted = false
): Promise<TaskSummary[]> {
	const query = new URLSearchParams({ include_completed: String(includeCompleted) });
	return getTaskView(
		fetcher,
		`/api/views/projects/${encodeURIComponent(projectId)}/all?${query}`,
		'Could not load all tasks.'
	);
}

export async function getToday(
	fetcher: typeof fetch,
	projectId: string,
	timezone: string,
	includeCompleted = false
): Promise<TaskSummary[]> {
	const query = new URLSearchParams({
		timezone,
		include_completed: String(includeCompleted)
	});
	return getTaskView(
		fetcher,
		`/api/views/projects/${encodeURIComponent(projectId)}/today?${query}`,
		'Could not load Today.'
	);
}

export async function getProjectTasks(
	fetcher: typeof fetch,
	projectId: string,
	includeCompleted = false
): Promise<TaskSummary[]> {
	const query = new URLSearchParams({ include_completed: String(includeCompleted) });
	return getTaskView(
		fetcher,
		`/api/views/projects/${encodeURIComponent(projectId)}?${query}`,
		'Could not load project tasks.'
	);
}

async function getTaskView(
	fetcher: typeof fetch,
	path: string,
	errorMessage: string
): Promise<TaskSummary[]> {
	const response = await fetcher(path, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) {
		throw new TaskRequestError(errorMessage);
	}

	const body = (await response.json()) as { tasks: TaskSummary[] };
	return body.tasks;
}

export async function createTask(
	fetcher: typeof fetch,
	title: string,
	projectId: string,
	sectionId?: string,
	parentId?: string
): Promise<Task> {
	return sendTaskRequest(
		fetcher,
		'/api/tasks',
		{
			title,
			projectId,
			...(sectionId === undefined ? {} : { sectionId }),
			...(parentId === undefined ? {} : { parentId })
		},
		'Could not create the task.'
	);
}

export async function createTaskWithProperties(
	fetcher: typeof fetch,
	draft: TaskCreateDraft
): Promise<Task> {
	return sendTaskRequest(
		fetcher,
		'/api/tasks',
		{
			title: draft.title,
			description: draft.description ?? null,
			projectId: draft.projectId,
			sectionId: draft.sectionId,
			priority: draft.priority,
			dueDate: draft.dueDate,
			dueTime: draft.dueDate ? draft.dueTime : null,
			dueTimezone: draft.dueDate && draft.dueTime ? draft.dueTimezone : null
		},
		'Could not create the task.'
	);
}

export async function getTaskSubtasks(fetcher: typeof fetch, taskId: string): Promise<Task[]> {
	const response = await fetcher(`/api/tasks/${encodeURIComponent(taskId)}/subtasks`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new TaskRequestError('Could not load subtasks.');
	const body = (await response.json()) as { tasks: Task[] };
	return body.tasks;
}

export async function getTaskComments(
	fetcher: typeof fetch,
	taskId: string
): Promise<TaskComment[]> {
	const response = await fetcher(`/api/tasks/${encodeURIComponent(taskId)}/comments`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new TaskRequestError('Could not load comments.');
	const body = (await response.json()) as { comments: TaskComment[] };
	return body.comments;
}

export async function createTaskComment(
	fetcher: typeof fetch,
	taskId: string,
	body: string
): Promise<TaskComment> {
	const response = await fetcher(`/api/tasks/${encodeURIComponent(taskId)}/comments`, {
		method: 'POST',
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify({ body })
	});
	if (!response.ok) throw new TaskRequestError('Could not add the comment.');
	return (await response.json()) as TaskComment;
}

export async function updateTaskComment(
	fetcher: typeof fetch,
	taskId: string,
	commentId: string,
	version: number,
	body: string
): Promise<TaskComment> {
	const response = await fetcher(
		`/api/tasks/${encodeURIComponent(taskId)}/comments/${encodeURIComponent(commentId)}`,
		{
			method: 'PATCH',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ version, body })
		}
	);
	if (response.status === 409) throw new TaskConflictError();
	if (!response.ok) throw new TaskRequestError('Could not update the comment.');
	return (await response.json()) as TaskComment;
}

export async function deleteTaskComment(
	fetcher: typeof fetch,
	taskId: string,
	commentId: string,
	version: number
): Promise<void> {
	const response = await fetcher(
		`/api/tasks/${encodeURIComponent(taskId)}/comments/${encodeURIComponent(commentId)}`,
		{
			method: 'DELETE',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ version })
		}
	);
	if (response.status === 409) throw new TaskConflictError();
	if (!response.ok) throw new TaskRequestError('Could not delete the comment.');
}

export async function reorderTask(
	fetcher: typeof fetch,
	taskId: string,
	version: number,
	sectionId: string | null,
	beforeTaskId: string | null
): Promise<TaskSummary[]> {
	const response = await fetcher(`/api/tasks/${encodeURIComponent(taskId)}/reorder`, {
		method: 'POST',
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify({ version, sectionId, beforeTaskId })
	});
	if (response.status === 409) throw new TaskConflictError();
	if (!response.ok) throw new TaskRequestError('Could not reorder the task.');
	const body = (await response.json()) as { tasks: TaskSummary[] };
	return body.tasks;
}

export async function updateTask(
	fetcher: typeof fetch,
	taskId: string,
	update: TaskUpdate
): Promise<Task> {
	const response = await fetcher(`/api/tasks/${encodeURIComponent(taskId)}`, {
		method: 'PATCH',
		credentials: 'same-origin',
		headers: {
			Accept: 'application/json',
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(update)
	});
	if (response.status === 409) {
		throw new TaskConflictError();
	}
	if (!response.ok) {
		throw new TaskRequestError('Could not update the task.');
	}

	return (await response.json()) as Task;
}

export async function completeTask(
	fetcher: typeof fetch,
	taskId: string,
	version: number
): Promise<Task> {
	return sendTaskRequest(
		fetcher,
		`/api/tasks/${encodeURIComponent(taskId)}/complete`,
		{ version },
		'Could not complete the task.'
	);
}

export async function reopenTask(
	fetcher: typeof fetch,
	taskId: string,
	version: number
): Promise<Task> {
	return sendTaskRequest(
		fetcher,
		`/api/tasks/${encodeURIComponent(taskId)}/reopen`,
		{ version },
		'Could not reopen the task.'
	);
}

export async function deleteTask(
	fetcher: typeof fetch,
	taskId: string,
	version: number
): Promise<void> {
	const response = await fetcher(`/api/tasks/${encodeURIComponent(taskId)}`, {
		method: 'DELETE',
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify({ version })
	});
	if (response.status === 409) throw new TaskConflictError();
	if (!response.ok) {
		throw new TaskRequestError('Could not delete the task.');
	}
}

async function sendTaskRequest(
	fetcher: typeof fetch,
	path: string,
	body: object | undefined,
	errorMessage: string
): Promise<Task> {
	const response = await fetcher(path, {
		method: 'POST',
		credentials: 'same-origin',
		headers: {
			Accept: 'application/json',
			...(body === undefined ? {} : { 'Content-Type': 'application/json' })
		},
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	if (response.status === 409) throw new TaskConflictError();
	if (!response.ok) {
		throw new TaskRequestError(errorMessage);
	}

	return (await response.json()) as Task;
}
