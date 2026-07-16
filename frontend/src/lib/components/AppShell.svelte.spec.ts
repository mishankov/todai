import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
	it('describes the current product stage', async () => {
		render(AppShell);

		await expect
			.element(page.getByRole('heading', { level: 1 }))
			.toHaveTextContent('Your task space is taking shape.');
		await expect.element(page.getByRole('status')).toHaveTextContent('Stage 0');
	});
});
