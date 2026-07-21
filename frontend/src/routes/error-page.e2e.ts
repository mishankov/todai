import { expect, test } from '@playwright/test';

test('shows a helpful 404 page', async ({ page }) => {
	const response = await page.goto('/a-page-that-does-not-exist');

	expect(response?.status()).toBe(404);
	await expect(page).toHaveTitle('404 · Page not found — Todai');
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('This page wandered off.');
	await expect(page.getByRole('link', { name: 'Back to Todai' })).toHaveAttribute('href', '/');
	await expect(page.getByRole('button', { name: 'Go back' })).toBeVisible();
});
