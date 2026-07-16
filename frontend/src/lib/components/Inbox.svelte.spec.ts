import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Task } from '$lib/tasks/client';
import Inbox from './Inbox.svelte';

describe('Inbox', () => {
	it('creates a task from the quick-add form', async () => {
		const created = testTask({ title: 'Buy milk' });
		const create = vi.fn(async () => created);
		render(Inbox, {
			initialTasks: [],
			create,
			complete: vi.fn(),
			reopen: vi.fn(),
			update: vi.fn(),
			remove: vi.fn()
		});

		await page.getByLabelText('Task title').fill('Buy milk');
		await page.getByRole('button', { name: 'Add task' }).click();

		expect(create).toHaveBeenCalledWith('Buy milk');
		await expect.element(page.getByText('Buy milk')).toBeVisible();
	});

	it('completes, reopens and deletes a task', async () => {
		const active = testTask({ title: 'Write a plan' });
		const completed = testTask({
			...active,
			status: 'completed',
			version: 2,
			completedAt: '2026-07-16T12:00:00Z'
		});
		const reopened = testTask({ ...active, version: 3 });
		const complete = vi.fn(async () => completed);
		const reopen = vi.fn(async () => reopened);
		const remove = vi.fn(async () => {});
		render(Inbox, {
			initialTasks: [active],
			create: vi.fn(),
			complete,
			reopen,
			update: vi.fn(),
			remove
		});

		await page.getByRole('button', { name: 'Complete Write a plan' }).click();
		expect(complete).toHaveBeenCalledWith(active.id);
		await page.getByRole('button', { name: 'Reopen Write a plan' }).click();
		expect(reopen).toHaveBeenCalledWith(active.id);
		await expect.element(page.getByRole('button', { name: 'Complete Write a plan' })).toBeVisible();

		await page.getByRole('button', { name: 'Delete Write a plan' }).click();
		expect(remove).toHaveBeenCalledWith(active.id);
		await expect.element(page.getByText('Inbox clear.')).toBeVisible();
	});

	it('updates task status and removal before the server responds', async () => {
		const active = testTask({ title: 'Review the UI' });
		const completed = testTask({
			...active,
			status: 'completed',
			version: 2,
			completedAt: '2026-07-16T12:00:00Z'
		});
		const completeRequest = deferred<Task>();
		const removeRequest = deferred<void>();
		render(Inbox, {
			initialTasks: [active],
			create: vi.fn(),
			complete: vi.fn(() => completeRequest.promise),
			reopen: vi.fn(),
			update: vi.fn(),
			remove: vi.fn(() => removeRequest.promise)
		});

		await page.getByRole('button', { name: 'Complete Review the UI' }).click();
		await expect.element(page.getByRole('button', { name: 'Reopen Review the UI' })).toBeVisible();

		completeRequest.resolve(completed);
		await page.getByRole('button', { name: 'Delete Review the UI' }).click();
		await expect.element(page.getByText('Inbox clear.')).toBeVisible();

		removeRequest.resolve();
	});

	it('restores optimistic changes when the server rejects them', async () => {
		const active = testTask({ title: 'Keep this task' });
		render(Inbox, {
			initialTasks: [active],
			create: vi.fn(),
			complete: vi.fn(async () => {
				throw new Error('complete failed');
			}),
			reopen: vi.fn(),
			update: vi.fn(),
			remove: vi.fn(async () => {
				throw new Error('delete failed');
			})
		});

		await page.getByRole('button', { name: 'Complete Keep this task' }).click();
		await expect
			.element(page.getByText('The task could not be updated. Please try again.'))
			.toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Complete Keep this task' }))
			.toBeVisible();

		await page.getByRole('button', { name: 'Delete Keep this task' }).click();
		await expect
			.element(page.getByText('The task could not be deleted. Please try again.'))
			.toBeVisible();
		await expect.element(page.getByText('Keep this task')).toBeVisible();
	});

	it('edits task fields with the observed version', async () => {
		const active = testTask({ title: 'Draft plan' });
		const updated = testTask({
			...active,
			title: 'Publish plan',
			description: 'Share with the team',
			priority: 3,
			version: 2
		});
		const update = vi.fn(async () => updated);
		render(Inbox, {
			initialTasks: [active],
			create: vi.fn(),
			complete: vi.fn(),
			reopen: vi.fn(),
			update,
			remove: vi.fn()
		});

		await page.getByRole('button', { name: 'Edit Draft plan' }).click();
		await page.getByLabelText('Title', { exact: true }).fill('Publish plan');
		await page.getByLabelText('Description').fill('Share with the team');
		await page.getByLabelText('Priority').selectOptions('3');
		await page.getByRole('button', { name: 'Save changes' }).click();

		expect(update).toHaveBeenCalledWith(
			active.id,
			expect.objectContaining({
				version: active.version,
				title: 'Publish plan',
				description: 'Share with the team',
				priority: 3
			})
		);
		await expect.element(page.getByText('Publish plan')).toBeVisible();
	});
});

function testTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'task-id',
		projectId: null,
		parentId: null,
		title: 'Task',
		description: null,
		status: 'active',
		priority: 0,
		dueAt: null,
		dueTimezone: null,
		position: 1024,
		version: 1,
		completedAt: null,
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function deferred<T>() {
	let resolve: (value: T | PromiseLike<T>) => void = () => {};
	const promise = new Promise<T>((promiseResolve) => {
		resolve = promiseResolve;
	});

	return { promise, resolve };
}
