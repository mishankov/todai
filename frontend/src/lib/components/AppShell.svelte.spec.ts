import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
	it('shows the authenticated user and signs out', async () => {
		const onLogout = vi.fn(async () => {});
		render(AppShell, { username: 'owner', onLogout });

		await expect
			.element(page.getByRole('heading', { level: 1 }))
			.toHaveTextContent('Welcome back, owner.');
		await expect.element(page.getByRole('status')).toHaveTextContent('Authentication connected');

		await page.getByRole('button', { name: 'Log out' }).click();
		expect(onLogout).toHaveBeenCalledOnce();
	});
});
