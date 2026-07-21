<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import SubtaskProgress from '$lib/tasks/SubtaskProgress.svelte';
	import type { Task, TaskCreateDraft, TaskSummary, TaskUpdate } from '$lib/tasks/client';
	import TaskEditorModal from './TaskEditorModal.svelte';
	import TaskQuickAdd from './TaskQuickAdd.svelte';

	interface Props {
		initialTasks: TaskSummary[];
		create?: (draft: TaskCreateDraft) => Promise<Task>;
		complete: (taskId: string, version: number) => Promise<Task>;
		reopen: (taskId: string, version: number) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string, version: number) => Promise<void>;
		eyebrow: string;
		heading: string;
		countNoun?: string;
		emptyTitle: string;
		emptyMessage: string;
		listLabel: string;
		projects?: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		currentProjectId?: string | null;
		currentSectionId?: string | null;
	}

	interface TaskGroup {
		key: string;
		label: string;
		tone: 'overdue' | 'today' | 'upcoming' | 'undated' | 'completed';
		sortTime: number;
		tasks: TaskSummary[];
	}

	let {
		initialTasks,
		create,
		complete,
		reopen,
		update,
		remove,
		eyebrow,
		heading,
		countNoun = 'active',
		emptyTitle,
		emptyMessage,
		listLabel,
		projects = [],
		sections = [],
		loadSections,
		currentProjectId,
		currentSectionId
	}: Props = $props();
	let tasks = $derived([...initialTasks]);
	let busyTaskIds = $state<string[]>([]);
	let errorMessage = $state('');
	let editingTaskId = $state<string | null>(null);
	let activeTasks = $derived(tasks.filter((item) => item.status === 'active'));
	let completedTasks = $derived(tasks.filter((item) => item.status === 'completed'));
	let taskGroups = $derived(buildTaskGroups(activeTasks, completedTasks));
	let editingTask = $derived(tasks.find((item) => item.id === editingTaskId));

	async function changeStatus(item: TaskSummary) {
		const previousItem = item;
		const nextStatus = item.status === 'active' ? 'completed' : 'active';
		const optimisticItem: TaskSummary = {
			...item,
			status: nextStatus,
			completedAt: nextStatus === 'completed' ? new Date().toISOString() : null
		};

		busyTaskIds = [...busyTaskIds, item.id];
		errorMessage = '';
		tasks = tasks.map((candidate) => (candidate.id === item.id ? optimisticItem : candidate));
		try {
			const updated =
				item.status === 'active'
					? await complete(item.id, item.version)
					: await reopen(item.id, item.version);
			tasks = tasks.map((candidate) =>
				candidate.id === updated.id ? { ...candidate, ...updated } : candidate
			);
		} catch {
			tasks = tasks.map((candidate) => (candidate.id === item.id ? previousItem : candidate));
			errorMessage = 'The task could not be updated. Please try again.';
		} finally {
			busyTaskIds = busyTaskIds.filter((taskId) => taskId !== item.id);
		}
	}

	async function deleteItem(item: TaskSummary) {
		const previousIndex = tasks.findIndex((candidate) => candidate.id === item.id);

		busyTaskIds = [...busyTaskIds, item.id];
		errorMessage = '';
		tasks = tasks.filter((candidate) => candidate.id !== item.id);
		try {
			await remove(item.id, item.version);
		} catch {
			if (!tasks.some((candidate) => candidate.id === item.id)) {
				const restoredTasks = [...tasks];
				restoredTasks.splice(previousIndex, 0, item);
				tasks = restoredTasks;
			}
			errorMessage = 'The task could not be deleted. Please try again.';
		} finally {
			busyTaskIds = busyTaskIds.filter((taskId) => taskId !== item.id);
		}
	}

	async function saveItem(item: TaskSummary, changes: TaskUpdate) {
		const updated = await update(item.id, changes);
		const remainsInView =
			(currentProjectId === undefined || updated.projectId === currentProjectId) &&
			(currentSectionId === undefined || updated.sectionId === currentSectionId);
		tasks = remainsInView
			? tasks.map((candidate) =>
					candidate.id === updated.id ? { ...candidate, ...updated } : candidate
				)
			: tasks.filter((candidate) => candidate.id !== updated.id);
		editingTaskId = null;
	}

	function openTaskEditor(item: TaskSummary) {
		editingTaskId = item.id;
	}

	function dueTime(item: TaskSummary): string {
		if (!item.dueTime) return '';

		const [hours, minutes] = item.dueTime.split(':').map(Number);
		return new Intl.DateTimeFormat(undefined, {
			hour: '2-digit',
			minute: '2-digit'
		}).format(new Date(2000, 0, 1, hours, minutes));
	}

	function isOverdue(item: TaskSummary): boolean {
		if (item.status !== 'active' || item.dueDate === null) return false;

		const now = new Date();
		const dueDate = parseDueDate(item.dueDate);
		const today = startOfDay(now);
		if (dueDate.getTime() < today.getTime()) return true;
		if (dueDate.getTime() > today.getTime() || item.dueTime === null) return false;

		const [hours, minutes] = item.dueTime.split(':').map(Number);
		return new Date(now.getFullYear(), now.getMonth(), now.getDate(), hours, minutes) < now;
	}

	function priorityLabel(priority: number): string {
		return ['', 'Low', 'Medium', 'High', 'Urgent'][priority] ?? '';
	}

	function projectName(projectId: string | null): string {
		if (!projectId) return '';
		return projects.find((project) => project.id === projectId)?.name ?? '';
	}

	function summaryFromTask(task: Task): TaskSummary {
		return { ...task, subtaskCount: 0, completedSubtaskCount: 0 };
	}

	function buildTaskGroups(active: TaskSummary[], completed: TaskSummary[]): TaskGroup[] {
		const now = new Date();
		const today = startOfDay(now);
		const tomorrow = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 1);
		const groups = new Map<string, TaskGroup>();

		for (const item of active) {
			if (!item.dueDate) {
				addToGroup(
					groups,
					{
						key: 'no-date',
						label: 'No date',
						tone: 'undated',
						sortTime: Number.POSITIVE_INFINITY,
						tasks: []
					},
					item
				);
				continue;
			}

			const dueDay = parseDueDate(item.dueDate);
			const key = localDateKey(dueDay);
			let label = formatCalendarDate(dueDay, now);
			let tone: TaskGroup['tone'] = 'upcoming';
			if (dueDay.getTime() < today.getTime()) {
				label = `Overdue · ${label}`;
				tone = 'overdue';
			} else if (dueDay.getTime() === today.getTime()) {
				label = 'Today';
				tone = 'today';
			} else if (dueDay.getTime() === tomorrow.getTime()) {
				label = 'Tomorrow';
			}

			addToGroup(
				groups,
				{
					key,
					label,
					tone,
					sortTime: dueDay.getTime(),
					tasks: []
				},
				item
			);
		}

		const result = [...groups.values()].sort((left, right) => left.sortTime - right.sortTime);
		for (const group of result) {
			group.tasks.sort(compareTasks);
		}
		if (completed.length > 0) {
			result.push({
				key: 'completed',
				label: 'Completed',
				tone: 'completed',
				sortTime: Number.POSITIVE_INFINITY,
				tasks: [...completed].sort((left, right) =>
					(right.completedAt ?? '').localeCompare(left.completedAt ?? '')
				)
			});
		}

		return result;
	}

	function addToGroup(groups: Map<string, TaskGroup>, group: TaskGroup, item: TaskSummary) {
		const existing = groups.get(group.key);
		if (existing) {
			existing.tasks.push(item);
			return;
		}

		group.tasks.push(item);
		groups.set(group.key, group);
	}

	function compareTasks(left: TaskSummary, right: TaskSummary): number {
		const timeDifference = timeOfDayMinutes(left.dueTime) - timeOfDayMinutes(right.dueTime);
		return timeDifference || right.priority - left.priority || left.position - right.position;
	}

	function timeOfDayMinutes(value: string | null): number {
		if (value === null) return -1;
		const [hours, minutes] = value.split(':').map(Number);
		return hours * 60 + minutes;
	}

	function startOfDay(value: Date): Date {
		return new Date(value.getFullYear(), value.getMonth(), value.getDate());
	}

	function parseDueDate(value: string): Date {
		const [year, month, day] = value.split('-').map(Number);
		return new Date(year, month - 1, day);
	}

	function localDateKey(value: Date): string {
		return `${value.getFullYear()}-${value.getMonth() + 1}-${value.getDate()}`;
	}

	function formatCalendarDate(value: Date, now: Date): string {
		return new Intl.DateTimeFormat(undefined, {
			weekday: 'long',
			month: 'long',
			day: 'numeric',
			...(value.getFullYear() === now.getFullYear() ? {} : { year: 'numeric' })
		}).format(value);
	}
