import { page, userEvent, type Locator } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Task, TaskCreateDraft, TaskSummary, TaskUpdate } from '$lib/tasks/client';
import type { Project, ProjectSection } from './client';
import ProjectTasks from './ProjectTasks.svelte';

describe('ProjectTasks', () => {
	it('shows project sections and their tasks in list layout', async () => {
		renderProjectTasks({
			sections: [
				testSection({ id: 'later', name: 'Later', position: 2048 }),
				testSection({ id: 'doing', name: 'Doing', position: 1024 })
			],
			tasks: [
				testTask({ id: 'unsectioned', title: 'Triage request' }),
				testTask({ id: 'draft', title: 'Draft proposal', sectionId: 'doing' }),
				testTask({ id: 'review', title: 'Review proposal', sectionId: 'later' })
			]
		});

		await expect.element(page.getByRole('button', { name: 'List', pressed: true })).toBeVisible();
		await expect
			.element(page.getByRole('region', { name: 'No section' }).getByText('Triage request'))
			.toBeVisible();
		await expect
			.element(page.getByRole('region', { name: 'Doing' }).getByText('Draft proposal'))
			.toBeVisible();
		await expect
			.element(page.getByRole('region', { name: 'Later' }).getByText('Review proposal'))
			.toBeVisible();
	});

	it('omits the unsectioned group when every active task belongs to a section', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing', position: 1024 });
		const later = testSection({ id: 'later', name: 'Later', position: 2048 });
		renderProjectTasks({
			sections: [doing, later],
			tasks: [
				testTask({ id: 'draft', title: 'Draft proposal', sectionId: doing.id }),
				testTask({ id: 'review', title: 'Review proposal', sectionId: later.id })
			]
		});

		await expect.element(page.getByRole('region', { name: 'Doing' })).toBeVisible();
		await expect.element(page.getByRole('region', { name: 'Later' })).toBeVisible();
		await expect.element(page.getByRole('region', { name: 'No section' })).not.toBeInTheDocument();
		await expect.element(page.getByRole('heading', { name: 'No section' })).not.toBeInTheDocument();
	});

	it('keeps unsectioned tasks and quick-add without a redundant heading when no sections exist', async () => {
		const task = testTask({ id: 'triage', title: 'Triage request' });
		renderProjectTasks({ tasks: [task] });

		await expect.element(page.getByText(task.title, { exact: true })).toBeVisible();
		await expect
			.element(page.getByRole('combobox', { name: 'Add task to No section' }))
			.toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'No section' })).not.toBeInTheDocument();
	});

	it('switches to board layout and persists the choice with the observed version', async () => {
		const project = testProject();
		const changed = testProject({ layout: 'board', version: 2 });
		const changeLayout = vi.fn(async () => changed);
		renderProjectTasks({ project, changeLayout });

		await page.getByRole('button', { name: 'Board' }).click();

		expect(changeLayout).toHaveBeenCalledWith(project.version, 'board');
		await expect.element(page.getByRole('button', { name: 'Board', pressed: true })).toBeVisible();
	});

	it('shows subtask progress in both project layouts', async () => {
		const project = testProject();
		const changed = testProject({ layout: 'board', version: 2 });
		const task = testTask({
			title: 'Prepare release',
			subtaskCount: 4,
			completedSubtaskCount: 2
		});
		renderProjectTasks({
			project,
			tasks: [task],
			changeLayout: vi.fn(async () => changed)
		});

		await expect.element(page.getByLabelText('2 of 4 subtasks completed')).toBeVisible();
		await page.getByRole('button', { name: 'Board' }).click();
		await expect.element(page.getByLabelText('2 of 4 subtasks completed')).toBeVisible();
	});

	it('hides completed tasks in board layout', async () => {
		const project = testProject();
		const changed = testProject({ layout: 'board', version: 2 });
		const completed = testTask({
			title: 'Finished task',
			status: 'completed',
			completedAt: '2026-07-17T09:00:00Z'
		});
		renderProjectTasks({
			project,
			tasks: [completed],
			changeLayout: vi.fn(async () => changed)
		});

		await expect.element(page.getByRole('region', { name: 'Completed' })).toBeVisible();

		await page.getByRole('button', { name: 'Board' }).click();

		await expect.element(page.getByRole('region', { name: 'Completed' })).not.toBeInTheDocument();
		await expect.element(page.getByText(completed.title, { exact: true })).not.toBeInTheDocument();
	});

	it('creates a task in the selected section', async () => {
		const section = testSection({ id: 'doing', name: 'Doing' });
		const created = testTask({ title: 'Ship the change', sectionId: section.id });
		const create = vi.fn(async () => created);
		renderProjectTasks({ sections: [section], create });

		await expect
			.element(page.getByRole('button', { name: 'project: Work. Open picker' }))
			.not.toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'section: Doing. Open picker' }))
			.not.toBeInTheDocument();
		await page.getByRole('combobox', { name: 'Add task to Doing' }).fill('Ship the change');
		await page.getByRole('button', { name: 'Add task to Doing' }).click();

		expect(create).toHaveBeenCalledWith(
			expect.objectContaining({
				title: 'Ship the change',
				projectId: section.projectId,
				sectionId: section.id
			})
		);
		await expect
			.element(page.getByRole('region', { name: 'Doing' }).getByText('Ship the change'))
			.toBeVisible();
	});

	it('reflects a new server snapshot without remounting the project', async () => {
		const existing = testTask({ id: 'existing', title: 'Existing task' });
		const external = testTask({ id: 'external', title: 'Created elsewhere', position: 2048 });
		const view = renderProjectTasks({ tasks: [existing] });

		await expect.element(view.getByText(existing.title, { exact: true })).toBeVisible();
		await view.rerender({ initialTasks: [existing, external] });

		await expect.element(view.getByText(external.title, { exact: true })).toBeVisible();
	});

	it('opens task editing as an accessible modal with the existing values', async () => {
		const section = testSection({ id: 'doing', name: 'Doing' });
		const task = testTask({
			title: 'Draft proposal',
			description: 'Share with the team',
			sectionId: section.id,
			priority: 3,
			dueDate: '2026-07-20',
			dueTime: '14:30'
		});
		renderProjectTasks({ sections: [section], tasks: [task] });

		await page.getByRole('button', { name: `Open ${task.title}` }).click();

		const dialog = page.getByRole('dialog', { name: `Edit task: ${task.title}` });
		await expect.element(dialog).toHaveAttribute('aria-modal', 'true');
		await expect.element(dialog.getByRole('combobox', { name: 'Title' })).toHaveValue(task.title);
		await expect
			.element(dialog.getByRole('textbox', { name: 'Description' }))
			.toHaveValue(task.description ?? '');
		await expect
			.element(dialog.getByRole('button', { name: 'Priority: High', exact: true }))
			.toBeVisible();
		await expect.element(dialog.getByRole('button', { name: /^Due date:/ })).toBeVisible();
		await expect.element(dialog.getByRole('button', { name: /^Due time:/ })).toBeVisible();
		await expect
			.element(dialog.getByRole('button', { name: /^project: .*\. Open picker$/ }))
			.not.toBeInTheDocument();
		await expect
			.element(dialog.getByRole('button', { name: /^section: .*\. Open picker$/ }))
			.not.toBeInTheDocument();
		await expect
			.element(dialog.getByRole('button', { name: /^priority: .*\. Open picker$/ }))
			.not.toBeInTheDocument();
		await expect
			.element(dialog.getByRole('button', { name: /^due: .*\. Open picker$/ }))
			.not.toBeInTheDocument();
	});

	it('applies title autocomplete in the full editor without showing property pills', async () => {
		const task = testTask({ title: 'Draft proposal' });
		renderProjectTasks({ tasks: [task] });

		await page.getByRole('button', { name: `Open ${task.title}` }).click();
		const dialog = page.getByRole('dialog', { name: `Edit task: ${task.title}` });
		const title = dialog.getByRole('combobox', { name: 'Title' });
		await title.fill(`${task.title} !hi`);
		await userEvent.keyboard('{Enter}');

		await expect.element(title).toHaveValue(task.title);
		await expect
			.element(dialog.getByRole('button', { name: 'Priority: High', exact: true }))
			.toBeVisible();
		await expect
			.element(dialog.getByRole('button', { name: 'priority: High. Open picker' }))
			.not.toBeInTheDocument();
	});

	it('closes task editing with Escape without updating the task', async () => {
		const task = testTask({ title: 'Draft proposal' });
		const update = vi.fn();
		renderProjectTasks({ tasks: [task], update });

		await page.getByRole('button', { name: `Open ${task.title}` }).click();
		await page
			.getByRole('dialog', { name: `Edit task: ${task.title}` })
			.getByLabelText('Title')
			.fill('Changed title');
		await userEvent.keyboard('{Escape}');

		await expect
			.element(page.getByRole('dialog', { name: `Edit task: ${task.title}` }))
			.not.toBeInTheDocument();
		expect(update).not.toHaveBeenCalled();
	});

	it('saves task changes and closes the modal', async () => {
		const task = testTask({ title: 'Draft proposal' });
		const updated = testTask({ ...task, title: 'Final proposal', version: 2 });
		const update = vi.fn(async () => updated);
		renderProjectTasks({ tasks: [task], update });

		await page.getByRole('button', { name: `Open ${task.title}` }).click();
		const dialog = page.getByRole('dialog', { name: `Edit task: ${task.title}` });
		await dialog.getByLabelText('Title').fill(updated.title);
		await dialog.getByRole('button', { name: 'Save changes' }).click();

		expect(update).toHaveBeenCalledWith(
			task.id,
			expect.objectContaining({ version: task.version, title: updated.title })
		);
		await expect.element(dialog).not.toBeInTheDocument();
		await expect.element(page.getByText(updated.title, { exact: true })).toBeVisible();
	});

	it('does not open task editing from complete or delete actions', async () => {
		const completable = testTask({ id: 'complete-me', title: 'Complete me' });
		const deletable = testTask({ id: 'delete-me', title: 'Delete me' });
		const complete = vi.fn(async () =>
			testTask({ ...completable, status: 'completed', completedAt: '2026-07-17T09:00:00Z' })
		);
		const remove = vi.fn(async () => undefined);
		renderProjectTasks({ tasks: [completable, deletable], complete, remove });

		await page.getByRole('button', { name: `Complete ${completable.title}` }).click();
		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();

		await page.getByRole('button', { name: `Delete ${deletable.title}` }).click();
		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		expect(complete).toHaveBeenCalledWith(completable.id, completable.version);
		expect(remove).toHaveBeenCalledWith(deletable.id, deletable.version);
	});

	it('moves a task between sections with drag-and-drop in list layout', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing', position: 1024 });
		const later = testSection({ id: 'later', name: 'Later', position: 2048 });
		const draft = testTask({ id: 'draft', title: 'Draft proposal', sectionId: doing.id });
		const review = testTask({ id: 'review', title: 'Review proposal', sectionId: later.id });
		const reordered = [
			testTask({ ...draft, sectionId: later.id, position: 1024, version: 2 }),
			testTask({ ...review, position: 2048 })
		];
		const reorder = vi.fn(async () => reordered);
		renderProjectTasks({ sections: [doing, later], tasks: [draft, review], reorder });

		const source = page
			.getByRole('region', { name: 'Doing' })
			.getByRole('listitem')
			.filter({ hasText: 'Draft proposal' });
		const target = page
			.getByRole('region', { name: 'Later' })
			.getByRole('listitem')
			.filter({ hasText: 'Review proposal' });
		const dataTransfer = startDrag(source);
		dragOver(target, dataTransfer);

		await expect.element(source).toHaveAttribute('data-dragging', 'true');
		expect(hasVisibleInsertionMarker(target, 'task')).toBe(true);

		dropOn(target, dataTransfer);

		expect(reorder).toHaveBeenCalledWith(draft.id, draft.version, later.id, review.id);
		await expect
			.element(page.getByRole('region', { name: 'Later' }).getByText('Draft proposal'))
			.toBeVisible();
	});

	it('moves a task to an empty section in board layout', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing', position: 1024 });
		const later = testSection({ id: 'later', name: 'Later', position: 2048 });
		const draft = testTask({ id: 'draft', title: 'Draft proposal', sectionId: doing.id });
		const moved = testTask({ ...draft, sectionId: later.id, version: 2 });
		const reorder = vi.fn(async () => [moved]);
		renderProjectTasks({
			project: testProject({ layout: 'board' }),
			sections: [doing, later],
			tasks: [draft],
			reorder
		});

		const source = page
			.getByRole('region', { name: 'Doing' })
			.getByRole('listitem')
			.filter({ hasText: 'Draft proposal' });
		const targetSection = page.getByRole('region', { name: 'Later' });
		const target = targetSection.getByLabelText('Drop task in Later');
		const dataTransfer = startDrag(source);
		dropOn(target, dataTransfer);

		expect(reorder).toHaveBeenCalledWith(draft.id, draft.version, later.id, null);
		await expect.element(targetSection.getByText('Draft proposal')).toBeVisible();
	});

	it('moves an unsectioned task after the last task in a board section', async () => {
		const niceToDo = testSection({ id: 'nice-to-do', name: 'Nice to do' });
		const sourceTask = testTask({ id: 'do-things', title: 'Сделать дела' });
		const existingTask = testTask({
			id: 'existing',
			title: 'лл',
			sectionId: niceToDo.id,
			position: 1024
		});
		const movedTask = testTask({
			...sourceTask,
			sectionId: niceToDo.id,
			position: 2048,
			version: 2
		});
		const reorder = vi.fn(async () => [existingTask, movedTask]);
		renderProjectTasks({
			project: testProject({ layout: 'board' }),
			sections: [niceToDo],
			tasks: [sourceTask, existingTask],
			reorder
		});

		const source = page
			.getByRole('region', { name: 'No section' })
			.getByRole('listitem')
			.filter({ hasText: 'Сделать дела' });
		const destination = page.getByRole('region', { name: 'Nice to do' });
		const target = destination.getByLabelText('Drop task in Nice to do');
		const dataTransfer = startDrag(source);
		dragOver(target, dataTransfer);

		await expect.element(source).toHaveAttribute('data-dragging', 'true');
		expect(target.element().getBoundingClientRect().height).toBeGreaterThanOrEqual(32);
		expect(hasVisibleInsertionMarker(target, 'task')).toBe(true);

		dropOn(target, dataTransfer);

		expect(reorder).toHaveBeenCalledWith(sourceTask.id, sourceTask.version, niceToDo.id, null);
		await expect.element(destination.getByText('Сделать дела')).toBeVisible();
	});

	it('shows the same insertion marker when dragging a task into an empty section', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing', position: 1024 });
		const later = testSection({ id: 'later', name: 'Later', position: 2048 });
		const draft = testTask({ id: 'draft', title: 'Draft proposal', sectionId: doing.id });
		renderProjectTasks({
			project: testProject({ layout: 'board' }),
			sections: [doing, later],
			tasks: [draft]
		});

		const source = page
			.getByRole('region', { name: 'Doing' })
			.getByRole('listitem')
			.filter({ hasText: 'Draft proposal' });
		const target = page.getByRole('region', { name: 'Later' }).getByLabelText('Drop task in Later');
		const dataTransfer = startDrag(source);
		dragOver(target, dataTransfer);

		await expect.element(source).toHaveAttribute('data-dragging', 'true');
		expect(hasVisibleInsertionMarker(target, 'task')).toBe(true);
		expect(document.body.textContent).not.toContain('Drop task here');
	});

	it('keeps the empty-section insertion marker clear of the quick-add form', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing', position: 1024 });
		const later = testSection({ id: 'later', name: 'Later', position: 2048 });
		const draft = testTask({ id: 'draft', title: 'Draft proposal', sectionId: doing.id });
		renderProjectTasks({
			project: testProject({ layout: 'board' }),
			sections: [doing, later],
			tasks: [draft]
		});

		const source = page
			.getByRole('region', { name: 'Doing' })
			.getByRole('listitem')
			.filter({ hasText: 'Draft proposal' });
		const destination = page.getByRole('region', { name: 'Later' });
		const target = destination.getByLabelText('Drop task in Later');
		const dataTransfer = startDrag(source);
		dragOver(target, dataTransfer);

		await expect.element(source).toHaveAttribute('data-dragging', 'true');
		const marker = target.element().querySelector('[data-insertion-marker="task"]');
		const quickAdd = destination
			.getByRole('combobox', { name: 'Add task to Later' })
			.element()
			.closest('form');
		expect(marker).toBeInstanceOf(HTMLElement);
		expect(quickAdd).toBeInstanceOf(HTMLElement);
		expect(
			rectanglesOverlap(
				insertionMarkerVisualBounds(marker as HTMLElement),
				(quickAdd as HTMLElement).getBoundingClientRect()
			)
		).toBe(false);
	});

	it('keeps the task insertion marker inside the leftmost board column', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing' });
		const unsectioned = testTask({ id: 'triage', title: 'Triage request' });
		const draft = testTask({ id: 'draft', title: 'Draft proposal', sectionId: doing.id });
		renderProjectTasks({
			project: testProject({ layout: 'board' }),
			sections: [doing],
			tasks: [unsectioned, draft]
		});

		const source = page
			.getByRole('region', { name: 'Doing' })
			.getByRole('listitem')
			.filter({ hasText: 'Draft proposal' });
		const destination = page.getByRole('region', { name: 'No section' });
		const target = destination.getByRole('listitem').filter({ hasText: 'Triage request' });
		const dataTransfer = startDrag(source);
		dragOver(target, dataTransfer);

		await expect.element(source).toHaveAttribute('data-dragging', 'true');
		const marker = target.element().querySelector('[data-insertion-marker="task"]');
		expect(marker).toBeInstanceOf(HTMLElement);
		expect(insertionMarkerVisualBounds(marker as HTMLElement).left).toBeGreaterThanOrEqual(
			destination.element().getBoundingClientRect().left
		);
	});

	it('shows section drag feedback and moves the section to the marked position', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing', position: 1024 });
		const later = testSection({ id: 'later', name: 'Later', position: 2048 });
		const reordered = [
			testSection({ ...later, position: 1024, version: 2 }),
			testSection({ ...doing, position: 2048 })
		];
		const reorderSection = vi.fn(async () => reordered);
		renderProjectTasks({ sections: [doing, later], reorderSection });

		const source = page.getByRole('region', { name: 'Later' });
		const sourceHandle = page.getByRole('group', { name: 'Move section Later' });
		const target = page.getByRole('region', { name: 'Doing' });
		const dataTransfer = startDrag(sourceHandle);
		dragOver(target, dataTransfer);

		await expect.element(source).toHaveAttribute('data-dragging', 'true');
		expect(hasVisibleInsertionMarker(target, 'section')).toBe(true);

		dropOn(target, dataTransfer);

		expect(reorderSection).toHaveBeenCalledWith(later.id, later.version, doing.id);
	});

	it('shows the same section insertion marker when moving a section to the end', async () => {
		const doing = testSection({ id: 'doing', name: 'Doing', position: 1024 });
		const later = testSection({ id: 'later', name: 'Later', position: 2048 });
		const reordered = [
			testSection({ ...later, position: 1024 }),
			testSection({ ...doing, position: 2048, version: 2 })
		];
		const reorderSection = vi.fn(async () => reordered);
		renderProjectTasks({ sections: [doing, later], reorderSection });

		const source = page.getByRole('region', { name: 'Doing' });
		const sourceHandle = page.getByRole('group', { name: 'Move section Doing' });
		const target = page.getByRole('group', { name: 'Add or move section' });
		const dataTransfer = startDrag(sourceHandle);
		dragOver(target, dataTransfer);

		await expect.element(source).toHaveAttribute('data-dragging', 'true');
		expect(hasVisibleInsertionMarker(target, 'section')).toBe(true);

		dropOn(target, dataTransfer);

		expect(reorderSection).toHaveBeenCalledWith(doing.id, doing.version, null);
	});
});

