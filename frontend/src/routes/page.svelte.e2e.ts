import { expect, test } from '@playwright/test';
import type { ActivityEvent } from '$lib/activity/client';
import type { Project, ProjectSection } from '$lib/projects/client';
import type { Task, TaskComment } from '$lib/tasks/client';

test('supports login, Inbox, project Tasks, Today, and logout', async ({ page }) => {
	page.on('pageerror', (error) => {
		throw error;
	});
	const primaryModifier = process.platform === 'darwin' ? 'Meta' : 'Control';
	let authenticated = false;
	let tasks: Task[] = [];
	let comments: TaskComment[] = [];
	let projects: Project[] = [];
	let sections: ProjectSection[] = [];
	let taskCreatePayloads: Record<string, unknown>[] = [];
	let inboxLoads = 0;
	let realtimeEventReady = false;
	let realtimeEventDelivered = false;
	const activityEvents: ActivityEvent[] = [
		{
			streamOffset: 1,
			id: 'activity-1',
			type: 'task.updated',
			occurredAt: new Date().toISOString(),
			actorType: 'user',
			actorId: 'user-id',
			source: 'web',
			aggregateType: 'task',
			aggregateId: 'task-1',
			correlationId: 'correlation-id',
			agentRunId: null,
			payload: { schemaVersion: 1, after: { title: 'Buy oat milk' } }
		}
	];

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
	await page.route('**/api/settings', async (route) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				settings: {
					timezone: 'Europe/Moscow',
					agentModel: 'gpt-5.4',
					agentThinkingEffort: 'medium',
					version: 1,
					createdAt: new Date().toISOString(),
					updatedAt: new Date().toISOString(),
					lastModifiedBy: 'user-e2e'
				},
				availableAgentModels: ['gpt-5.4'],
				availableAgentThinkingEfforts: ['off', 'low', 'medium', 'high']
			})
		});
	});
	await page.route('**/api/agent/sessions', async (route) => {
		await route.fulfill({
			status: 201,
			contentType: 'application/json',
			body: JSON.stringify({
				id: 'session-e2e',
				createdAt: new Date().toISOString(),
				updatedAt: new Date().toISOString()
			})
		});
	});
	await page.route('**/api/agent/sessions/session-e2e/events', async (route) => {
		await route.fulfill({ status: 200, contentType: 'text/event-stream', body: '' });
	});
	await page.route('**/api/activity/changes*', async (route) => {
		const after = new URL(route.request().url()).searchParams.get('after');
		const shouldDeliver = after === '1' && realtimeEventReady && !realtimeEventDelivered;
		realtimeEventDelivered ||= shouldDeliver;
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				cursor: shouldDeliver ? 2 : Number(after ?? 1),
				events: shouldDeliver
					? [{ streamOffset: 2, id: 'realtime-event', type: 'task.created' }]
					: []
			})
		});
	});
	await page.route('**/api/activity?*', async (route) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ events: activityEvents })
		});
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
	await page.route('**/api/views/projects/*/inbox?*', async (route) => {
		const projectId = new URL(route.request().url()).pathname.split('/').at(-2);
		inboxLoads += 1;
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				tasks: tasks.filter(
					(item) =>
						item.parentId === null && item.projectId === projectId && item.sectionId === null
				)
			})
		});
	});
	await page.route('**/api/views/projects/*/today?*', async (route) => {
		const projectId = new URL(route.request().url()).pathname.split('/').at(-2);
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				tasks: tasks.filter(
					(item) => item.parentId === null && item.projectId === projectId && item.dueDate !== null
				)
			})
		});
	});
	await page.route('**/api/views/projects/*?*', async (route) => {
		const projectId = new URL(route.request().url()).pathname.split('/').at(-1);
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				tasks: tasks.filter((item) => item.parentId === null && item.projectId === projectId)
			})
		});
	});
	await page.route('**/api/tasks', async (route) => {
		const request = route.request().postDataJSON();
		taskCreatePayloads = [...taskCreatePayloads, request];
		const parent = tasks.find((item) => item.id === request.parentId);
		const created = testTask({
			id: `task-${tasks.length + 1}`,
			title: request.title,
			projectId: parent?.projectId ?? request.projectId,
			sectionId: parent?.sectionId ?? request.sectionId ?? null,
			parentId: parent?.id ?? null,
			description: request.description ?? null,
			priority: request.priority ?? 0,
			dueDate: request.dueDate ?? null,
			dueTime: request.dueTime ?? null,
			dueTimezone: request.dueTimezone ?? null
		});
		tasks = [...tasks, created];
		await route.fulfill({
			status: 201,
			contentType: 'application/json',
			body: JSON.stringify(created)
		});
	});
	await page.route('**/api/tasks/search?*', async (route) => {
		const parameters = new URL(route.request().url()).searchParams;
		const query = (parameters.get('query') ?? '').trim().toLocaleLowerCase();
		const projectId = parameters.get('project_id');
		const status = parameters.get('status');
		const limit = Number(parameters.get('limit') ?? 20);
		const results = tasks
			.filter(
				(item) =>
					item.parentId === null &&
					item.projectId === projectId &&
					(status === null || item.status === status) &&
					(item.title.toLocaleLowerCase().includes(query) ||
						(item.description ?? '').toLocaleLowerCase().includes(query))
			)
			.sort(
				(left, right) => Number(left.status === 'completed') - Number(right.status === 'completed')
			)
			.slice(0, limit);
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ tasks: results })
		});
	});
	await page.route('**/api/tasks/*/complete', async (route) => {
		const taskId = new URL(route.request().url()).pathname.split('/').at(-2);
		const updated = tasks.find((item) => item.id === taskId)!;
		const request = route.request().postDataJSON();
		if (request.version !== updated.version) {
			await route.fulfill({ status: 409 });
			return;
		}
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
		const request = route.request().postDataJSON();
		if (request.version !== updated.version) {
			await route.fulfill({ status: 409 });
			return;
		}
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
		if (method === 'GET') {
			if (taskId === 'search') {
				await route.fallback();
				return;
			}
			const found = tasks.find((item) => item.id === taskId);
			await route.fulfill({
				status: found ? 200 : 404,
				contentType: 'application/json',
				body: found ? JSON.stringify(found) : JSON.stringify({ message: 'not found' })
			});
			return;
		}
		if (method === 'PATCH') {
			const changes = route.request().postDataJSON();
			const updated = tasks.find((item) => item.id === taskId)!;
			if (changes.version !== updated.version) {
				await route.fulfill({ status: 409 });
				return;
			}

			const movedProject = changes.projectId && changes.projectId !== updated.projectId;
			Object.assign(updated, changes, {
				sectionId: movedProject
					? null
					: Object.hasOwn(changes, 'sectionId')
						? changes.sectionId
						: updated.sectionId,
				version: updated.version + 1
			});
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

		const request = route.request().postDataJSON();
		const deleted = tasks.find((item) => item.id === taskId)!;
		if (request.version !== deleted.version) {
			await route.fulfill({ status: 409 });
			return;
		}
		tasks = tasks.filter((item) => item.id !== taskId);
		await route.fulfill({ status: 204 });
	});
	await page.route('**/api/tasks/*/subtasks', async (route) => {
		const taskId = new URL(route.request().url()).pathname.split('/').at(-2);
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ tasks: tasks.filter((item) => item.parentId === taskId) })
		});
	});
	await page.route('**/api/tasks/*/comments', async (route) => {
		const taskId = new URL(route.request().url()).pathname.split('/').at(-2)!;
		if (route.request().method() === 'POST') {
			const request = route.request().postDataJSON();
			const created = testComment({
				id: `comment-${comments.length + 1}`,
				taskId,
				body: request.body
			});
			comments = [...comments, created];
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
			body: JSON.stringify({ comments: comments.filter((item) => item.taskId === taskId) })
		});
	});
	await page.route('**/api/tasks/*/comments/*', async (route) => {
		const path = new URL(route.request().url()).pathname.split('/');
		const commentId = path.at(-1)!;
		const existing = comments.find((item) => item.id === commentId)!;
		const request = route.request().postDataJSON();
		if (request.version !== existing.version) {
			await route.fulfill({ status: 409 });
			return;
		}
		if (route.request().method() === 'DELETE') {
			comments = comments.filter((item) => item.id !== commentId);
			await route.fulfill({ status: 204 });
			return;
		}
		Object.assign(existing, { body: request.body, version: existing.version + 1 });
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(existing)
		});
	});

	await page.goto('/');
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Welcome back.');

	await page.getByLabel('Username').fill('owner');
	await page.getByLabel('Password').fill('correct horse battery staple');
	await page.getByRole('button', { name: 'Sign in' }).click();

	await expect(page).toHaveURL(/\/projects$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Projects');
	await page.getByLabel('Project name').fill('Personal');
	await page.getByRole('button', { name: 'Create' }).click();
	await expect(page).toHaveURL(/\/projects\/project-1$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Inbox');
	await expect.poll(() => inboxLoads).toBeGreaterThanOrEqual(2);
	tasks = [
		...tasks,
		testTask({ id: 'task-from-server', projectId: 'project-1', title: 'Synced from server' })
	];
	realtimeEventReady = true;
	await expect(page.getByText('Synced from server')).toBeVisible();
	await expect.poll(() => inboxLoads).toBeGreaterThanOrEqual(3);

	await page.getByLabel('Task title').fill('Buy milk');
	await page.getByRole('button', { name: 'Add task' }).click();
	await expect(page.getByText('Buy milk')).toBeVisible();

	await page.getByRole('button', { name: 'Edit Buy milk' }).click();
	await expect(page).toHaveURL(/\/projects\/project-1\/tasks\/task-2$/);
	await page.getByRole('textbox', { name: 'Add a subtask' }).fill('Compare brands');
	await page.getByRole('button', { name: 'Add subtask' }).click();
	await expect(page.getByText('Compare brands', { exact: true })).toBeVisible();
	await page.getByRole('button', { name: 'Complete Compare brands' }).click();
	await expect(page.getByRole('button', { name: 'Reopen Compare brands' })).toBeVisible();
	await page.getByRole('textbox', { name: 'Add a comment' }).fill('Prefer an unsweetened option.');
	await page.getByRole('button', { name: 'Send comment' }).click();
	await expect(page.getByText('Prefer an unsweetened option.', { exact: true })).toBeVisible();
	const milkDialog = page.getByRole('dialog', { name: 'Edit task: Buy milk' });
	await milkDialog.getByLabel('Title', { exact: true }).fill('Buy oat milk');
	await milkDialog.getByLabel('Description').fill('For breakfast');
	await milkDialog.getByRole('button', { name: 'Priority: None' }).click();
	await milkDialog.getByRole('option', { name: 'High' }).click();
	await milkDialog.getByRole('button', { name: 'Due date: No date' }).click();
	await milkDialog.getByRole('option', { name: /^Tomorrow/ }).click();
	await milkDialog.getByRole('button', { name: 'Due time: + Time' }).click();
	await milkDialog.getByRole('option', { name: /^Morning/ }).click();
	await milkDialog.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByText('Buy oat milk')).toBeVisible();
	await expect(
		page.getByRole('button', { name: 'Open Buy oat milk' }).getByText('High')
	).toBeVisible();
	expect(tasks.find((item) => item.title === 'Buy oat milk')).toMatchObject({
		dueDate: tomorrowDate(),
		dueTime: '09:00',
		priority: 3
	});

	await page.getByRole('link', { name: 'Manage projects' }).click();
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Projects');
	await page.getByLabel('Project name').fill('Work');
	await page.getByRole('button', { name: 'Create' }).click();
	await expect(page).toHaveURL(/\/projects\/project-2$/);
	await page.getByRole('link', { name: 'Tasks' }).click();
	await expect(page).toHaveURL(/\/projects\/project-2\/tasks$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Tasks');
	const addSection = page.getByRole('group', { name: 'Add or move section' });
	await page.getByLabel('Section name').fill('Planning');
	await addSection.getByRole('button', { name: 'Add', exact: true }).click();
	await page.getByRole('combobox', { name: 'Add task to Planning' }).fill('Plan sprint');
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
	await expect(page).toHaveURL(/\/projects\/project-2\/tasks$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Tasks');
	await expect(page.getByText('Buy oat milk')).toHaveCount(0);
	await expect(page.getByText('Plan sprint')).toBeVisible();

	await page.keyboard.press(`${primaryModifier}+K`);
	await expect(page.getByRole('dialog', { name: 'Command palette' })).toBeVisible();
	await page.keyboard.type('Today');
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(/\/projects\/project-2\/today$/);

	await page.keyboard.press(`${primaryModifier}+K`);
	await page.keyboard.type('Personal');
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(/\/projects\/project-1/);

	await page.keyboard.press(`${primaryModifier}+K`);
	await page.keyboard.type('Buy oat milk');
	await expect(page.getByRole('option', { name: /Buy oat milk/ })).toBeVisible();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('dialog', { name: 'Edit task: Buy oat milk' })).toBeVisible();
	await page.keyboard.press('Escape');

	await page.keyboard.press(`${primaryModifier}+K`);
	await page.keyboard.type('Work');
	await page.keyboard.press('End');
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(/\/projects\/project-2\/today$/);
	await page.keyboard.press(`${primaryModifier}+4`);
	await expect(page).toHaveURL(/\/projects\/project-2\/tasks$/);

	await page.getByRole('button', { name: 'Edit Plan sprint' }).click();
	const sprintDialog = page.getByRole('dialog', { name: 'Edit task: Plan sprint' });
	await sprintDialog.getByRole('button', { name: /^Location:/ }).click();
	await sprintDialog.getByRole('button', { name: 'Project: Work', exact: true }).click();
	await sprintDialog.getByRole('option', { name: 'Personal' }).click();
	await sprintDialog.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByText('Plan sprint')).toHaveCount(0);

	await page.evaluate(() =>
		localStorage.setItem('todai.project.project-1.last-view', '/projects/project-1/tasks')
	);
	await page
		.locator('aside')
		.getByRole('combobox', { name: 'Project', exact: true })
		.selectOption('project-1');
	await expect(page).toHaveURL(/\/projects\/project-1\/tasks$/);
	await expect(page.getByText('Plan sprint')).toBeVisible();
	const planSprint = tasks.find((item) => item.title === 'Plan sprint')!;
	await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
	await page.getByRole('button', { name: 'Open Plan sprint' }).click();
	await expect(page).toHaveURL(
		new RegExp(`/projects/project-1/tasks/${encodeURIComponent(planSprint.id)}$`)
	);
	await expect(page.getByLabel('Project', { exact: true })).toHaveValue('project-1');
	await expect(page.getByRole('dialog', { name: 'Edit task: Plan sprint' })).toBeVisible();
	await page.getByRole('button', { name: 'Copy link' }).click();
	await expect(page.getByText('Link copied', { exact: true })).toBeVisible();
	expect(await page.evaluate(() => navigator.clipboard.readText())).toMatch(
		new RegExp(`/projects/project-1/tasks/${encodeURIComponent(planSprint.id)}$`)
	);
	await page.goBack();
	await expect(page).toHaveURL(/\/projects\/project-1\/tasks$/);
	await expect(page.getByRole('dialog', { name: 'Edit task: Plan sprint' })).toHaveCount(0);
	await page.goForward();
	await expect(page.getByRole('dialog', { name: 'Edit task: Plan sprint' })).toBeVisible();
	await page.reload();
	await expect(page.getByRole('dialog', { name: 'Edit task: Plan sprint' })).toBeVisible();
	await page.getByRole('button', { name: 'Close task editor' }).click();
	await expect(page).toHaveURL(/\/projects\/project-1\/tasks$/);

	await page.goto(`/projects/project-2/tasks/${encodeURIComponent(planSprint.id)}`);
	await expect(page).toHaveURL(
		new RegExp(`/projects/project-1/tasks/${encodeURIComponent(planSprint.id)}$`)
	);
	await expect(page.getByLabel('Project', { exact: true })).toHaveValue('project-1');
	await expect(page.getByRole('dialog', { name: 'Edit task: Plan sprint' })).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(page).toHaveURL(/\/projects\/project-1\/tasks$/);

	const personalSectionName = 'Personal plans';
	const personalSectionForm = page.getByRole('group', { name: 'Add or move section' });
	await page.getByLabel('Section name').fill(personalSectionName);
	await personalSectionForm.getByRole('button', { name: 'Add', exact: true }).click();

	const todayLink = page.getByRole('link', { name: 'Today' });
	await page.keyboard.down(primaryModifier);
	await expect
		.poll(() => todayLink.evaluate((element) => getComputedStyle(element, '::after').opacity))
		.toBe('1');
	await expect
		.poll(() => todayLink.evaluate((element) => getComputedStyle(element, '::after').content))
		.toContain('3');
	await page.keyboard.up(primaryModifier);
	await expect
		.poll(() => todayLink.evaluate((element) => getComputedStyle(element, '::after').opacity))
		.toBe('0');

	await page.keyboard.press(`${primaryModifier}+Alt+N`);
	const quickAdd = page.getByRole('dialog', { name: 'Create a task' });
	await expect(quickAdd).toBeVisible();
	const richTitle = quickAdd.getByRole('combobox', { name: 'Title' });
	await richTitle.fill('Created with the keyboard #Per');
	await expect(
		page.getByRole('listbox', { name: 'project options' }).getByRole('option', { name: 'Personal' })
	).toBeVisible();
	await page.keyboard.press('Enter');
	await expect(richTitle).toHaveValue('Created with the keyboard');
	await expect(
		quickAdd.getByRole('button', { name: 'Location: Personal / No section (Inbox)' })
	).toBeVisible();
	await page.keyboard.type(' /Personal');
	await expect(
		page
			.getByRole('listbox', { name: 'location options' })
			.getByRole('option', { name: `Personal: ${personalSectionName}` })
	).toBeVisible();
	await page.keyboard.press('ArrowDown');
	await page.keyboard.press('Enter');
	await expect(
		quickAdd.getByRole('button', { name: `Location: Personal / ${personalSectionName}` })
	).toBeVisible();
	await page.keyboard.type(' !High');
	await expect(
		page.getByRole('listbox', { name: 'priority options' }).getByRole('option', { name: 'High' })
	).toBeVisible();
	await page.keyboard.press('Enter');
	await expect(quickAdd.getByRole('button', { name: 'Priority: High' })).toBeVisible();
	await page.keyboard.type(' @Today');
	await expect(
		page.getByRole('listbox', { name: 'due options' }).getByRole('option', { name: /^Today/ })
	).toBeVisible();
	await page.keyboard.press('Enter');
	await expect(
		page
			.getByRole('listbox', { name: 'due-time options' })
			.getByRole('option', { name: /^Morning/ })
	).toBeVisible();
	await page.keyboard.press('Enter');
	await page.keyboard.press('Enter');
	await expect(quickAdd).toHaveCount(0);
	const richPayload = taskCreatePayloads.at(-1)!;
	expect(richPayload).toMatchObject({
		title: 'Created with the keyboard',
		projectId: 'project-1',
		sectionId: sections.find((item) => item.name === personalSectionName)?.id,
		priority: 3,
		dueDate: todayDate(),
		dueTime: '09:00'
	});

	await page.keyboard.press(`${primaryModifier}+3`);
	await expect(page).toHaveURL(/\/today$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Today');
	await expect(page.getByText('Created with the keyboard')).toBeVisible();

	await page.keyboard.press(`${primaryModifier}+J`);
	await expect(page.getByRole('dialog', { name: 'Assistant' })).toBeVisible();
	await page.keyboard.press(`${primaryModifier}+J`);
	await expect(page.getByRole('button', { name: 'Open assistant' })).toBeVisible();
	await expect(page.getByText('Buy oat milk')).toBeVisible();

	await page.getByRole('button', { name: 'Complete Buy oat milk' }).click();
	await expect(page.getByRole('button', { name: 'Reopen Buy oat milk' })).toBeVisible();
	await page.getByRole('button', { name: 'Reopen Buy oat milk' }).click();
	await expect(page.getByRole('button', { name: 'Complete Buy oat milk' })).toBeVisible();
	await page.getByRole('button', { name: 'Delete Buy oat milk' }).click();
	await expect(page.getByText('Buy oat milk')).toHaveCount(0);

	await page.getByRole('link', { name: 'Activity' }).click();
	await expect(page).toHaveURL(/\/activity$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Activity');
	await expect(page.getByText('“Buy oat milk”')).toBeVisible();

	await page.getByRole('button', { name: 'Log out' }).click();
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Welcome back.');

	await page.goto(`/projects/project-1/tasks/${encodeURIComponent(planSprint.id)}`);
	await expect(page).toHaveURL(/\/login\?returnTo=/);
	await page.getByLabel('Username').fill('owner');
	await page.getByLabel('Password').fill('correct horse battery staple');
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page).toHaveURL(
		new RegExp(`/projects/project-1/tasks/${encodeURIComponent(planSprint.id)}$`)
	);
	await expect(page.getByRole('dialog', { name: 'Edit task: Plan sprint' })).toBeVisible();
});

function testProject(overrides: Partial<Project> = {}): Project {
	return {
		id: 'project-id',
		name: 'Project',
		layout: 'list',
		colorTheme: 'sage',
		agentModel: 'gpt-default',
		agentThinkingEffort: 'medium',
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
		projectId: 'project-id',
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

function testComment(overrides: Partial<TaskComment> = {}): TaskComment {
	return {
		id: 'comment-id',
		taskId: 'task-id',
		authorId: 'user-id',
		body: 'Comment',
		version: 1,
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z',
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
	return localDate(0);
}

function tomorrowDate(): string {
	return localDate(1);
}

function localDate(offset: number): string {
	const date = new Date();
	date.setDate(date.getDate() + offset);
	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}
