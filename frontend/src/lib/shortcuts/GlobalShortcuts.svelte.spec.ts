import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Project, ProjectSection } from '$lib/projects/client';
import type { Task } from '$lib/tasks/client';
import { chatToggleRequestEvent } from './events';
import GlobalShortcuts from './GlobalShortcuts.svelte';
import { isApplePlatform } from './registry';

describe('GlobalShortcuts', () => {
	it('opens one quick-add dialog, focuses its title, and saves the selected properties', async () => {
		const project = testProject();
		const section = testSection();
		const created = testTask();
		const updated = testTask({
			priority: 3,
			dueDate: '2026-07-22',
			dueTime: '09:30',
			dueTimezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
			sectionId: section.id,
			version: 2
		});
		const createTask = vi.fn(async () => created);
		const updateTask = vi.fn(async () => updated);
		const refresh = vi.fn(async () => {});

		render(GlobalShortcuts, {
			activeProject: project,
			projects: [project],
			currentPath: `/projects/${project.id}`,
			navigate: vi.fn(),
			refresh,
			loadSections: vi.fn(async () => [section]),
			createTask,
			updateTask
		});

		dispatchShortcut('KeyN', true);
		const dialog = page.getByRole('dialog', { name: 'Create a task' });
		await expect.element(dialog).toBeVisible();
		expect(document.activeElement).toBe(dialog.getByLabelText('Title').element());
		dispatchShortcut('KeyN');
		expect(
			document.querySelectorAll('[role="dialog"][aria-labelledby="quick-add-title"]')
		).toHaveLength(1);

		await dialog.getByLabelText('Title').fill('Plan the release');
		await dialog.getByLabelText('Section').selectOptions(section.id);
		await dialog.getByLabelText('Priority').selectOptions('3');
		await dialog.getByLabelText('Due date').fill('2026-07-22');
		await dialog.getByLabelText('Due time').fill('09:30');
		await dialog.getByRole('button', { name: 'Create task' }).click();

		expect(createTask).toHaveBeenCalledWith('Plan the release', project.id, section.id);
		expect(updateTask).toHaveBeenCalledWith(
			created.id,
			expect.objectContaining({
				version: created.version,
				priority: 3,
				dueDate: '2026-07-22',
				dueTime: '09:30',
				sectionId: section.id
			})
		);
		await expect.element(dialog).not.toBeInTheDocument();
		expect(refresh).toHaveBeenCalledOnce();
	});

	it('dispatches chat and uses client-side navigation for the active project', async () => {
		const project = testProject({ id: 'project/with space' });
		const navigate = vi.fn(async () => {});
		const chatToggle = vi.fn();
		window.addEventListener(chatToggleRequestEvent, chatToggle);
		try {
			render(GlobalShortcuts, {
				activeProject: project,
				projects: [project],
				currentPath: `/projects/${project.id}`,
				navigate
			});

			dispatchShortcut('KeyJ');
			expect(chatToggle).toHaveBeenCalledOnce();
			dispatchShortcut('Digit3');
			expect(navigate).toHaveBeenCalledWith('/projects/project%2Fwith%20space/today');
		} finally {
			window.removeEventListener(chatToggleRequestEvent, chatToggle);
		}
	});

	it('keeps project commands inactive without a project but leaves help available', async () => {
		const navigate = vi.fn();
		render(GlobalShortcuts, { projects: [], currentPath: '/projects', navigate });

		dispatchShortcut('KeyN');
		await expect
			.element(page.getByRole('dialog', { name: 'Create a task' }))
			.not.toBeInTheDocument();
		dispatchShortcut('Slash');
		await expect.element(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible();
		dispatchShortcut('Slash');
		await expect
			.element(page.getByRole('dialog', { name: 'Keyboard shortcuts' }))
			.not.toBeInTheDocument();
		expect(navigate).not.toHaveBeenCalled();
	});

	it('shows shortcut hints only while the platform modifier is held', () => {
		render(GlobalShortcuts, { projects: [], currentPath: '/projects' });
		const modifier = isApplePlatform(window.navigator.platform) ? 'Meta' : 'Control';

		window.dispatchEvent(new KeyboardEvent('keydown', { key: modifier, bubbles: true }));
		expect(document.documentElement.dataset.shortcutHints).toBe('visible');

		window.dispatchEvent(new KeyboardEvent('keyup', { key: modifier, bubbles: true }));
		expect(document.documentElement.dataset.shortcutHints).toBeUndefined();

		window.dispatchEvent(new KeyboardEvent('keydown', { key: modifier, bubbles: true }));
		window.dispatchEvent(new Event('blur'));
		expect(document.documentElement.dataset.shortcutHints).toBeUndefined();
	});
});

function dispatchShortcut(code: string, altKey = false) {
	const apple = isApplePlatform(window.navigator.platform);
	window.dispatchEvent(
		new KeyboardEvent('keydown', {
			bubbles: true,
			cancelable: true,
			code,
			key: code,
			altKey,
			metaKey: apple,
			ctrlKey: !apple
		})
	);
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
		title: 'Plan the release',
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