interface RenderOptions {
	project?: Project;
	sections?: ProjectSection[];
	tasks?: TaskSummary[];
	create?: (draft: TaskCreateDraft) => Promise<Task>;
	complete?: (taskId: string, version: number) => Promise<Task>;
	update?: (taskId: string, changes: TaskUpdate) => Promise<Task>;
	remove?: (taskId: string, version: number) => Promise<void>;
	reorder?: (
		taskId: string,
		version: number,
		sectionId: string | null,
		beforeTaskId: string | null
	) => Promise<TaskSummary[]>;
	changeLayout?: (version: number, layout: 'list' | 'board') => Promise<Project>;
	reorderSection?: (
		sectionId: string,
		version: number,
		beforeSectionId: string | null
	) => Promise<ProjectSection[]>;
}

function renderProjectTasks(options: RenderOptions = {}) {
	const project = options.project ?? testProject();
	return render(ProjectTasks, {
		project,
		projects: [project],
		initialSections: options.sections ?? [],
		initialTasks: options.tasks ?? [],
		create: options.create ?? vi.fn(),
		complete: options.complete ?? vi.fn(),
		reopen: vi.fn(),
		update: options.update ?? vi.fn(),
		remove: options.remove ?? vi.fn(),
		reorder: options.reorder ?? vi.fn(),
		changeLayout: options.changeLayout ?? vi.fn(),
		createSection: vi.fn(),
		updateSection: vi.fn(),
		deleteSection: vi.fn(),
		reorderSection: options.reorderSection ?? vi.fn()
	});
}

