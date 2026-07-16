import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Task } from '$lib/tasks/client';
import Today from './Today.svelte';

describe('Today', () => {
	it('shows due time, priority and the number of remaining tasks', async () => {
		const dueAt = new Date();
		dueAt.setHours(23, 59, 0, 0);
		render(Today, {
			initialTasks: [testTask({ title: 'Ship Today', priority: 4, dueAt: dueAt.toISOString() })],
			complete: vi.fn(),
			reopen: vi.fn(),
			update: vi.fn(),
			remove: vi.fn()
		});

		await expect.element(page.getByRole('heading', { name: 'Today', level: 1 })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'Today', level: 2 })).toBeVisible();
		await expect.element(page.getByText('1 remaining')).toBeVisible();
		await expect.element(page.getByText(/23:59|11:59/)).toBeVisible();
		await expect.element(page.getByText('Urgent')).toBeVisible();
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
