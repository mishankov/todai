import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Project } from '$lib/projects/client';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
	it('shows the active project as the top-level workspace', async () => {
		const project = testProject({ id: 'work-id', name: 'Work', colorTheme: 'ocean' });
		render(AppShell, {
			username: 'owner',
			projects: [project, testProject({ id: 'home-id', name: 'Home' })],
			activeProject: project,
			onLogout: vi.fn(),
			currentPath: '/projects/work-id/today'
		});

		await expect.element(page.getByLabelText('Project', { exact: true })).toHaveValue('work-id');
		await expect
			.element(page.getByRole('link', { name: 'Today' }))
			.toHaveAttribute('aria-current', 'page');
		await expect
			.element(page.getByRole('link', { name: 'Inbox' }))
			.toHaveAttribute('href', '/projects/work-id');
		await expect
			.element(page.getByRole('link', { name: 'Overview' }))
			.toHaveAttribute('href', '/projects/work-id/overview');
		await expect
			.element(page.getByRole('link', { name: 'Tasks' }))
			.toHaveAttribute('href', '/projects/work-id/tasks');
		await expect
			.element(
				page.getByRole('button', {
					name: /Create task \((Cmd \+ N \/ Cmd \+ Option \+ N|Ctrl \+ N \/ Ctrl \+ Alt \+ N)\)/
				})
			)
			.toHaveAttribute(
				'title',
				expect.stringMatching(
					/Create task \((Cmd \+ N \/ Cmd \+ Option \+ N|Ctrl \+ N \/ Ctrl \+ Alt \+ N)\)/
				)
			);
		await expect
			.element(
				page.getByRole('button', { name: /Open command palette \((Cmd|Ctrl) \+ K\)/ }).first()
			)
			.toHaveAttribute('aria-keyshortcuts', expect.stringMatching(/(Meta|Control)\+K/));
		await expect.element(page.getByRole('link', { name: 'Sections' })).not.toBeInTheDocument();
		await expect.element(page.getByText('Organize', { exact: true })).not.toBeInTheDocument();
	});

	it('marks Tasks as the active project view', async () => {
		const project = testProject({ id: 'work-id' });
		render(AppShell, {
			username: 'owner',
			projects: [project],
			activeProject: project,
			onLogout: vi.fn(),
			currentPath: '/projects/work-id/tasks'
		});

		await expect
			.element(page.getByRole('link', { name: 'Tasks' }))
			.toHaveAttribute('aria-current', 'page');
	});

	it('marks project settings independently from account settings', async () => {
		const project = testProject({ id: 'work-id' });
		render(AppShell, {
			username: 'owner',
			projects: [project],
			activeProject: project,
			onLogout: vi.fn(),
			currentPath: '/projects/work-id/settings'
		});

		await expect
			.element(page.getByRole('link', { name: 'Project settings' }))
			.toHaveAttribute('aria-current', 'page');
		await expect
			.element(page.getByRole('link', { name: 'Account settings' }))
			.not.toHaveAttribute('aria-current');
	});
});

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
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
