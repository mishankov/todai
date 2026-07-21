import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Project, ProjectSection } from '$lib/projects/client';
import type { Task } from '$lib/tasks/client';
import CommandPalette from './CommandPalette.svelte';

describe('CommandPalette', () => {
	it('exposes dialog, combobox, listbox, flat keyboard navigation, and a focus trap', async () => {
		const project = testProject({ name: 'Work' });
		const close = vi.fn();
		const switchProject = vi.fn();

		renderPalette({ project, close, switchProject });

		await expect
			.element(page.getByRole('dialog', { name: 'Command palette' }))
			.toHaveAttribute('aria-modal', 'true');
		const input = page.getByRole('combobox', { name: 'Search commands, projects, and tasks' });
		expect(document.activeElement).toBe(input.element());
		await expect.element(input).toHaveAttribute('aria-controls', 'command-palette-results');
		await expect
			.element(page.getByRole('listbox', { name: 'Palette results' }))
			.toHaveAttribute('aria-busy', 'false');

		await userEvent.keyboard('{End}');
		await expect.element(page.getByRole('option', { selected: true })).toHaveTextContent('Work');
		await userEvent.keyboard('{Enter}');
		expect(close).toHaveBeenCalledWith(false);
		expect(switchProject).toHaveBeenCalledWith(project);

		const closeButton = page.getByRole('button', { name: 'Close command palette' });
		closeButton.element().focus();
		await userEvent.keyboard('{Tab}');
		expect(document.activeElement).toBe(input.element());
	});

	it('debounces, aborts stale requests, and never lets an old response replace the new query', async () => {
		const first = deferred<Task[]>();
		const second = deferred<Task[]>();
		const signals: AbortSignal[] = [];
		const search = vi.fn((query: string, _projectId: string, signal: AbortSignal) => {
			signals.push(signal);
			return query === 'first' ? first.promise : second.promise;
		});
		renderPalette({ search, debounceMs: 0 });
		const input = page.getByRole('combobox', { name: 'Search commands, projects, and tasks' });

		await input.fill('first');
		await vi.waitFor(() => expect(search).toHaveBeenCalledTimes(1));
		await expect.element(page.getByText('Searching tasks…', { exact: true })).toBeVisible();
		await input.fill('second');
		await vi.waitFor(() => expect(search).toHaveBeenCalledTimes(2));
		expect(signals[0].aborted).toBe(true);

		second.resolve([testTask({ id: 'new', title: 'Second result' })]);
		await expect.element(page.getByRole('option', { name: /Second result/ })).toBeVisible();
		first.resolve([testTask({ id: 'old', title: 'Stale first result' })]);
		await expect
			.element(page.getByRole('option', { name: /Stale first result/ }))
			.not.toBeInTheDocument();
	});

	it('keeps local results when task search fails and reports an empty state after a later query', async () => {
		const search = vi
			.fn<(query: string, projectId: string, signal: AbortSignal) => Promise<Task[]>>()
			.mockRejectedValueOnce(new Error('offline'))
			.mockResolvedValueOnce([]);
		renderPalette({ search, debounceMs: 0 });
		const input = page.getByRole('combobox', { name: 'Search commands, projects, and tasks' });

		await input.fill('settings');
		await expect
			.element(
				page.getByText('Task search failed. Change the search to try again.', { exact: true })
			)
			.toBeVisible();
		await expect.element(page.getByRole('option', { name: /Project settings/ })).toBeVisible();

		await input.fill('nothing matches this');
		await expect
			.element(page.getByText('No matching commands, projects, or tasks.', { exact: true }))
			.toBeVisible();
		expect(search).toHaveBeenCalledTimes(2);
	});
});

function renderPalette(
	overrides: {
		project?: Project;
		close?: (restoreFocus?: boolean) => void;
		switchProject?: (project: Project) => void | Promise<void>;
		search?: (query: string, projectId: string, signal: AbortSignal) => Promise<Task[]>;
		debounceMs?: number;
	} = {}
) {
	const project = overrides.project ?? testProject();
	return render(CommandPalette, {
		projects: [project],
		activeProject: project,
		applePlatform: false,
		close: overrides.close ?? (() => {}),
		executeCommand: () => {},
		switchProject: overrides.switchProject ?? (() => {}),
		selectTask: () => {},
		search: overrides.search ?? vi.fn(async () => []),
		loadSections: vi.fn(async () => [testSection()]),
		debounceMs: overrides.debounceMs ?? 0
	});
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((promiseResolve, promiseReject) => {
		resolve = promiseResolve;
		reject = promiseReject;
	});
	return { promise, resolve, reject };
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
		createdAt: '2026-07-21T10:00:00Z',
		updatedAt: '2026-07-21T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testSection(overrides: Partial<ProjectSection> = {}): ProjectSection {
	return {
		id: 'section-id',
		projectId: 'project-id',
		name: 'Planning',
		position: 1024,
		version: 1,
		createdAt: '2026-07-21T10:00:00Z',
		updatedAt: '2026-07-21T10:00:00Z',
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
		createdAt: '2026-07-21T10:00:00Z',
		updatedAt: '2026-07-21T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
