import { expect, test } from '@playwright/test';

test('redirects to login and supports the complete session flow', async ({ page }) => {
	let authenticated = false;

	await page.route('**/api/auth/me', async (route) => {
		if (!authenticated) {
			await route.fulfill({ status: 401 });
			return;
		}

		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ username: 'owner' })
		});
	});
	await page.route('**/api/auth/login', async (route) => {
		const credentials = route.request().postDataJSON();
		authenticated =
			credentials.login === 'owner' && credentials.password === 'correct horse battery staple';
		await route.fulfill({ status: authenticated ? 200 : 401 });
	});
	await page.route('**/api/auth/logout', async (route) => {
		authenticated = false;
		await route.fulfill({ status: 200 });
	});

	await page.goto('/');
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Welcome back.');

	await page.getByLabel('Username').fill('owner');
	await page.getByLabel('Password').fill('correct horse battery staple');
	await page.getByRole('button', { name: 'Sign in' }).click();

	await expect(page).toHaveURL(/\/$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Welcome back, owner.');

	await page.getByRole('button', { name: 'Log out' }).click();
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Welcome back.');
});
