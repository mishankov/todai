import { expect, test } from '@playwright/test';

test('shows the application shell', async ({ page }) => {
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toContainText('task space');
});
