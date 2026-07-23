import { expect, test } from '@playwright/test';
import type { Project } from '$lib/projects/client';
import type { Appearance } from '$lib/appearance/theme';

const darkProjectPalettes = {
	sage: {
		canvas: '#121713',
		sidebar: '#172019',
		surface: '#1b241d',
		elevated: '#222d25',
		control: '#202a22',
		border: '#5b7562',
		borderStrong: '#6e8374',
		hover: '#243329',
		selected: '#2b3e31',
		focus: '#79d392b3',
		surfaceRgb: 'rgb(27, 36, 29)',
		hoverRgb: 'rgb(36, 51, 41)'
	},
	ocean: {
		canvas: '#11171b',
		sidebar: '#151f25',
		surface: '#19242a',
		elevated: '#202e36',
		control: '#1c2930',
		border: '#5b7380',
		borderStrong: '#6b7f89',
		hover: '#21323b',
		selected: '#29424e',
		focus: '#79c5f2b3',
		surfaceRgb: 'rgb(25, 36, 42)',
		hoverRgb: 'rgb(33, 50, 59)'
	},
	plum: {
		canvas: '#181219',
		sidebar: '#211923',
		surface: '#251c28',
		elevated: '#2d2231',
		control: '#281e2c',
		border: '#7a6483',
		borderStrong: '#8d7595',
		hover: '#34283a',
		selected: '#422f4a',
		focus: '#d0a0e5b3',
		surfaceRgb: 'rgb(37, 28, 40)',
		hoverRgb: 'rgb(52, 40, 58)'
	},
	sand: {
		canvas: '#181512',
		sidebar: '#201c17',
		surface: '#252019',
		elevated: '#2e271e',
		control: '#29231b',
		border: '#7d6b58',
		borderStrong: '#8e7a65',
		hover: '#352e26',
		selected: '#45392d',
		focus: '#e3b77cb3',
		surfaceRgb: 'rgb(37, 32, 25)',
		hoverRgb: 'rgb(53, 46, 38)'
	},
	rose: {
		canvas: '#191214',
		sidebar: '#21191c',
		surface: '#271c20',
		elevated: '#302228',
		control: '#2b1e23',
		border: '#81636c',
		borderStrong: '#92717b',
		hover: '#38282e',
		selected: '#492f37',
		focus: '#efa2b0b3',
		surfaceRgb: 'rgb(39, 28, 32)',
		hoverRgb: 'rgb(56, 40, 46)'
	},
	graphite: {
		canvas: '#151618',
		sidebar: '#1b1d20',
		surface: '#212328',
		elevated: '#292c32',
		control: '#25282d',
		border: '#696f7b',
		borderStrong: '#7a818c',
		hover: '#2c2f34',
		selected: '#383c44',
		focus: '#c1c8d2b3',
		surfaceRgb: 'rgb(33, 35, 40)',
		hoverRgb: 'rgb(44, 47, 52)'
	}
} satisfies Record<Project['colorTheme'], Record<string, string>>;

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
		const allTasksMatch = path.match(/^\/api\/views\/projects\/([^/]+)\/all$/);
		if (allTasksMatch) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ tasks: [testTask(allTasksMatch[1])] })
			});
			return;
		}
		const projectTasksMatch = path.match(/^\/api\/views\/projects\/([^/]+)$/);
		if (projectTasksMatch) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ tasks: [testTask(projectTasksMatch[1])] })
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
		const palette = darkProjectPalettes[project.colorTheme];
		await page.getByLabel('Project', { exact: true }).selectOption(project.id);
		await expect(page).toHaveURL(new RegExp(`/projects/${project.id}/overview$`));
		const projectContext = page.locator('.project-context');
		await expect(projectContext).toHaveClass(new RegExp(`theme-${project.colorTheme}`));
		await expect(page.locator('html')).toHaveAttribute('data-appearance', 'dark');
		await expect
			.poll(() =>
				projectContext.evaluate((element) => {
					const style = getComputedStyle(element);
					return {
						canvas: style.getPropertyValue('--color-canvas').trim(),
						sidebar: style.getPropertyValue('--color-sidebar').trim(),
						surface: style.getPropertyValue('--color-surface').trim(),
						elevated: style.getPropertyValue('--color-surface-elevated').trim(),
						control: style.getPropertyValue('--color-control').trim(),
						border: style.getPropertyValue('--color-border').trim(),
						borderStrong: style.getPropertyValue('--color-border-strong').trim(),
						hover: style.getPropertyValue('--color-hover').trim(),
						selected: style.getPropertyValue('--color-selected').trim(),
						focus: style.getPropertyValue('--color-focus').trim()
					};
				})
			)
			.toEqual({
				canvas: palette.canvas,
				sidebar: palette.sidebar,
				surface: palette.surface,
				elevated: palette.elevated,
				control: palette.control,
				border: palette.border,
				borderStrong: palette.borderStrong,
				hover: palette.hover,
				selected: palette.selected,
				focus: palette.focus
			});

		await page.goto(`/projects/${project.id}/tasks`);
		await expect(page.locator('.task-card')).toHaveCSS('background-color', palette.surfaceRgb);
		await expect(page.locator('.task-quick-add .composer')).toHaveCSS(
			'background-color',
			palette.surfaceRgb
		);
		await expect(page.locator('.task-quick-add button[type="submit"]')).toHaveCSS(
			'background-color',
			palette.hoverRgb
		);
		await page.screenshot({
			path: `test-results/appearance-dark-${project.colorTheme}-board.png`,
			fullPage: true
		});
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
		layout: 'board',
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

function testTask(projectId: string) {
	return {
		id: `task-${projectId}`,
		projectId,
		sectionId: null,
		parentId: null,
		title: 'Palette task',
		description: null,
		status: 'active',
		priority: 1,
		dueDate: null,
		dueTime: null,
		dueTimezone: null,
		position: 1024,
		version: 1,
		completedAt: null,
		createdAt: '2026-07-22T00:00:00Z',
		updatedAt: '2026-07-22T00:00:00Z',
		lastModifiedBy: 'user-id',
		subtaskCount: 0,
		completedSubtaskCount: 0
	};
}
