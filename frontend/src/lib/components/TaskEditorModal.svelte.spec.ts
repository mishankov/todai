import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { ActivityEvent } from '$lib/activity/client';
import type { Project, ProjectSection } from '$lib/projects/client';
import { publishActivityEvent } from '$lib/realtime/events';
import type { Task, TaskComment, TaskUpdate } from '$lib/tasks/client';
import TaskEditorModal from './TaskEditorModal.svelte';

describe('TaskEditorModal relationships', () => {
	it('shows task metadata as one compact property bar', async () => {
		const project = testProject({ name: 'Работа' });
		const section = testSection({ projectId: project.id, name: 'Задачи' });
		const task = testTask({ projectId: project.id, sectionId: section.id });

		renderModal({
			task,
			projects: [project],
			sections: [section]
		});

		await expect.element(page.getByRole('group', { name: 'Task properties' })).toBeVisible();
		const location = page.getByRole('button', { name: 'Location: Работа / Задачи' });
		await expect.element(location).toBeVisible();
		await location.click();
		await expect
			.element(page.getByRole('button', { name: 'Project: Работа', exact: true }))
			.toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Section: Задачи', exact: true }))
			.toBeVisible();
		await location.click();
		await expect.element(page.getByRole('button', { name: 'Priority: None' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Due date: No date' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Due time: + Time' })).toBeDisabled();

		const properties = page.getByRole('group', { name: 'Task properties' }).element();
		expect(properties.children).toHaveLength(4);

		await page.getByRole('button', { name: 'Due date: No date' }).click();
		await page.getByRole('option', { name: /^Tomorrow/ }).click();
		await expect.element(page.getByRole('button', { name: /^Due time:/ })).toBeEnabled();
	});

	it('loads subtasks and comments into accessible sections', async () => {
		const task = testTask({ title: 'Plan release' });
		const active = testTask({ id: 'active-child', parentId: task.id, title: 'Draft notes' });
		const completed = testTask({
			id: 'done-child',
			parentId: task.id,
			title: 'Review scope',
			status: 'completed',
			completedAt: '2026-07-19T11:00:00Z'
		});
		const comment = testComment({ taskId: task.id, body: 'Keep the rollout gradual.' });
		const subtasksRequest = deferred<Task[]>();
		const commentsRequest = deferred<TaskComment[]>();
		const loadSubtasks = vi.fn(() => subtasksRequest.promise);
		const loadComments = vi.fn(() => commentsRequest.promise);

		renderModal({ task, loadSubtasks, loadComments });

		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.toHaveAttribute('aria-modal', 'true');
		await expect.element(page.getByRole('heading', { name: 'Subtasks' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'Comments' })).toBeVisible();
		await expect.element(page.getByRole('textbox', { name: 'Add a subtask' })).toBeVisible();
		await expect.element(page.getByRole('textbox', { name: 'Add a comment' })).toBeVisible();
		await expect
			.element(page.getByText('Loading subtasks…', { exact: true }))
			.toHaveAttribute('role', 'status');
		await expect
			.element(page.getByText('Loading comments…', { exact: true }))
			.toHaveAttribute('role', 'status');
		await expect
			.element(page.getByRole('region', { name: 'Subtasks' }))
			.toHaveAttribute('aria-busy', 'true');
		await expect
			.element(page.getByRole('complementary', { name: 'Comments' }))
			.toHaveAttribute('aria-busy', 'true');

		subtasksRequest.resolve([active, completed]);
		commentsRequest.resolve([comment]);
		await expect.element(page.getByText(active.title, { exact: true })).toBeVisible();
		await expect.element(page.getByText(completed.title, { exact: true })).toBeVisible();
		await expect.element(page.getByText('1 of 2 complete', { exact: true })).toBeVisible();
		await expect.element(page.getByText(comment.body, { exact: true })).toBeVisible();
		expect(loadSubtasks).toHaveBeenCalledWith(task.id);
		expect(loadComments).toHaveBeenCalledWith(task.id);
	});

	it('creates a subtask without saving or closing the parent task editor', async () => {
		const task = testTask({ title: 'Plan release' });
		const child = testTask({ id: 'child-id', parentId: task.id, title: 'Draft notes' });
		const save = vi.fn(async () => {});
		const addSubtask = vi.fn(async () => child);

		renderModal({ task, save, addSubtask });
		const input = page.getByRole('textbox', { name: 'Add a subtask' });
		await input.fill(child.title);
		await page.getByRole('button', { name: 'Add subtask' }).click();

		expect(addSubtask).toHaveBeenCalledWith(child.title);
		expect(save).not.toHaveBeenCalled();
		await expect.element(page.getByText(child.title, { exact: true })).toBeVisible();
		await expect.element(input).toHaveValue('');
		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.toBeVisible();
	});

	it('decomposes inside the open task card and refreshes its subtasks', async () => {
		const task = testTask({ title: 'Plan release' });
		const child = testTask({ id: 'child-id', parentId: task.id, title: 'Define rollout' });
		const request = deferred<void>();
		const decomposeTask = vi.fn(() => request.promise);
		let loaded = false;
		const loadSubtasks = vi.fn(async () => (loaded ? [child] : []));

		renderModal({ task, decomposeTask, loadSubtasks });
		await expect.element(page.getByText('No subtasks yet.', { exact: true })).toBeVisible();
		await page.getByRole('button', { name: 'Decompose' }).click();

		expect(decomposeTask).toHaveBeenCalledWith(task);
		await expect.element(page.getByRole('button', { name: 'Decomposing…' })).toBeDisabled();
		const details = document.querySelector<HTMLElement>('.details-column');
		expect(details).not.toBeNull();
		expect(details!.scrollWidth).toBeLessThanOrEqual(details!.clientWidth + 1);
		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.toBeVisible();

		loaded = true;
		request.resolve();
		await expect.element(page.getByText(child.title, { exact: true })).toBeVisible();
		await expect.element(page.getByText('Decomposition complete.', { exact: true })).toBeVisible();
		expect(loadSubtasks).toHaveBeenCalledTimes(2);
	});

	it('deletes a subtask with its observed version and keeps the editor open', async () => {
		const task = testTask({ title: 'Plan release' });
		const child = testTask({ id: 'child-id', parentId: task.id, title: 'Draft notes' });
		const removeSubtask = vi.fn(async () => {});

		renderModal({
			task,
			loadSubtasks: vi.fn(async () => [child]),
			removeSubtask
		});
		await expect.element(page.getByText(child.title, { exact: true })).toBeVisible();
		await page.getByRole('button', { name: `Delete ${child.title}` }).click();

		expect(removeSubtask).toHaveBeenCalledWith(child.id, child.version);
		await expect.element(page.getByText(child.title, { exact: true })).not.toBeInTheDocument();
		await expect.element(page.getByText('0 of 0 complete', { exact: true })).toBeVisible();
		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.toBeVisible();
	});

	it('completes and reopens a subtask with its observed version', async () => {
		const task = testTask({ title: 'Plan release' });
		const child = testTask({ id: 'child-id', parentId: task.id, title: 'Draft notes' });
		const completed = testTask({
			...child,
			status: 'completed',
			version: 2,
			completedAt: '2026-07-19T11:00:00Z'
		});
		const reopened = testTask({ ...child, version: 3 });
		const completeSubtask = vi.fn(async () => completed);
		const reopenSubtask = vi.fn(async () => reopened);

		renderModal({
			task,
			loadSubtasks: vi.fn(async () => [child]),
			completeSubtask,
			reopenSubtask
		});
		await page.getByRole('button', { name: `Complete ${child.title}` }).click();

		expect(completeSubtask).toHaveBeenCalledWith(child.id, child.version);
		await page.getByRole('button', { name: `Reopen ${child.title}` }).click();

		expect(reopenSubtask).toHaveBeenCalledWith(completed.id, completed.version);
		await expect
			.element(page.getByRole('button', { name: `Complete ${child.title}` }))
			.toBeVisible();
	});

	it('submits a comment independently and clears its composer', async () => {
		const task = testTask();
		const created = testComment({ taskId: task.id, body: 'Keep the rollout gradual.' });
		const save = vi.fn(async () => {});
		const addComment = vi.fn(async () => created);

		renderModal({ task, save, addComment });
		const input = page.getByRole('textbox', { name: 'Add a comment' });
		await input.fill(created.body);
		await page.getByRole('button', { name: 'Send comment' }).click();

		expect(addComment).toHaveBeenCalledWith(created.body);
		expect(save).not.toHaveBeenCalled();
		await expect.element(page.getByText(created.body, { exact: true })).toBeVisible();
		await expect.element(input).toHaveValue('');
	});

	it('deletes a comment with its observed version', async () => {
		const task = testTask();
		const comment = testComment({ taskId: task.id });
		const removeComment = vi.fn(async () => {});

		renderModal({
			task,
			loadComments: vi.fn(async () => [comment]),
			removeComment
		});
		await expect.element(page.getByText(comment.body, { exact: true })).toBeVisible();
		await page.getByRole('button', { name: 'Delete comment' }).click();

		expect(removeComment).toHaveBeenCalledWith(comment.id, comment.version);
		await expect.element(page.getByText(comment.body, { exact: true })).not.toBeInTheDocument();
	});

	it('keeps editors usable and reports relationship request failures', async () => {
		const task = testTask();
		const addSubtask = vi.fn(async () => {
			throw new Error('create failed');
		});

		renderModal({
			task,
			loadSubtasks: vi.fn(async () => {
				throw new Error('subtasks failed');
			}),
			loadComments: vi.fn(async () => {
				throw new Error('comments failed');
			}),
			addSubtask
		});
		const input = page.getByRole('textbox', { name: 'Add a subtask' });
		await input.fill('Retry this');
		await page.getByRole('button', { name: 'Add subtask' }).click();

		await expect.element(input).toHaveValue('Retry this');
		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.toBeVisible();
		const alerts = Array.from(document.querySelectorAll<HTMLElement>('[role="alert"]')).map(
			(element) => element.textContent?.toLowerCase() ?? ''
		);
		expect(alerts.some((message) => message.includes('subtask'))).toBe(true);
		expect(alerts.some((message) => message.includes('comment'))).toBe(true);
	});

	it('refreshes open relationships after related task activity', async () => {
		const task = testTask();
		const remoteSubtask = testTask({
			id: 'remote-child',
			parentId: task.id,
			title: 'Added by the assistant'
		});
		const remoteComment = testComment({
			id: 'remote-comment',
			taskId: task.id,
			body: 'Added in another client'
		});
		let reloaded = false;
		const loadSubtasks = vi.fn(async () => (reloaded ? [remoteSubtask] : []));
		const loadComments = vi.fn(async () => (reloaded ? [remoteComment] : []));

		renderModal({ task, loadSubtasks, loadComments });
		await expect.element(page.getByText('No subtasks yet.', { exact: true })).toBeVisible();
		reloaded = true;
		publishActivityEvent(testActivity({ aggregateId: task.id }));

		await expect.element(page.getByText(remoteSubtask.title, { exact: true })).toBeVisible();
		await expect.element(page.getByText(remoteComment.body, { exact: true })).toBeVisible();
		expect(loadSubtasks).toHaveBeenCalledTimes(2);
		expect(loadComments).toHaveBeenCalledTimes(2);
	});
});

