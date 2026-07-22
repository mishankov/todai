import { expect, test } from '@playwright/test';
import type { Project } from '$lib/projects/client';
import type { Appearance } from '$lib/appearance/theme';

test('applies system and saved appearance across reloads, projects, and unauthenticated pages', async ({
	page
}) => {
	let authenticated = true;
	let appearance: Appearance = 'system';
	let settingsVersion = 1;
	const projects = [
		testProject('project-sage', 'Personal', 'sage'),
		testProject('project-ocean', 'Work', 'ocean'),
		testProject('project-plum', 'Writing', 'plum'),
		testProject('project-sand', 'Home', 'sand'),
		testProject('project-rose', 'Health', 'rose'),
		testProject('project-graphite', 'Archive', 'graphite')
	];

	await page.route('**/api/**', async (route) => {
		const request = route.request();
		const url = new URL(request.url());
		const path = url.pathname;
		if (path === '/api/auth/me') {
			await route.fulfill(
				authenticated
					? {
							status: 200,
							contentType: 'application/json',
							body: JSON.stringify({ username: 'owner' })
						}
					: { status: 401 }
			);
			return;
		}
		if (path === '/api/settings') {
			if (request.method() === 'PATCH') {
				appearance = request.postDataJSON().appearance;
				settingsVersion += 1;
			}
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(settingsView(appearance, settingsVersion))
			});
			return;
		}
		if (path === '/api/projects') {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ projects })
			});
			return;
		}
		const projectMatch = path.match(/^\/api\/projects\/([^/]+)$/);
		if (projectMatch) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(projects.find((project) => project.id === projectMatch[1]))
			});
			return;
		}
		if (/^\/api\/projects\/[^/]+\/sections$/.test(path)) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ sections: [] })
			});
			return;
		}
		if (/^\/api\/views\/projects\/[^/]+\/inbox$/.test(path)) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ tasks: [] })
			});
			return;
		}
		if (/^\/api\/views\/projects\/[^/]+\/all$/.test(path)) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ tasks: [] })
			});
			return;
		}
		if (/^\/api\/views\/projects\/[^/]+$/.test(path)) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ tasks: [] })
			});
			return;
		}
		if (path === '/api/activity/changes') {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ cursor: 0, events: [] })
			});
			return;
		}
		await route.fulfill({ status: 404 });
	});

	await page.emulateMedia({ colorScheme: 'dark' });
	await page.goto('/settings');
	await expect(page.locator('html')).toHaveAttribute('data-appearance', 'dark');

	await page.emulateMedia({ colorScheme: 'light' });
	await expect(page.locator('html')).toHaveAttribute('data-appearance', 'light');

	await page.getByRole('radio', { name: /Dark/ }).click();
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.locator('html')).toHaveAttribute('data-appearance', 'dark');
	await page.emulateMedia({ colorScheme: 'light' });
	await expect(page.locator('html')).toHaveAttribute('data-appearance', 'dark');

	await page.reload();
	await expect(page.locator('html')).toHaveAttribute('data-appearance', 'dark');
	for (const project of projects) {
		await page.getByLabel('Project', { exact: true }).selectOption(project.id);
		await expect(page).toHaveURL(new RegExp(`/projects/${project.id}/overview$`));
		await expect(page.locator('.project-context')).toHaveClass(
			new RegExp(`theme-${project.colorTheme}`)
		);
		await expect(page.locator('html')).toHaveAttribute('data-appearance', 'dark');
	}

	await page.goto('/projects/project-graphite/settings');
	await page.screenshot({ path: 'test-results/appearance-dark-desktop.png', fullPage: true });
	await page.setViewportSize({ width: 390, height: 844 });
	await page.waitForTimeout(250);
	await page.screenshot({ path: 'test-results/appearance-dark-mobile.png' });
	await page.setViewportSize({ width: 1280, height: 720 });

	await page.emulateMedia({ colorScheme: 'dark' });
	await page.goto('/settings');
	await page.getByRole('radio', { name: /Light/ }).click();
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.locator('html')).toHaveAttribute('data-appearance', 'light');
	await page.screenshot({ path: 'test-results/appearance-light-settings.png', fullPage: true });

	authenticated = false;
	await page.goto('/login');
	await expect(page.locator('html')).not.toHaveAttribute('data-appearance');
	await expect
		.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).colorScheme))
		.toContain('dark');
	await page.screenshot({ path: 'test-results/appearance-system-login-dark.png', fullPage: true });

	await page.goto('/projects/not-a-real-route/missing');
	await expect(page.getByRole('heading', { name: 'This page wandered off.' })).toBeVisible();
	await expect(page.locator('html')).not.toHaveAttribute('data-appearance');
	await expect
		.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).colorScheme))
		.toContain('dark');
});

function settingsView(appearance: Appearance, version: number) {
	return {
		settings: {
			timezone: 'UTC',
			agentModel: 'gpt-default',
			agentThinkingEffort: 'medium',
			appearance,
			version,
			createdAt: '2026-07-22T00:00:00Z',
			updatedAt: '2026-07-22T00:00:00Z',
			lastModifiedBy: 'user-id'
		},
		availableAgentModels: ['gpt-default'],
		availableAgentThinkingEfforts: ['medium']
	};
}

function testProject(id: string, name: string, colorTheme: Project['colorTheme']): Project {
	return {
		id,
		name,
		layout: 'list',
		colorTheme,
		agentModel: 'gpt-default',
		agentThinkingEffort: 'medium',
		position: 1024,
		version: 1,
		archivedAt: null,
		createdAt: '2026-07-22T00:00:00Z',
		updatedAt: '2026-07-22T00:00:00Z',
		lastModifiedBy: 'user-id'
	};
}
