import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Project } from '$lib/projects/client';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
	it('shows the authenticated user and signs out', async () => {
		let loggedOut = false;
		const onLogout = async () => {
			loggedOut = true;
		};
		const view = render(AppShell, { username: 'owner', onLogout, currentPath: '/today' });

		await expect.element(view.getByText('owner', { exact: true })).toHaveTextContent('owner');
		await expect
			.element(view.getByRole('link', { name: 'Today' }))
			.toHaveAttribute('aria-current', 'page');

		await view.getByRole('button', { name: 'Open navigation' }).click();
		await view.getByRole('button', { name: 'Log out' }).click();
		await vi.waitFor(() => expect(loggedOut).toBe(true));
	});

	it('shows projects in the sidebar and marks the current project', async () => {
		const project = testProject({ id: 'work-id', name: 'Work' });
		render(AppShell, {
			username: 'owner',
			projects: [project],
			onLogout: vi.fn(),
			currentPath: '/projects/work-id'
		});

		await expect
			.element(page.getByRole('link', { name: 'Work' }))
			.toHaveAttribute('aria-current', 'page');
		await expect.element(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
	});

	it('marks All tasks as the current section', async () => {
		render(AppShell, {
			username: 'owner',
			onLogout: vi.fn(),
			currentPath: '/all'
		});

		await expect
			.element(page.getByRole('link', { name: 'All tasks' }))
			.toHaveAttribute('aria-current', 'page');
	});

	it('links to activity and marks it as the current section', async () => {
		render(AppShell, {
			username: 'owner',
			onLogout: vi.fn(),
			currentPath: '/activity'
		});

		await expect
			.element(page.getByRole('link', { name: 'Activity' }))
			.toHaveAttribute('aria-current', 'page');
	});

	it('links to settings and marks it as the current section', async () => {
		render(AppShell, {
			username: 'owner',
			onLogout: vi.fn(),
			currentPath: '/settings'
		});

		await expect
			.element(page.getByRole('link', { name: 'Settings' }))
			.toHaveAttribute('aria-current', 'page');
	});
});

function testProject(overrides: Partial<Project> = {}): Project {
	return {
		id: 'project-id',
		name: 'Project',
		layout: 'list',
		position: 1024,
		version: 1,
		archivedAt: null,
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
