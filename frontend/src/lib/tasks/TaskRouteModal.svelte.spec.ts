import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Project } from '$lib/projects/client';
import type { Task } from './client';
import TaskRouteModal from './TaskRouteModal.svelte';

describe('TaskRouteModal', () => {
	it('shows an accessible loading state before opening the shared editor', async () => {
		const task = testTask({ status: 'completed', completedAt: '2026-07-22T11:00:00Z' });
		const request = deferred<Task>();
		render(TaskRouteModal, {
			projects: [testProject()],
			routeOverride: { projectId: task.projectId, taskId: task.id },
			loadTask: vi.fn(() => request.promise),
			loadSections: vi.fn(async () => []),
			closeRoute: vi.fn()
		});

		await expect
			.element(page.getByRole('dialog', { name: 'Loading task editor' }))
			.toHaveAttribute('aria-modal', 'true');
		await expect
			.element(page.getByText('Loading task…', { exact: true }))
			.toHaveAttribute('role', 'status');

		request.resolve(task);
		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.toBeVisible();
	});

	it('shows a safe error state without a partial editor', async () => {
		const closeRoute = vi.fn();
		const route = { projectId: 'project-id', taskId: 'missing-task' };
		render(TaskRouteModal, {
			projects: [testProject()],
			routeOverride: route,
			loadTask: vi.fn(async () => {
				throw new Error('not found');
			}),
			closeRoute
		});

		await expect
			.element(page.getByRole('dialog', { name: 'Task unavailable' }))
			.toHaveAttribute('aria-modal', 'true');
		await expect.element(page.getByRole('dialog', { name: /Edit task/ })).not.toBeInTheDocument();
		await page.getByRole('button', { name: 'Return to tasks' }).click();
		expect(closeRoute).toHaveBeenCalledWith(route);
	});
});

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((settle) => (resolve = settle));
	return { promise, resolve };
}

function testProject(overrides: Partial<Project> = {}): Project {
	return {
		id: 'project-id',
		name: 'Project',
		layout: 'list',
		colorTheme: 'sage',
		agentModel: 'gpt-default',
		agentThinkingEffort: 'medium',
		position: 1024,
		version: 1,
		archivedAt: null,
		createdAt: '2026-07-22T10:00:00Z',
		updatedAt: '2026-07-22T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'task-id',
		projectId: 'project-id',
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
		createdAt: '2026-07-22T10:00:00Z',
		updatedAt: '2026-07-22T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
