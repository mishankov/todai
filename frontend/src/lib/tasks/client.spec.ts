import { describe, expect, it, vi } from 'vitest';
import {
	createTask,
	createTaskWithProperties,
	createTaskComment,
	deleteTaskComment,
	getAllTasks,
	getInbox,
	getTaskComments,
	getTaskSubtasks,
	getToday,
	TaskConflictError,
	TaskRequestError,
	type Task,
	type TaskComment,
	updateTaskComment
} from './client';

describe('task client relationships', () => {
	it('loads built-in views only inside the selected project', async () => {
		const fetcher = jsonFetcher({ tasks: [] });

		await getInbox(fetcher, 'project/id', true);
		await getToday(fetcher, 'project/id', 'Europe/Moscow', true);
		await getAllTasks(fetcher, 'project/id', true);

		expect(fetcher).toHaveBeenNthCalledWith(
			1,
			'/api/views/projects/project%2Fid/inbox?include_completed=true',
			expect.any(Object)
		);
		expect(fetcher).toHaveBeenNthCalledWith(
			2,
			'/api/views/projects/project%2Fid/today?timezone=Europe%2FMoscow&include_completed=true',
			expect.any(Object)
		);
		expect(fetcher).toHaveBeenNthCalledWith(
			3,
			'/api/views/projects/project%2Fid/all?include_completed=true',
			expect.any(Object)
		);
	});

	it('creates a subtask with its parent identity', async () => {
		const created = testTask({ id: 'child-id', parentId: 'parent/id', title: 'Draft outline' });
		const fetcher = jsonFetcher(created, 201);

		await expect(
			createTask(
				fetcher,
				created.title,
				created.projectId,
				undefined,
				created.parentId ?? undefined
			)
		).resolves.toEqual(created);
		expect(fetcher).toHaveBeenCalledWith('/api/tasks', {
			method: 'POST',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({
				title: created.title,
				projectId: 'project-id',
				parentId: 'parent/id'
			})
		});
	});

	it('persists quick-add properties only when the draft is submitted', async () => {
		const created = testTask({ id: 'created-id', version: 1 });
		const configured = testTask({
			...created,
			priority: 3,
			dueDate: '2026-07-22',
			dueTime: '09:00',
			dueTimezone: 'Europe/Moscow',
			version: 2
		});
		const fetcher = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(created, 201))
			.mockResolvedValueOnce(jsonResponse(configured)) as unknown as typeof fetch;

		await expect(
			createTaskWithProperties(fetcher, {
				title: 'Plan release',
				projectId: 'project-id',
				sectionId: null,
				priority: 3,
				dueDate: '2026-07-22',
				dueTime: '09:00',
				dueTimezone: 'Europe/Moscow'
			})
		).resolves.toEqual(configured);
		expect(fetcher).toHaveBeenCalledTimes(2);
		expect(fetcher).toHaveBeenNthCalledWith(
			2,
			'/api/tasks/created-id',
			expect.objectContaining({
				method: 'PATCH',
				body: JSON.stringify({
					version: 1,
					priority: 3,
					dueDate: '2026-07-22',
					dueTime: '09:00',
					dueTimezone: 'Europe/Moscow'
				})
			})
		);
	});

	it('loads subtasks for the selected task', async () => {
		const subtasks = [testTask({ id: 'child-id', parentId: 'parent/id' })];
		const fetcher = jsonFetcher({ tasks: subtasks });

		await expect(getTaskSubtasks(fetcher, 'parent/id')).resolves.toEqual(subtasks);
		expect(fetcher).toHaveBeenCalledWith('/api/tasks/parent%2Fid/subtasks', {
			credentials: 'same-origin',
			headers: { Accept: 'application/json' }
		});
	});

	it('loads comments for the selected task', async () => {
		const comments = [testComment()];
		const fetcher = jsonFetcher({ comments });

		await expect(getTaskComments(fetcher, 'task/id')).resolves.toEqual(comments);
		expect(fetcher).toHaveBeenCalledWith('/api/tasks/task%2Fid/comments', {
			credentials: 'same-origin',
			headers: { Accept: 'application/json' }
		});
	});

	it('creates a comment independently from task updates', async () => {
		const created = testComment({ body: 'Remember the edge case' });
		const fetcher = jsonFetcher(created, 201);

		await expect(createTaskComment(fetcher, 'task/id', created.body)).resolves.toEqual(created);
		expect(fetcher).toHaveBeenCalledWith('/api/tasks/task%2Fid/comments', {
			method: 'POST',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ body: created.body })
		});
	});

	it('updates a comment with the observed version', async () => {
		const updated = testComment({ body: 'Updated note', version: 2 });
		const fetcher = jsonFetcher(updated);

		await expect(
			updateTaskComment(fetcher, 'task/id', 'comment/id', 1, updated.body)
		).resolves.toEqual(updated);
		expect(fetcher).toHaveBeenCalledWith('/api/tasks/task%2Fid/comments/comment%2Fid', {
			method: 'PATCH',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ version: 1, body: updated.body })
		});
	});

	it('deletes a comment with the observed version', async () => {
		const fetcher = vi.fn(
			async () => new Response(null, { status: 204 })
		) as unknown as typeof fetch;

		await expect(deleteTaskComment(fetcher, 'task/id', 'comment/id', 3)).resolves.toBeUndefined();
		expect(fetcher).toHaveBeenCalledWith('/api/tasks/task%2Fid/comments/comment%2Fid', {
			method: 'DELETE',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ version: 3 })
		});
	});

	it('reports request failures and optimistic conflicts', async () => {
		const failed = statusFetcher(500);
		const conflicted = statusFetcher(409);

		await expect(getTaskSubtasks(failed, 'task-id')).rejects.toBeInstanceOf(TaskRequestError);
		await expect(getTaskComments(failed, 'task-id')).rejects.toBeInstanceOf(TaskRequestError);
		await expect(createTaskComment(failed, 'task-id', 'Note')).rejects.toBeInstanceOf(
			TaskRequestError
		);
		await expect(
			updateTaskComment(conflicted, 'task-id', 'comment-id', 1, 'Note')
		).rejects.toBeInstanceOf(TaskConflictError);
		await expect(deleteTaskComment(conflicted, 'task-id', 'comment-id', 1)).rejects.toBeInstanceOf(
			TaskConflictError
		);
	});
});

function jsonFetcher(body: unknown, status = 200): typeof fetch {
	return vi.fn(async () => jsonResponse(body, status)) as unknown as typeof fetch;
}

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function statusFetcher(status: number): typeof fetch {
	return vi.fn(async () => new Response(null, { status })) as unknown as typeof fetch;
}

function testTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'task-id',
		projectId: 'project-id',
		sectionId: null,
		parentId: null,
		title: 'Subtask',
		description: null,
		status: 'active',
		priority: 0,
		dueDate: null,
		dueTime: null,
		dueTimezone: null,
		position: 1024,
		version: 1,
		completedAt: null,
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testComment(overrides: Partial<TaskComment> = {}): TaskComment {
	return {
		id: 'comment-id',
		taskId: 'task-id',
		body: 'Initial note',
		version: 1,
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z',
		authorId: 'user-id',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