interface ModalOverrides {
	task?: Task;
	projects?: Project[];
	sections?: ProjectSection[];
	loadSections?: (projectId: string) => Promise<ProjectSection[]>;
	save?: (update: TaskUpdate) => Promise<void>;
	loadSubtasks?: (taskId: string) => Promise<Task[]>;
	addSubtask?: (title: string) => Promise<Task>;
	completeSubtask?: (taskId: string, version: number) => Promise<Task>;
	reopenSubtask?: (taskId: string, version: number) => Promise<Task>;
	removeSubtask?: (taskId: string, version: number) => Promise<void>;
	loadComments?: (taskId: string) => Promise<TaskComment[]>;
	addComment?: (body: string) => Promise<TaskComment>;
	removeComment?: (commentId: string, version: number) => Promise<void>;
	decomposeTask?: (task: Task) => Promise<void>;
}

function renderModal(overrides: ModalOverrides = {}) {
	const task = overrides.task ?? testTask();
	return render(TaskEditorModal, {
		task,
		projects: overrides.projects ?? [],
		sections: overrides.sections,
		loadSections: overrides.loadSections,
		save: overrides.save ?? vi.fn(async () => {}),
		close: vi.fn(),
		loadSubtasks: overrides.loadSubtasks ?? vi.fn(async () => []),
		addSubtask: overrides.addSubtask ?? vi.fn(),
		completeSubtask: overrides.completeSubtask ?? vi.fn(),
		reopenSubtask: overrides.reopenSubtask ?? vi.fn(),
		removeSubtask: overrides.removeSubtask ?? vi.fn(),
		loadComments: overrides.loadComments ?? vi.fn(async () => []),
		addComment: overrides.addComment ?? vi.fn(),
		removeComment: overrides.removeComment ?? vi.fn(),
		decomposeTask: overrides.decomposeTask ?? vi.fn(async () => {})
	});
}

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
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
} {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((settle) => {
		resolve = settle;
	});
	return { promise, resolve };
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
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testComment(overrides: Partial<TaskComment> = {}): TaskComment {
	return {
		id: 'comment-id',
		taskId: 'task-id',
		authorId: 'user-id',
		body: 'Initial note',
		version: 1,
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testActivity(overrides: Partial<ActivityEvent> = {}): ActivityEvent {
	return {
		streamOffset: 1,
		id: 'event-id',
		type: 'task.updated',
		occurredAt: '2026-07-19T10:00:00Z',
		actorType: 'external_agent',
		actorId: 'agent-id',
		source: 'internal_api',
		aggregateType: 'task',
		aggregateId: 'task-id',
		correlationId: 'correlation-id',
		agentRunId: null,
		payload: {},
		...overrides
	};
}
