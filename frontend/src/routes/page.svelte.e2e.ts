import { expect, test } from '@playwright/test';
import type { Project } from '$lib/projects/client';
import type { Task } from '$lib/tasks/client';

test('supports login, Inbox, projects, Today, and logout', async ({ page }) => {
	let authenticated = false;
	let tasks: Task[] = [];
	let projects: Project[] = [];

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
	await page.route('**/api/projects?*', async (route) => {
		const includeArchived =
			new URL(route.request().url()).searchParams.get('include_archived') === 'true';
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				projects: includeArchived ? projects : projects.filter((item) => item.archivedAt === null)
			})
		});
	});
	await page.route('**/api/projects', async (route) => {
		const request = route.request().postDataJSON();
		const created = testProject({ id: `project-${projects.length + 1}`, name: request.name });
		projects = [...projects, created];
		await route.fulfill({
			status: 201,
			contentType: 'application/json',
			body: JSON.stringify(created)
		});
	});
	await page.route('**/api/projects/*', async (route) => {
		const projectId = new URL(route.request().url()).pathname.split('/').at(-1);
		const found = projects.find((item) => item.id === projectId)!;
		if (route.request().method() === 'PATCH') {
			const changes = route.request().postDataJSON();
			Object.assign(found, changes, {
				version: found.version + 1,
				archivedAt: changes.archived === true ? new Date().toISOString() : found.archivedAt
			});
		}
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(found)
		});
	});
	await page.route('**/api/views/inbox?*', async (route) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ tasks })
		});
	});
	await page.route('**/api/views/today?*', async (route) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ tasks: tasks.filter((item) => item.dueAt !== null) })
		});
	});
	await page.route('**/api/views/projects/*?*', async (route) => {
		const projectId = new URL(route.request().url()).pathname.split('/').at(-1);
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ tasks: tasks.filter((item) => item.projectId === projectId) })
		});
	});
	await page.route('**/api/tasks', async (route) => {
		const request = route.request().postDataJSON();
		const created = testTask({
			id: `task-${tasks.length + 1}`,
			title: request.title,
			projectId: request.projectId ?? null
		});
		tasks = [...tasks, created];
		await route.fulfill({
			status: 201,
			contentType: 'application/json',
			body: JSON.stringify(created)
		});
	});
	await page.route('**/api/tasks/*/complete', async (route) => {
		const taskId = new URL(route.request().url()).pathname.split('/').at(-2);
		const updated = tasks.find((item) => item.id === taskId)!;
		Object.assign(updated, {
			status: 'completed',
			version: updated.version + 1,
			completedAt: new Date().toISOString()
		});
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(updated)
		});
	});
	await page.route('**/api/tasks/*/reopen', async (route) => {
		const taskId = new URL(route.request().url()).pathname.split('/').at(-2);
		const updated = tasks.find((item) => item.id === taskId)!;
		Object.assign(updated, {
			status: 'active',
			version: updated.version + 1,
			completedAt: null
		});
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(updated)
		});
	});
	await page.route('**/api/tasks/*', async (route) => {
		const method = route.request().method();
		const taskId = new URL(route.request().url()).pathname.split('/').at(-1);
		if (method === 'PATCH') {
			const changes = route.request().postDataJSON();
			const updated = tasks.find((item) => item.id === taskId)!;
			if (changes.version !== updated.version) {
				await route.fulfill({ status: 409 });
				return;
			}

			Object.assign(updated, changes, { version: updated.version + 1 });
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(updated)
			});
			return;
		}
		if (method !== 'DELETE') {
			await route.fallback();
			return;
		}

		tasks = tasks.filter((item) => item.id !== taskId);
		await route.fulfill({ status: 204 });
	});

	await page.goto('/');
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Welcome back.');

	await page.getByLabel('Username').fill('owner');
	await page.getByLabel('Password').fill('correct horse battery staple');
	await page.getByRole('button', { name: 'Sign in' }).click();

	await expect(page).toHaveURL(/\/$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Inbox');

	await page.getByLabel('Task title').fill('Buy milk');
	await page.getByRole('button', { name: 'Add task' }).click();
	await expect(page.getByText('Buy milk')).toBeVisible();

	await page.getByRole('button', { name: 'Edit Buy milk' }).click();
	await page.getByLabel('Title', { exact: true }).fill('Buy oat milk');
	await page.getByLabel('Description').fill('For breakfast');
	await page.getByLabel('Priority').selectOption('3');
	await page.getByLabel('Due date').fill(todayAt(23, 59));
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByText('Buy oat milk')).toBeVisible();
	await expect(page.getByText('High')).toBeVisible();

	await page.getByRole('link', { name: 'Projects' }).click();
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Projects');
	await page.getByLabel('Project name').fill('Work');
	await page.getByRole('button', { name: 'Create' }).click();
	await page.getByRole('link', { name: 'Work' }).click();
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Work');
	await page.getByLabel('Task title').fill('Plan sprint');
	await page.getByRole('button', { name: 'Add task' }).click();
	await page.getByRole('button', { name: 'Edit Plan sprint' }).click();
	await page.getByLabel('Project').selectOption('');
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByText('Plan sprint')).toHaveCount(0);

	await page.getByRole('link', { name: 'Inbox' }).click();
	await expect(page.getByText('Plan sprint')).toBeVisible();

	await page.getByRole('link', { name: 'Today' }).click();
	await expect(page).toHaveURL(/\/today$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Today');
	await expect(page.getByText('Buy oat milk')).toBeVisible();

	await page.getByRole('button', { name: 'Complete Buy oat milk' }).click();
	await expect(page.getByRole('button', { name: 'Reopen Buy oat milk' })).toBeVisible();
	await page.getByRole('button', { name: 'Reopen Buy oat milk' }).click();
	await expect(page.getByRole('button', { name: 'Complete Buy oat milk' })).toBeVisible();
	await page.getByRole('button', { name: 'Delete Buy oat milk' }).click();
	await expect(page.getByText('Buy oat milk')).toHaveCount(0);

	await page.getByRole('button', { name: 'Log out' }).click();
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Welcome back.');
});

function testProject(overrides: Partial<Project> = {}): Project {
	return {
		id: 'project-id',
		name: 'Project',
		position: 1024,
		version: 1,
		archivedAt: null,
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'task-id',
		projectId: null,
		parentId: null,
		title: 'Task',
		description: null,
		status: 'active',
		priority: 0,
		dueAt: null,
		dueTimezone: null,
		position: 1024,
		version: 1,
		completedAt: null,
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function todayAt(hours: number, minutes: number): string {
	const date = new Date();
	date.setHours(hours, minutes, 0, 0);
	const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
	return local.toISOString().slice(0, 16);
}