function testProject(overrides: Partial<Project> = {}): Project {
	return {
		id: 'project-id',
		name: 'Work',
		layout: 'list',
		colorTheme: 'sage',
		agentModel: 'gpt-default',
		agentThinkingEffort: 'medium',
		position: 1024,
		version: 1,
		archivedAt: null,
		createdAt: '2026-07-17T08:00:00Z',
		updatedAt: '2026-07-17T08:00:00Z',
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
		createdAt: '2026-07-17T08:00:00Z',
		updatedAt: '2026-07-17T08:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testTask(overrides: Partial<TaskSummary> = {}): TaskSummary {
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
		createdAt: '2026-07-17T08:00:00Z',
		updatedAt: '2026-07-17T08:00:00Z',
		lastModifiedBy: 'user-id',
		subtaskCount: 0,
		completedSubtaskCount: 0,
		...overrides
	};
}

function startDrag(source: Locator): DataTransfer {
	const dataTransfer = new DataTransfer();
	source
		.element()
		.dispatchEvent(new DragEvent('dragstart', { bubbles: true, cancelable: true, dataTransfer }));
	return dataTransfer;
}

function dragOver(target: Locator, dataTransfer: DataTransfer) {
	target
		.element()
		.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer }));
}

function dropOn(target: Locator, dataTransfer: DataTransfer) {
	dragOver(target, dataTransfer);
	target
		.element()
		.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer }));
}

