import { expect, test } from '@playwright/test';
import type { Project, ProjectSection } from '$lib/projects/client';
import type { Task } from '$lib/tasks/client';

test('supports login, Inbox, projects, All tasks, Today, and logout', async ({ page }) => {
	let authenticated = false;
	let tasks: Task[] = [];
	let projects: Project[] = [];
	let sections: ProjectSection[] = [];

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
	await page.route('**/api/projects/*/sections', async (route) => {
		const projectId = new URL(route.request().url()).pathname.split('/').at(-2)!;
		if (route.request().method() === 'POST') {
			const request = route.request().postDataJSON();
			const created = testSection({
				id: `section-${sections.length + 1}`,
				projectId,
				name: request.name,
				position: (sections.length + 1) * 1024
			});
			sections = [...sections, created];
			await route.fulfill({
				status: 201,
				contentType: 'application/json',
				body: JSON.stringify(created)
			});
			return;
		}
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ sections: sections.filter((item) => item.projectId === projectId) })
		});
	});
	await page.route('**/api/projects/*/sections/*/reorder', async (route) => {
		const parts = new URL(route.request().url()).pathname.split('/');
		const sectionId = parts.at(-2)!;
		const request = route.request().postDataJSON();
		const moved = sections.find((item) => item.id === sectionId)!;
		const remaining = sections.filter((item) => item.id !== sectionId);
		const index =
			request.beforeSectionId === null
				? remaining.length
				: remaining.findIndex((item) => item.id === request.beforeSectionId);
		remaining.splice(index, 0, moved);
		sections = remaining.map((item, itemIndex) => ({
			...item,
			position: (itemIndex + 1) * 1024,
			version: item.version + 1
		}));
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ sections })
		});
	});
	await page.route('**/api/projects/*/sections/*', async (route) => {
		const sectionId = new URL(route.request().url()).pathname.split('/').at(-1)!;
		if (route.request().method() === 'DELETE') {
			sections = sections.filter((item) => item.id !== sectionId);
			tasks = tasks.map((item) =>
				item.sectionId === sectionId ? { ...item, sectionId: null } : item
			);
			await route.fulfill({ status: 204 });
			return;
		}
		const request = route.request().postDataJSON();
		const updated = sections.find((item) => item.id === sectionId)!;
		Object.assign(updated, request, { version: updated.version + 1 });
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(updated)
		});
	});
	await page.route('**/api/views/inbox?*', async (route) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ tasks })
		});
	});
	await page.route('**/api/views/all?*', async (route) => {
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
			body: JSON.stringify({ tasks: tasks.filter((item) => item.dueDate !== null) })
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
			projectId: request.projectId ?? null,
			sectionId: request.sectionId ?? null
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
	await page.route('**/api/tasks/*/reorder', async (route) => {
		const taskId = new URL(route.request().url()).pathname.split('/').at(-2)!;
		const request = route.request().postDataJSON();
		const moved = tasks.find((item) => item.id === taskId)!;
		const remaining = tasks.filter((item) => item.id !== taskId);
		const destination = remaining
			.filter(
				(item) =>
					item.projectId === moved.projectId &&
					item.sectionId === request.sectionId &&
					item.status === 'active'
			)
			.sort((left, right) => left.position - right.position);
		const index =
			request.beforeTaskId === null
				? destination.length
				: destination.findIndex((item) => item.id === request.beforeTaskId);
		destination.splice(index, 0, { ...moved, sectionId: request.sectionId });
		const positions = new Map(
			destination.map((item, itemIndex) => [item.id, (itemIndex + 1) * 1024])
		);
		tasks = [...remaining, { ...moved, sectionId: request.sectionId }].map((item) =>
			positions.has(item.id)
				? { ...item, position: positions.get(item.id)!, version: item.version + 1 }
				: item
		);
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ tasks: tasks.filter((item) => item.projectId === moved.projectId) })
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
	await page.getByLabel('Due date').fill(todayDate());
	await page.getByLabel('Due time').fill('23:59');
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByText('Buy oat milk')).toBeVisible();
	await expect(page.getByText('High')).toBeVisible();

	await page.getByRole('link', { name: 'Projects', exact: true }).click();
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Projects');
	await page.getByLabel('Project name').fill('Work');
	await page.getByRole('button', { name: 'Create' }).click();
	await page.locator('.workspace').getByRole('link', { name: 'Work' }).click();
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Work');
	const addSection = page.getByRole('group', { name: 'Add or move section' });
	await page.getByLabel('Section name').fill('Planning');
	await addSection.getByRole('button', { name: 'Add', exact: true }).click();
	await page.getByRole('textbox', { name: 'Add task to Planning' }).fill('Plan sprint');
	await page.getByRole('button', { name: 'Add task to Planning' }).click();
	await page.getByLabel('Section name').fill('Later');
	await addSection.getByRole('button', { name: 'Add', exact: true }).click();
	const planningTasks = page.getByRole('list', { name: 'Planning tasks' });
	const laterTasks = page.getByRole('list', { name: 'Later tasks' });
	await planningTasks
		.getByRole('listitem')
		.filter({ hasText: 'Plan sprint' })
		.dragTo(laterTasks.getByLabel('Drop task in Later'));
	await expect(laterTasks.getByText('Plan sprint')).toBeVisible();
	await page.getByRole('button', { name: 'Board' }).click();
	await expect(page.getByRole('button', { name: 'Board' })).toHaveAttribute('aria-pressed', 'true');
	await laterTasks
		.getByRole('listitem')
		.filter({ hasText: 'Plan sprint' })
		.dragTo(planningTasks.getByLabel('Drop task in Planning'));
	await expect(planningTasks.getByText('Plan sprint')).toBeVisible();
	await page.getByRole('link', { name: 'All tasks' }).click();
	await expect(page).toHaveURL(/\/all$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('All tasks');
	await expect(page.getByText('Buy oat milk')).toBeVisible();
	await expect(page.getByText('Plan sprint')).toBeVisible();

	await page.getByRole('link', { name: 'Work' }).click();
	await page.getByRole('button', { name: 'Edit Plan sprint' }).click();
	await page.getByRole('combobox', { name: 'Project', exact: true }).selectOption('');
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

function testTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'task-id',
		projectId: null,
		sectionId: null,
		parentId: null,
		title: 'Task',
		description: null,
		status: 'active',
		priority: 0,
		dueDate: null,
		dueTime: null,
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

function testSection(overrides: Partial<ProjectSection> = {}): ProjectSection {
	return {
		id: 'section-id',
		projectId: 'project-id',
		name: 'Section',
		position: 1024,
		version: 1,
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function todayDate(): string {
	const date = new Date();
	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}
