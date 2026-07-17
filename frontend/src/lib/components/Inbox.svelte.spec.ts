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

	it('opens task editing in the shared modal', async () => {
		const task = testTask({ title: 'Draft plan' });
		render(Inbox, {
			initialTasks: [task],
			create: vi.fn(),
			complete: vi.fn(),
			reopen: vi.fn(),
			update: vi.fn(),
			remove: vi.fn()
		});

		await page.getByRole('button', { name: `Open ${task.title}` }).click();

		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.toHaveAttribute('aria-modal', 'true');
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
		await page.getByLabelText('Due date').fill('2026-07-20');
		await page.getByRole('button', { name: 'Save changes' }).click();

		expect(update).toHaveBeenCalledWith(
			active.id,
			expect.objectContaining({
				version: active.version,
				title: 'Publish plan',
				description: 'Share with the team',
				priority: 3,
				dueDate: '2026-07-20',
				dueTime: null,
				dueTimezone: null
			})
		);
		await expect.element(page.getByText('Publish plan')).toBeVisible();
	});

	it('groups tasks by their planned date', async () => {
		const today = startOfDay(new Date());
		const yesterday = addDays(today, -1);
		const tomorrow = addDays(today, 1);
		const completed = testTask({
			id: 'completed',
			title: 'Already done',
			status: 'completed',
			completedAt: new Date().toISOString()
		});
		render(Inbox, {
			initialTasks: [
				testTask({ id: 'overdue', title: 'Overdue task', dueDate: dateValue(yesterday) }),
				testTask({ id: 'today', title: 'Today task', dueDate: dateValue(today) }),
				testTask({ id: 'tomorrow', title: 'Tomorrow task', dueDate: dateValue(tomorrow) }),
				testTask({ id: 'undated', title: 'Undated task' }),
				completed
			],
			create: vi.fn(),
			complete: vi.fn(),
			reopen: vi.fn(),
			update: vi.fn(),
			remove: vi.fn()
		});

		await expect.element(page.getByRole('heading', { name: /Overdue/ })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'Today' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'Tomorrow' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'No date' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'Completed' })).toBeVisible();
	});
});

function startOfDay(value: Date): Date {
	return new Date(value.getFullYear(), value.getMonth(), value.getDate());
}

function addDays(value: Date, days: number): Date {
	const result = new Date(value);
	result.setDate(result.getDate() + days);
	return result;
}

function dateValue(value: Date): string {
	const year = value.getFullYear();
	const month = String(value.getMonth() + 1).padStart(2, '0');
	const day = String(value.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

function testTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'task-id',
		projectId: null,
		sectionId: null,
		parentId: null,
		title: 'Task',
		description: null,
		status: 'active',
		priority: 0,
		dueDate: null,
		dueTime: null,
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