</script>

<section class="task-view" aria-labelledby="task-view-heading">
	<header class="view-header">
		<div>
			<p class="eyebrow">{eyebrow}</p>
			<h1 id="task-view-heading">{heading}</h1>
		</div>
		<span class="count">{activeTasks.length} {countNoun}</span>
	</header>

	{#if create}
		<TaskQuickAdd
			{create}
			oncreated={(created) => (tasks = [...tasks, summaryFromTask(created)])}
			{projects}
			{sections}
			{loadSections}
			initialProjectId={currentProjectId ?? projects[0]?.id ?? ''}
			initialSectionId={currentSectionId ?? null}
		/>
	{/if}

	{#if errorMessage}
		<p class="error" role="alert">{errorMessage}</p>
	{/if}

	<div class="task-space">
		{#if taskGroups.length > 0}
			<div class="task-groups" aria-label={listLabel}>
				{#each taskGroups as group (group.key)}
					<section class={`task-group ${group.tone}`} aria-labelledby={`group-${group.key}`}>
						<header class="group-header">
							<h2 id={`group-${group.key}`}>{group.label}</h2>
							<span>{group.tasks.length}</span>
						</header>
						<ul aria-label={`${group.label} tasks`}>
							{#each group.tasks as item (item.id)}
								<li class:completed-task={item.status === 'completed'}>
									<button
										class:checked={item.status === 'completed'}
										class="task-toggle"
										type="button"
										aria-label={`${item.status === 'active' ? 'Complete' : 'Reopen'} ${item.title}`}
										disabled={busyTaskIds.includes(item.id)}
										onclick={(event) => {
											event.stopPropagation();
											void changeStatus(item);
										}}
									>
										{item.status === 'completed' ? '✓' : ''}
									</button>
									<button
										class="task-copy"
										type="button"
										aria-label={`Open ${item.title}`}
										onclick={() => openTaskEditor(item)}
									>
										<span class="task-title">{item.title}</span>
										{#if item.description}
											<span class="task-description">{item.description}</span>
										{/if}
										{#if item.dueTime || item.priority > 0 || projectName(item.projectId) || item.subtaskCount > 0}
											<span class="task-metadata">
												<SubtaskProgress
													completed={item.completedSubtaskCount}
													total={item.subtaskCount}
												/>
												{#if item.dueTime}
													<span class:overdue={isOverdue(item)} class="due">{dueTime(item)}</span>
												{/if}
												{#if item.priority > 0}
													<span class={`priority priority-${item.priority}`}>
														{priorityLabel(item.priority)}
													</span>
												{/if}
												{#if projectName(item.projectId)}
													<span class="task-project">{projectName(item.projectId)}</span>
												{/if}
											</span>
										{/if}
									</button>
									<div class="task-actions">
										<button
											class="edit-task"
											type="button"
											aria-label={`Edit ${item.title}`}
											onclick={(event) => {
												event.stopPropagation();
												openTaskEditor(item);
											}}>Edit</button
										>
										<button
											class="delete-task"
											type="button"
											aria-label={`Delete ${item.title}`}
											disabled={busyTaskIds.includes(item.id)}
											onclick={(event) => {
												event.stopPropagation();
												void deleteItem(item);
											}}>Delete</button
										>
									</div>
								</li>
							{/each}
						</ul>
					</section>
				{/each}
			</div>
		{:else}
			<div class="empty-state">
				<p>{emptyTitle}</p>
				<span>{emptyMessage}</span>
			</div>
		{/if}
	</div>
</section>

{#if editingTask}
	<TaskEditorModal
		task={editingTask}
		{projects}
		{sections}
		{loadSections}
		save={(changes) => saveItem(editingTask, changes)}
		close={() => (editingTaskId = null)}
	/>
{/if}

<style>
	.task-view {
		width: min(52rem, 100%);
		margin: 0 auto;
	}

	.view-header {
		display: flex;
		align-items: end;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.eyebrow {
		margin: 0 0 0.3rem;
		color: #8a8984;
		font-size: 0.68rem;
		font-weight: 720;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: clamp(1.75rem, 5vw, 2.25rem);
		line-height: 1.1;
		letter-spacing: -0.04em;
	}

	.count {
		color: #8a8984;
		font-size: 0.75rem;
		font-weight: 650;
	}

	.task-space {
		min-height: 12rem;
	}

	.task-groups {
		display: grid;
		gap: 2.1rem;
		margin-top: 2rem;
	}

	.task-group ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.group-header {
		display: flex;
		align-items: baseline;
		gap: 0.45rem;
		padding-bottom: 0.55rem;
		border-bottom: 1px solid #d9d9d5;
	}

	.group-header h2 {
		margin: 0;
		font-size: 0.87rem;
		font-weight: 760;
		letter-spacing: -0.01em;
	}

	.group-header span {
		color: #999792;
		font-size: 0.7rem;
	}

	.task-group.overdue .group-header h2 {
		color: #c13d33;
	}

	.task-group.today .group-header h2 {
		color: var(--theme-accent, #32885e);
	}

	.task-group.completed .group-header h2,
	.task-group.undated .group-header h2 {
		color: #686762;
	}

	.task-group li {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		min-height: 3.25rem;
		padding: 0.72rem 0;
		border-bottom: 1px solid #ecece9;
	}

	.task-toggle {
		display: grid;
		flex: 0 0 auto;
		width: 1.25rem;
		height: 1.25rem;
		margin-top: 0.08rem;
		place-items: center;
		padding: 0;
		border: 1.5px solid #8f8e89;
		border-radius: 50%;
		color: #fff;
		background: transparent;
		font-size: 0.66rem;
		cursor: pointer;
	}

	.task-toggle:hover:not(:disabled) {
		border-color: #555550;
		background: #f1f1ee;
	}

	.task-toggle.checked {
		border-color: #8f8e89;
		background: #8f8e89;
	}

	.task-toggle:disabled {
		cursor: wait;
		opacity: 0.55;
	}

	.task-copy {
		display: grid;
		flex: 1;
		gap: 0.22rem;
		min-width: 0;
		padding: 0;
		border: 0;
		color: inherit;
		background: transparent;
		text-align: left;
		cursor: pointer;
	}

	.task-title {
		color: #31312e;
		font-size: 0.9rem;
		line-height: 1.35;
		overflow-wrap: anywhere;
	}

	.task-description {
		display: -webkit-box;
		overflow: hidden;
		color: #777671;
		font-size: 0.75rem;
		line-height: 1.35;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
	}

	.task-metadata {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
		color: #777671;
		font-size: 0.7rem;
	}

	.due {
		color: var(--theme-accent, #32885e);
	}

	.due.overdue {
		color: #c13d33;
		font-weight: 700;
	}

	.priority {
		font-weight: 700;
	}

	.priority-3 {
		color: #c67424;
	}

	.priority-4 {
		color: #c13d33;
	}

	.task-project {
		color: var(--theme-accent, #52705a);
	}

	.task-actions {
		display: flex;
		flex: 0 0 auto;
		gap: 0.1rem;
		opacity: 0;
		transition: opacity 100ms ease;
	}

	.task-group li:hover .task-actions,
	.task-actions:focus-within {
		opacity: 1;
	}

	.edit-task,
	.delete-task {
		padding: 0.3rem 0.4rem;
		border: 0;
		border-radius: 0.3rem;
		color: #6b6a65;
		background: transparent;
		font-size: 0.68rem;
		font-weight: 650;
		cursor: pointer;
	}

	.edit-task:hover {
		color: #292927;
		background: #efefec;
	}

	.delete-task:hover:not(:disabled) {
		color: #b83f34;
		background: #feeae7;
	}

	.delete-task:disabled {
		cursor: wait;
		opacity: 0.35;
	}

	.empty-state {
		padding: 5rem 1rem;
		text-align: center;
	}

	.empty-state p {
		margin: 0 0 0.4rem;
		font-weight: 750;
	}

	.empty-state span {
		color: #777671;
		font-size: 0.86rem;
	}

	.completed-task .task-title,
	.completed-task .task-description {
		text-decoration: line-through;
		opacity: 0.62;
	}

	.error {
		margin: 0.85rem 0 0;
		color: #b83f34;
		font-size: 0.8rem;
	}

	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}

	@media (max-width: 40rem) {
		.view-header {
			align-items: start;
		}

		.count {
			padding-top: 0.45rem;
		}

		.task-groups {
			gap: 1.8rem;
		}

		.task-actions {
			opacity: 1;
		}

		.task-group li {
			gap: 0.6rem;
		}

		.edit-task,
		.delete-task {
			font-size: 0;
		}

		.edit-task::after,
		.delete-task::after {
			font-size: 0.8rem;
		}

		.edit-task::after {
			content: 'Edit';
		}

		.delete-task::after {
			content: '×';
			font-size: 1rem;
		}
	}
</style>
