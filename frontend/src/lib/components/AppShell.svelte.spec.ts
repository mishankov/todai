import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Project } from '$lib/projects/client';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
	it('shows the authenticated user and signs out', async () => {
		const onLogout = vi.fn(async () => {});
		render(AppShell, { username: 'owner', onLogout, currentPath: '/today' });

		await expect.element(page.getByText('owner', { exact: true })).toHaveTextContent('owner');
		await expect
			.element(page.getByRole('link', { name: 'Today' }))
			.toHaveAttribute('aria-current', 'page');

		await page.getByRole('button', { name: 'Open navigation' }).click();
		await page.getByRole('button', { name: 'Log out' }).click();
		expect(onLogout).toHaveBeenCalledOnce();
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
