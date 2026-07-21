import type { Task, TaskStatus } from '$lib/tasks/client';

export class TaskSearchError extends Error {
	constructor() {
		super('Could not search tasks.');
		this.name = 'TaskSearchError';
	}
}

export async function searchTasks(
	fetcher: typeof fetch,
	query: string,
	projectId: string,
	options: { status?: TaskStatus; limit?: number; signal?: AbortSignal } = {}
): Promise<Task[]> {
	const parameters = new URLSearchParams({
		query,
		project_id: projectId,
		limit: String(options.limit ?? 20)
	});
	if (options.status) parameters.set('status', options.status);
	const response = await fetcher(`/api/tasks/search?${parameters}`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' },
		signal: options.signal
	});
	if (!response.ok) throw new TaskSearchError();
	const body = (await response.json()) as { tasks: Task[] };
	return body.tasks;
}