function hasVisibleInsertionMarker(target: Locator, type: 'task' | 'section'): boolean {
	const marker = target.element().querySelector(`[data-insertion-marker="${type}"]`);
	if (!(marker instanceof HTMLElement)) return false;
	const bounds = marker.getBoundingClientRect();
	const style = getComputedStyle(marker);
	return bounds.width > 0 && bounds.height > 0 && style.visibility !== 'hidden';
}

interface ElementBounds {
	top: number;
	right: number;
	bottom: number;
	left: number;
}

function insertionMarkerVisualBounds(marker: HTMLElement): ElementBounds {
	const markerBounds = marker.getBoundingClientRect();
	const circleStyle = getComputedStyle(marker, '::before');
	const circleLeft = Number.parseFloat(circleStyle.left);
	const circleTop = Number.parseFloat(circleStyle.top);
	const circleWidth = Number.parseFloat(circleStyle.width);
	const circleHeight = Number.parseFloat(circleStyle.height);
	if (![circleLeft, circleTop, circleWidth, circleHeight].every(Number.isFinite)) {
		return markerBounds;
	}

	const circleBounds = {
		left: markerBounds.left + circleLeft,
		top: markerBounds.top + circleTop,
		right: markerBounds.left + circleLeft + circleWidth,
		bottom: markerBounds.top + circleTop + circleHeight
	};
	return {
		left: Math.min(markerBounds.left, circleBounds.left),
		top: Math.min(markerBounds.top, circleBounds.top),
		right: Math.max(markerBounds.right, circleBounds.right),
		bottom: Math.max(markerBounds.bottom, circleBounds.bottom)
	};
}

function rectanglesOverlap(left: ElementBounds, right: ElementBounds): boolean {
	return (
		left.left < right.right &&
		left.right > right.left &&
		left.top < right.bottom &&
		left.bottom > right.top
	);
}
