import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
	it('shows the authenticated user and signs out', async () => {
		const onLogout = vi.fn(async () => {});
		render(AppShell, { username: 'owner', onLogout, currentPath: '/today' });

		await expect.element(page.getByText('owner', { exact: true })).toHaveTextContent('owner');
		await expect
			.element(page.getByRole('link', { name: 'Today' }))
			.toHaveAttribute('aria-current', 'page');

		await page.getByRole('button', { name: 'Log out' }).click();
		expect(onLogout).toHaveBeenCalledOnce();
	});
});
