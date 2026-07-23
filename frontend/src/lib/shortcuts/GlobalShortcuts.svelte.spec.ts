import { page, userEvent } from 'vitest/browser';
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
		const createTask = vi.fn(async () => created);
		const refresh = vi.fn(async () => {});

		render(GlobalShortcuts, {
			activeProject: project,
			projects: [project],
			currentPath: `/projects/${project.id}/sections/${section.id}`,
			navigate: vi.fn(),
			refresh,
			loadSections: vi.fn(async () => [section]),
			createTask
		});

		dispatchShortcut('KeyN', true);
		const dialog = page.getByRole('dialog', { name: 'Create a task' });
		await expect.element(dialog).toBeVisible();
		dispatchShortcut('KeyK');
		await expect
			.element(page.getByRole('dialog', { name: 'Command palette' }))
			.not.toBeInTheDocument();
		expect(document.activeElement).toBe(dialog.getByLabelText('Title').element());
		dispatchShortcut('KeyN');
		expect(
			document.querySelectorAll('[role="dialog"][aria-labelledby="quick-add-title"]')
		).toHaveLength(1);

		await expect
			.element(dialog.getByRole('button', { name: /^project: .*\. Open picker$/ }))
			.not.toBeInTheDocument();
		await expect
			.element(dialog.getByRole('button', { name: /^section: .*\. Open picker$/ }))
			.not.toBeInTheDocument();
		const title = dialog.getByLabelText('Title');
		await title.fill('Plan the release !hi');
		await userEvent.keyboard('{Enter}');
		await expect.element(title).toHaveValue('Plan the release');
		await expect
			.element(dialog.getByRole('button', { name: 'Priority: High', exact: true }))
			.toBeVisible();
		await expect
			.element(dialog.getByRole('button', { name: 'priority: High. Open picker' }))
			.not.toBeInTheDocument();
		await dialog.getByRole('button', { name: /^Location:/ }).click();
		const locationPopover = dialog.getByRole('dialog', { name: 'Task location' });
		await expect.element(locationPopover).toBeVisible();
		expect(getComputedStyle(locationPopover.element()).position).toBe('fixed');
		expect(locationPopover.element().getBoundingClientRect().bottom).toBeLessThanOrEqual(
			window.innerHeight
		);
		await dialog.getByRole('button', { name: /^Section:/ }).click();
		await page.getByRole('option', { name: section.name }).click();
		await dialog.getByRole('button', { name: 'Due date: No date' }).click();
		await page.getByRole('option', { name: /^Tomorrow/ }).click();
		await dialog.getByRole('button', { name: /^Due time:/ }).click();
		await page.getByRole('option', { name: /^Morning/ }).click();
		await dialog.getByRole('button', { name: 'Create task' }).click();

		expect(createTask).toHaveBeenCalledWith(
			expect.objectContaining({
				title: 'Plan the release',
				projectId: project.id,
				sectionId: section.id,
				priority: 3,
				dueDate: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
				dueTime: '09:00',
				dueTimezone: Intl.DateTimeFormat().resolvedOptions().timeZone
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

	it('uses the active project in shared account command destinations', async () => {
		const project = testProject({ id: 'project/with space' });
		const navigate = vi.fn(async () => {});
		render(GlobalShortcuts, {
			activeProject: project,
			projects: [project],
			currentPath: `/projects/${encodeURIComponent(project.id)}/tasks`,
			navigate,
			loadSections: vi.fn(async () => [])
		});

		dispatchShortcut('KeyK');
		const palette = page.getByRole('dialog', { name: 'Command palette' });
		await palette
			.getByRole('combobox', { name: 'Search commands, projects, and tasks' })
			.fill('Manage projects');
		await userEvent.keyboard('{Enter}');
		expect(navigate).toHaveBeenLastCalledWith('/projects?project=project%2Fwith+space');

		dispatchShortcut('KeyK');
		await page
			.getByRole('dialog', { name: 'Command palette' })
			.getByRole('combobox', { name: 'Search commands, projects, and tasks' })
			.fill('Account settings');
		await userEvent.keyboard('{Enter}');
		expect(navigate).toHaveBeenLastCalledWith('/settings?project=project%2Fwith+space');
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

	it('toggles the palette on authenticated routes and restores the previous focus', async () => {
		const previous = document.createElement('button');
		previous.textContent = 'Before palette';
		document.body.append(previous);
		previous.focus();
		try {
			render(GlobalShortcuts, { projects: [testProject()], currentPath: '/settings' });

			dispatchShortcut('KeyK');
			const palette = page.getByRole('dialog', { name: 'Command palette' });
			await expect.element(palette).toBeVisible();
			expect(document.activeElement).toBe(
				palette.getByRole('combobox', { name: 'Search commands, projects, and tasks' }).element()
			);

			dispatchShortcut('KeyK');
			await expect.element(palette).not.toBeInTheDocument();
			await vi.waitFor(() => expect(document.activeElement).toBe(previous));
		} finally {
			previous.remove();
		}
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
