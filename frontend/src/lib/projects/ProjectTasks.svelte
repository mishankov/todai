<script lang="ts">
	import TaskEditorModal from '$lib/components/TaskEditorModal.svelte';
	import TaskQuickAdd from '$lib/components/TaskQuickAdd.svelte';
	import SubtaskProgress from '$lib/tasks/SubtaskProgress.svelte';
	import type { Task, TaskCreateDraft, TaskSummary, TaskUpdate } from '$lib/tasks/client';
	import { untrack } from 'svelte';
	import type { Project, ProjectLayout, ProjectSection } from './client';

	interface Props {
		project: Project;
		projects: Project[];
		initialSections: ProjectSection[];
		initialTasks: TaskSummary[];
		create: (draft: TaskCreateDraft) => Promise<Task>;
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		complete: (taskId: string, version: number) => Promise<Task>;
		reopen: (taskId: string, version: number) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string, version: number) => Promise<void>;
		reorder: (
			taskId: string,
			version: number,
			sectionId: string | null,
			beforeTaskId: string | null
		) => Promise<TaskSummary[]>;
		changeLayout: (version: number, layout: ProjectLayout) => Promise<Project>;
		createSection: (name: string) => Promise<ProjectSection>;
		updateSection: (sectionId: string, version: number, name: string) => Promise<ProjectSection>;
		deleteSection: (sectionId: string, version: number) => Promise<void>;
		reorderSection: (
			sectionId: string,
			version: number,
			beforeSectionId: string | null
		) => Promise<ProjectSection[]>;
	}

	interface SectionGroup {
		key: string;
		name: string;
		section: ProjectSection | null;
		tasks: TaskSummary[];
		completed: boolean;
		showHeader: boolean;
	}

	interface TaskDropTarget {
		sectionKey: string;
		beforeTaskId: string | null;
	}

	interface SectionDropTarget {
		beforeSectionId: string | null;
	}

	let {
		project,
		projects,
		initialSections,
		initialTasks,
		create,
		loadSections,
		complete,
		reopen,
		update,
		remove,
		reorder,
		changeLayout,
		createSection,
		updateSection,
		deleteSection,
		reorderSection
	}: Props = $props();

	let currentProject = $state(untrack(() => project));
	let sections = $state(untrack(() => [...initialSections]));
	let tasks = $state(untrack(() => [...initialTasks]));
	let newSectionName = $state('');
	let creatingSection = $state(false);
	let editingSectionId = $state<string | null>(null);
	let editingSectionName = $state('');
	let editingTaskId = $state<string | null>(null);
	let busyTaskIds = $state<string[]>([]);
	let busySectionIds = $state<string[]>([]);
	let changingLayout = $state(false);
	let draggedTaskId = $state<string | null>(null);
	let taskDropTarget = $state<TaskDropTarget | null>(null);
	let draggedSectionId = $state<string | null>(null);
	let sectionDropTarget = $state<SectionDropTarget | null>(null);
	let errorMessage = $state('');
	let activeTasks = $derived(tasks.filter((item) => item.status === 'active'));
	let sectionGroups = $derived(
		buildSectionGroups(sections, tasks).filter(
			(group) => currentProject.layout === 'list' || !group.completed
		)
	);
	let editingTask = $derived(tasks.find((item) => item.id === editingTaskId));

	$effect(() => {
		const nextProject = project;
		const nextSections = initialSections;
		const nextTasks = initialTasks;
		untrack(() => {
			currentProject = nextProject;
			sections = [...nextSections];
			tasks = [...nextTasks];
			if (editingTaskId !== null && !nextTasks.some((item) => item.id === editingTaskId)) {
				editingTaskId = null;
			}
			if (editingSectionId !== null && !nextSections.some((item) => item.id === editingSectionId)) {
				editingSectionId = null;
			}
		});
	});

	async function setLayout(layout: ProjectLayout) {
		if (layout === currentProject.layout || changingLayout) return;
		const previous = currentProject;
		currentProject = { ...currentProject, layout };
		changingLayout = true;
		errorMessage = '';
		try {
			currentProject = await changeLayout(previous.version, layout);
		} catch {
			currentProject = previous;
			errorMessage = 'The project layout could not be changed. Please try again.';
		} finally {
			changingLayout = false;
		}
	}

	async function addSection() {
		const name = newSectionName.trim();
		if (!name) return;
		creatingSection = true;
		errorMessage = '';
		try {
			const created = await createSection(name);
			sections = [...sections.filter((item) => item.id !== created.id), created];
			newSectionName = '';
		} catch {
			errorMessage = 'The section could not be created. Please try again.';
		} finally {
			creatingSection = false;
		}
	}

	function beginSectionRename(section: ProjectSection) {
		editingSectionId = section.id;
		editingSectionName = section.name;
	}

	async function renameSection(section: ProjectSection) {
		const name = editingSectionName.trim();
		if (!name) return;
		busySectionIds = [...busySectionIds, section.id];
		errorMessage = '';
		try {
			const updated = await updateSection(section.id, section.version, name);
			sections = sections.map((candidate) => (candidate.id === updated.id ? updated : candidate));
			editingSectionId = null;
		} catch {
			errorMessage = 'The section could not be renamed. Reload and try again.';
		} finally {
			busySectionIds = busySectionIds.filter((id) => id !== section.id);
		}
	}

	async function removeSection(section: ProjectSection) {
		const previousSections = sections;
		const previousTasks = tasks;
		busySectionIds = [...busySectionIds, section.id];
		sections = sections.filter((candidate) => candidate.id !== section.id);
		tasks = tasks.map((item) =>
			item.sectionId === section.id ? { ...item, sectionId: null, version: item.version + 1 } : item
		);
		errorMessage = '';
		try {
			await deleteSection(section.id, section.version);
		} catch {
			sections = previousSections;
			tasks = previousTasks;
			errorMessage = 'The section could not be deleted. Reload and try again.';
		} finally {
			busySectionIds = busySectionIds.filter((id) => id !== section.id);
		}
	}

	async function changeStatus(item: TaskSummary) {
		const previous = item;
		const status = item.status === 'active' ? 'completed' : 'active';
		const optimistic = {
			...item,
			status,
			completedAt: status === 'completed' ? new Date().toISOString() : null
		} satisfies TaskSummary;
		busyTaskIds = [...busyTaskIds, item.id];
		tasks = tasks.map((candidate) => (candidate.id === item.id ? optimistic : candidate));
		errorMessage = '';
		try {
			const updated =
				item.status === 'active'
					? await complete(item.id, item.version)
					: await reopen(item.id, item.version);
			tasks = tasks.map((candidate) =>
				candidate.id === updated.id ? { ...candidate, ...updated } : candidate
			);
		} catch {
			tasks = tasks.map((candidate) => (candidate.id === item.id ? previous : candidate));
			errorMessage = 'The task could not be updated. Please try again.';
		} finally {
			busyTaskIds = busyTaskIds.filter((id) => id !== item.id);
		}
	}

	async function deleteTask(item: TaskSummary) {
		const previous = tasks;
		busyTaskIds = [...busyTaskIds, item.id];
		tasks = tasks.filter((candidate) => candidate.id !== item.id);
		errorMessage = '';
		try {
			await remove(item.id, item.version);
		} catch {
			tasks = previous;
			errorMessage = 'The task could not be deleted. Please try again.';
		} finally {
			busyTaskIds = busyTaskIds.filter((id) => id !== item.id);
		}
	}

	async function saveTask(item: TaskSummary, changes: TaskUpdate) {
		const updated = await update(item.id, changes);
		tasks =
			updated.projectId === currentProject.id
				? tasks.map((candidate) =>
						candidate.id === updated.id ? { ...candidate, ...updated } : candidate
					)
				: tasks.filter((candidate) => candidate.id !== updated.id);
		editingTaskId = null;
	}

	function openTaskEditor(item: TaskSummary) {
		editingTaskId = item.id;
	}

	function summaryFromTask(task: Task): TaskSummary {
		return { ...task, subtaskCount: 0, completedSubtaskCount: 0 };
	}

	function beginTaskDrag(event: DragEvent, item: Task) {
		event.stopPropagation();
		endSectionDrag();
		draggedTaskId = item.id;
		taskDropTarget = null;
		if (event.dataTransfer) {
			event.dataTransfer.effectAllowed = 'move';
			event.dataTransfer.setData('text/plain', `task:${item.id}`);
		}
	}

	function allowTaskDrop(event: DragEvent, sectionId: string | null, beforeTaskId: string | null) {
		if (!draggedTaskId) return;
		event.preventDefault();
		event.stopPropagation();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		if (beforeTaskId === draggedTaskId) {
			taskDropTarget = null;
			return;
		}
		const nextTarget = { sectionKey: sectionKey(sectionId), beforeTaskId };
		if (
			taskDropTarget?.sectionKey !== nextTarget.sectionKey ||
			taskDropTarget.beforeTaskId !== nextTarget.beforeTaskId
		) {
			taskDropTarget = nextTarget;
		}
	}

	function endTaskDrag() {
		draggedTaskId = null;
		taskDropTarget = null;
	}

	async function dropTask(event: DragEvent, sectionId: string | null, beforeTaskId: string | null) {
		if (!draggedTaskId) return;
		event.preventDefault();
		event.stopPropagation();
		const taskId = draggedTaskId;
		endTaskDrag();
		const moved = tasks.find((item) => item.id === taskId);
		if (!moved || (beforeTaskId === taskId && moved.sectionId === sectionId)) return;

		const previous = tasks;
		tasks = moveTaskLocally(tasks, taskId, sectionId, beforeTaskId);
		busyTaskIds = [...busyTaskIds, taskId];
		errorMessage = '';
		try {
			tasks = await reorder(taskId, moved.version, sectionId, beforeTaskId);
		} catch {
			tasks = previous;
			errorMessage = 'The task could not be moved. Reload and try again.';
		} finally {
			busyTaskIds = busyTaskIds.filter((id) => id !== taskId);
		}
	}

	function beginSectionDrag(event: DragEvent, section: ProjectSection) {
		endTaskDrag();
		draggedSectionId = section.id;
		sectionDropTarget = null;
		if (event.dataTransfer) {
			event.dataTransfer.effectAllowed = 'move';
			event.dataTransfer.setData('text/plain', `section:${section.id}`);
		}
	}

	function allowSectionDrop(event: DragEvent, beforeSectionId: string | null) {
		if (!draggedSectionId) return;
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		if (beforeSectionId === draggedSectionId) {
			sectionDropTarget = null;
			return;
		}
		if (sectionDropTarget?.beforeSectionId !== beforeSectionId) {
			sectionDropTarget = { beforeSectionId };
		}
	}

	function endSectionDrag() {
		draggedSectionId = null;
		sectionDropTarget = null;
	}

	async function dropSection(event: DragEvent, beforeSectionId: string | null) {
		if (!draggedSectionId) return;
		event.preventDefault();
		const sectionId = draggedSectionId;
		endSectionDrag();
		if (sectionId === beforeSectionId) return;
		const moved = sections.find((section) => section.id === sectionId);
		if (!moved) return;

		const previous = sections;
		sections = moveSectionLocally(sections, sectionId, beforeSectionId);
		busySectionIds = [...busySectionIds, sectionId];
		errorMessage = '';
		try {
			sections = await reorderSection(sectionId, moved.version, beforeSectionId);
		} catch {
			sections = previous;
			errorMessage = 'The section could not be moved. Reload and try again.';
		} finally {
			busySectionIds = busySectionIds.filter((id) => id !== sectionId);
		}
	}

	function buildSectionGroups(
		allSections: ProjectSection[],
		allTasks: TaskSummary[]
	): SectionGroup[] {
		const orderedSections = [...allSections].sort(
			(left, right) =>
				left.position - right.position || left.createdAt.localeCompare(right.createdAt)
		);
		const unsectionedTasks = tasksForSection(allTasks, null);
		const groups: SectionGroup[] = [];
		if (orderedSections.length === 0 || unsectionedTasks.length > 0) {
			groups.push({
				key: 'no-section',
				name: 'No section',
				section: null,
				tasks: unsectionedTasks,
				completed: false,
				showHeader: orderedSections.length > 0
			});
		}
		groups.push(
			...orderedSections.map((section) => ({
				key: section.id,
				name: section.name,
				section,
				tasks: tasksForSection(allTasks, section.id),
				completed: false,
				showHeader: true
			}))
		);
		const completed = allTasks
			.filter((item) => item.status === 'completed')
			.sort((left, right) => (right.completedAt ?? '').localeCompare(left.completedAt ?? ''));
		if (completed.length > 0) {
			groups.push({
				key: 'completed',
				name: 'Completed',
				section: null,
				tasks: completed,
				completed: true,
				showHeader: true
			});
		}
		return groups;
	}

	function tasksForSection(allTasks: TaskSummary[], sectionId: string | null): TaskSummary[] {
		return allTasks
			.filter((item) => item.status === 'active' && item.sectionId === sectionId)
			.sort(
				(left, right) =>
					left.position - right.position || left.createdAt.localeCompare(right.createdAt)
			);
	}

	function moveTaskLocally(
		allTasks: TaskSummary[],
		taskId: string,
		sectionId: string | null,
		beforeTaskId: string | null
	): TaskSummary[] {
		const moved = allTasks.find((item) => item.id === taskId);
		if (!moved) return allTasks;
		const remaining = allTasks.filter((item) => item.id !== taskId);
		const destination = tasksForSection(remaining, sectionId);
		const insertIndex =
			beforeTaskId === null
				? destination.length
				: Math.max(
						destination.findIndex((item) => item.id === beforeTaskId),
						0
					);
		destination.splice(insertIndex, 0, { ...moved, sectionId });
		const positions = new Map(destination.map((item, index) => [item.id, (index + 1) * 1024]));
		return [...remaining, { ...moved, sectionId }].map((item) =>
			positions.has(item.id) ? { ...item, position: positions.get(item.id)! } : item
		);
	}

	function moveSectionLocally(
		allSections: ProjectSection[],
		sectionId: string,
		beforeSectionId: string | null
	): ProjectSection[] {
		const ordered = [...allSections].sort((left, right) => left.position - right.position);
		const moved = ordered.find((section) => section.id === sectionId);
		if (!moved) return allSections;
		const remaining = ordered.filter((section) => section.id !== sectionId);
		const insertIndex =
			beforeSectionId === null
				? remaining.length
				: Math.max(
						remaining.findIndex((section) => section.id === beforeSectionId),
						0
					);
		remaining.splice(insertIndex, 0, moved);
		return remaining.map((section, index) => ({ ...section, position: (index + 1) * 1024 }));
	}

	function sectionKey(sectionId: string | null): string {
		return sectionId ?? 'no-section';
	}

	function isTaskDropTarget(sectionId: string | null, beforeTaskId: string | null): boolean {
		return (
			taskDropTarget?.sectionKey === sectionKey(sectionId) &&
			taskDropTarget.beforeTaskId === beforeTaskId
		);
	}

	function isSectionDropTarget(beforeSectionId: string | null): boolean {
		return sectionDropTarget?.beforeSectionId === beforeSectionId;
	}

	function taskDropAnnouncement(): string {
		if (!taskDropTarget) return '';
		const group = sectionGroups.find((candidate) => candidate.key === taskDropTarget?.sectionKey);
		if (!group) return '';
		if (taskDropTarget.beforeTaskId === null) return `Move task to the end of ${group.name}`;
		const beforeTask = tasks.find((item) => item.id === taskDropTarget?.beforeTaskId);
		return beforeTask
			? `Move task before ${beforeTask.title} in ${group.name}`
			: `Move task to ${group.name}`;
	}

	function formatDue(item: Task): string {
		if (!item.dueDate) return '';
		const date = parseDate(item.dueDate);
		const today = startOfDay(new Date());
		const tomorrow = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 1);
		let label = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(date);
		if (date.getTime() === today.getTime()) label = 'Today';
		if (date.getTime() === tomorrow.getTime()) label = 'Tomorrow';
		if (item.dueTime) label += ` · ${formatTime(item.dueTime)}`;
		return label;
	}

	function formatTime(value: string): string {
		const [hours, minutes] = value.split(':').map(Number);
		return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(
			new Date(2000, 0, 1, hours, minutes)
		);
	}

	function isOverdue(item: Task): boolean {
		if (item.status !== 'active' || !item.dueDate) return false;
		const now = new Date();
		const date = parseDate(item.dueDate);
		const today = startOfDay(now);
		if (date < today) return true;
		if (date > today || !item.dueTime) return false;
		const [hours, minutes] = item.dueTime.split(':').map(Number);
		return new Date(now.getFullYear(), now.getMonth(), now.getDate(), hours, minutes) < now;
	}

	function parseDate(value: string): Date {
		const [year, month, day] = value.split('-').map(Number);
		return new Date(year, month - 1, day);
	}

	function startOfDay(value: Date): Date {
		return new Date(value.getFullYear(), value.getMonth(), value.getDate());
	}

	function priorityLabel(priority: number): string {
		return ['', 'Low', 'Medium', 'High', 'Urgent'][priority] ?? '';
	}
</script>

<section
	class:board={currentProject.layout === 'board'}
	class:task-dragging={draggedTaskId !== null}
	class="project-task-view"
>
	<header class="view-header">
		<div>
			<p>{currentProject.name}</p>
			<h1>Tasks</h1>
		</div>
		<div class="header-actions">
			<span>{activeTasks.length} active</span>
			<div class="layout-switch" aria-label="Tasks layout">
				<button
					type="button"
					class:active={currentProject.layout === 'list'}
					aria-pressed={currentProject.layout === 'list'}
					disabled={changingLayout}
					onclick={() => void setLayout('list')}>List</button
				>
				<button
					type="button"
					class:active={currentProject.layout === 'board'}
					aria-pressed={currentProject.layout === 'board'}
					disabled={changingLayout}
					onclick={() => void setLayout('board')}>Board</button
				>
			</div>
		</div>
	</header>

	{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}
	<p class="sr-only" aria-live="polite">{taskDropAnnouncement()}</p>

	<div class="sections" class:board-sections={currentProject.layout === 'board'}>
		{#each sectionGroups as group (group.key)}
			<section
				class="project-section"
				class:completed={group.completed}
				class:dragging-section={draggedSectionId === group.section?.id}
				data-dragging={draggedSectionId === group.section?.id ? 'true' : undefined}
				aria-label={group.name}
				ondragover={group.section
					? (event) => allowSectionDrop(event, group.section!.id)
					: undefined}
				ondrop={group.section ? (event) => void dropSection(event, group.section!.id) : undefined}
			>
				{#if group.section && isSectionDropTarget(group.section.id)}
					<span
						class="insertion-marker section-insertion-marker"
						data-insertion-marker="section"
						aria-hidden="true"
					></span>
				{/if}
				{#if group.showHeader}
					<header class="section-header">
						{#if group.section && editingSectionId === group.section.id}
							<form
								class="rename-section"
								onsubmit={(event) => {
									event.preventDefault();
									void renameSection(group.section!);
								}}
							>
								<input aria-label="Section name" maxlength="200" bind:value={editingSectionName} />
								<button type="button" onclick={() => (editingSectionId = null)}>Cancel</button>
								<button type="submit" disabled={!editingSectionName.trim()}>Save</button>
							</form>
						{:else}
							<div
								class:section-drag-handle={group.section !== null}
								role="group"
								aria-label={group.section ? `Move section ${group.name}` : group.name}
								draggable={group.section !== null}
								ondragstart={group.section
									? (event) => beginSectionDrag(event, group.section!)
									: undefined}
								ondragend={endSectionDrag}
							>
								<h2>{group.name}</h2>
								<span>{group.tasks.length}</span>
							</div>
							{#if group.section}
								<div class="section-actions">
									<button type="button" onclick={() => beginSectionRename(group.section!)}
										>Rename</button
									>
									<button
										type="button"
										disabled={busySectionIds.includes(group.section.id)}
										onclick={() => void removeSection(group.section!)}>Delete</button
									>
								</div>
							{/if}
						{/if}
					</header>
				{/if}

				<ul
					class:empty-task-list={!group.completed && group.tasks.length === 0}
					aria-label={`${group.name} tasks`}
					ondragover={!group.completed
						? (event) => allowTaskDrop(event, group.section?.id ?? null, null)
						: undefined}
					ondrop={!group.completed
						? (event) => void dropTask(event, group.section?.id ?? null, null)
						: undefined}
				>
					{#each group.tasks as item (item.id)}
						<li
							class="task-card"
							class:done={item.status === 'completed'}
							class:dragging={draggedTaskId === item.id}
							data-dragging={draggedTaskId === item.id ? 'true' : undefined}
							draggable={!group.completed}
							ondragstart={(event) => beginTaskDrag(event, item)}
							ondragend={endTaskDrag}
							ondragover={!group.completed
								? (event) => allowTaskDrop(event, group.section?.id ?? null, item.id)
								: undefined}
							ondrop={!group.completed
								? (event) => void dropTask(event, group.section?.id ?? null, item.id)
								: undefined}
						>
							{#if isTaskDropTarget(group.section?.id ?? null, item.id)}
								<span
									class="insertion-marker task-insertion-marker"
									data-insertion-marker="task"
									aria-hidden="true"
								></span>
							{/if}
							<button
								class="task-toggle"
								class:checked={item.status === 'completed'}
								type="button"
								aria-label={`${item.status === 'active' ? 'Complete' : 'Reopen'} ${item.title}`}
								disabled={busyTaskIds.includes(item.id)}
								onclick={(event) => {
									event.stopPropagation();
									void changeStatus(item);
								}}>{item.status === 'completed' ? '✓' : ''}</button
							>
							<button
								class="task-copy"
								type="button"
								aria-label={`Open ${item.title}`}
								onclick={() => openTaskEditor(item)}
							>
								<strong>{item.title}</strong>
								{#if item.description}<span class="description">{item.description}</span>{/if}
								{#if item.dueDate || item.priority > 0 || item.subtaskCount > 0}
									<span class="metadata">
										<SubtaskProgress
											completed={item.completedSubtaskCount}
											total={item.subtaskCount}
										/>
										{#if item.dueDate}
											<span class:overdue={isOverdue(item)}>{formatDue(item)}</span>
										{/if}
										{#if item.priority > 0}
											<span class={`priority priority-${item.priority}`}
												>{priorityLabel(item.priority)}</span
											>
										{/if}
									</span>
								{/if}
							</button>
							<div class="task-actions">
								<button
									type="button"
									aria-label={`Edit ${item.title}`}
									onclick={(event) => {
										event.stopPropagation();
										openTaskEditor(item);
									}}>Edit</button
								>
								<button
									type="button"
									aria-label={`Delete ${item.title}`}
									disabled={busyTaskIds.includes(item.id)}
									onclick={(event) => {
										event.stopPropagation();
										void deleteTask(item);
									}}>Delete</button
								>
							</div>
						</li>
					{/each}
					{#if !group.completed}
						<li
							class="task-drop-end"
							class:empty-drop={group.tasks.length === 0}
							aria-label={`Drop task in ${group.name}`}
							ondragover={(event) => allowTaskDrop(event, group.section?.id ?? null, null)}
							ondrop={(event) => void dropTask(event, group.section?.id ?? null, null)}
						>
							{#if isTaskDropTarget(group.section?.id ?? null, null)}
								<span
									class="insertion-marker task-insertion-marker"
									data-insertion-marker="task"
									aria-hidden="true"
								></span>
							{/if}
						</li>
					{/if}
				</ul>

				{#if !group.completed}
					<TaskQuickAdd
						{create}
						oncreated={(created) =>
							(tasks = [
								...tasks.filter((item) => item.id !== created.id),
								summaryFromTask(created)
							])}
						{projects}
						{sections}
						{loadSections}
						initialProjectId={currentProject.id}
						initialSectionId={group.section?.id ?? null}
						label={`Add task to ${group.name}`}
					/>
				{/if}
			</section>
		{/each}

		<section
			class="add-section"
			role="group"
			aria-label="Add or move section"
			ondragover={(event) => allowSectionDrop(event, null)}
			ondrop={(event) => void dropSection(event, null)}
		>
			{#if isSectionDropTarget(null)}
				<span
					class="insertion-marker section-insertion-marker"
					data-insertion-marker="section"
					aria-hidden="true"
				></span>
			{/if}
			<form
				onsubmit={(event) => {
					event.preventDefault();
					void addSection();
				}}
			>
				<label class="sr-only" for="new-section">Section name</label>
				<input
					id="new-section"
					bind:value={newSectionName}
					maxlength="200"
					placeholder="Add section"
				/>
				<button type="submit" disabled={creatingSection || !newSectionName.trim()}>Add</button>
			</form>
		</section>
	</div>
</section>

{#if editingTask}
	<TaskEditorModal
		task={editingTask}
		{projects}
		{sections}
		{loadSections}
		save={(changes) => saveTask(editingTask, changes)}
		close={() => (editingTaskId = null)}
	/>
{/if}

<style>
	.project-task-view {
		width: min(52rem, 100%);
		margin: 0 auto;
	}
	.project-task-view.board {
		width: 100%;
	}
	.view-header,
	.section-header,
	.section-header > div,
	.header-actions,
	.layout-switch,
	.task-card,
	.task-actions,
	.section-actions,
	.quick-add,
	.add-section form,
	.rename-section {
		display: flex;
		align-items: center;
	}
	.view-header {
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1.5rem;
	}
	.view-header p {
		margin: 0 0 0.3rem;
		color: var(--theme-accent, #52705a);
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
	.header-actions {
		gap: 0.8rem;
		color: #7c817d;
		font-size: 0.75rem;
	}
	.layout-switch {
		padding: 0.16rem;
		border: 1px solid var(--theme-border, #d9e1d7);
		border-radius: 0.5rem;
		background: var(--theme-sidebar, #f1f5ef);
	}
	.layout-switch button {
		padding: 0.38rem 0.6rem;
		border: 0;
		border-radius: 0.35rem;
		color: #687068;
		background: transparent;
		font-size: 0.72rem;
		font-weight: 700;
	}
	.layout-switch button.active {
		color: var(--theme-accent, #245937);
		background: #fff;
		box-shadow: 0 1px 4px color-mix(in srgb, var(--theme-accent, #2d6540) 10%, transparent);
	}
	.sections {
		display: grid;
		gap: 1.5rem;
	}
	.sections.board-sections {
		grid-auto-columns: minmax(17rem, 20rem);
		grid-auto-flow: column;
		grid-template-columns: none;
		align-items: start;
		overflow-x: auto;
		padding: 0.25rem 0 1.5rem;
		scroll-snap-type: x proximity;
	}
	.board-sections .project-section,
	.board-sections .add-section {
		scroll-snap-align: start;
	}
	.project-section,
	.add-section {
		position: relative;
	}
	.project-section.dragging-section {
		opacity: 0.38;
	}
	.insertion-marker {
		position: absolute;
		border-radius: 999px;
		background: var(--theme-accent, #4f8a60);
		box-shadow: 0 0 0 2px var(--theme-canvas, #f8faf7);
		pointer-events: none;
		z-index: 3;
	}
	.insertion-marker::before {
		position: absolute;
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 50%;
		background: var(--theme-accent, #4f8a60);
		content: '';
	}
	.section-insertion-marker {
		top: -1.1rem;
		left: 0;
		right: 0;
		height: 0.18rem;
	}
	.section-insertion-marker::before {
		top: -0.16rem;
		left: -0.18rem;
	}
	.board-sections .section-insertion-marker {
		top: 0;
		bottom: 0;
		left: -0.1rem;
		right: auto;
		width: 0.18rem;
		height: auto;
	}
	.board-sections .section-insertion-marker::before {
		top: -0.18rem;
		left: -0.16rem;
	}
	.section-header {
		justify-content: space-between;
		gap: 0.5rem;
		min-height: 2.25rem;
		padding-bottom: 0.55rem;
		border-bottom: 1px solid var(--theme-border, #d9dfd7);
	}
	.section-header > div:first-child {
		gap: 0.45rem;
		min-width: 0;
	}
	.section-drag-handle {
		cursor: grab;
	}
	.section-drag-handle:active,
	.task-card:active {
		cursor: grabbing;
	}
	.section-header h2 {
		overflow: hidden;
		margin: 0;
		font-size: 0.87rem;
		font-weight: 760;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.section-header span {
		color: #989d98;
		font-size: 0.7rem;
	}
	.section-actions,
	.task-actions {
		gap: 0.25rem;
		opacity: 0;
		transition: opacity 120ms ease;
	}
	.project-section:hover .section-actions,
	.task-card:hover .task-actions,
	.task-card:focus-within .task-actions {
		opacity: 1;
	}
	.section-actions button,
	.task-actions button {
		padding: 0.25rem 0.35rem;
		border: 0;
		color: #798079;
		background: transparent;
		font-size: 0.67rem;
		cursor: pointer;
	}
	.project-section ul {
		position: relative;
		display: grid;
		gap: 0;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.project-section ul.empty-task-list {
		position: relative;
		min-height: 0.4rem;
	}
	.task-card {
		position: relative;
		gap: 0.7rem;
		min-width: 0;
		padding: 0.72rem 0.15rem;
		border-bottom: 1px solid var(--theme-border, #e6e9e4);
		cursor: grab;
	}
	.task-card.dragging {
		opacity: 0.38;
	}
	.task-insertion-marker {
		top: -0.15rem;
		left: 0.25rem;
		right: 0;
		height: 0.18rem;
	}
	.task-insertion-marker::before {
		top: -0.16rem;
		left: -0.18rem;
	}
	.board-sections .task-card {
		align-items: flex-start;
		margin-top: 0.55rem;
		padding: 0.8rem;
		border: 1px solid var(--theme-border, #dbe2d9);
		border-radius: 0.65rem;
		background: #fff;
		box-shadow: 0 0.25rem 0.8rem color-mix(in srgb, var(--theme-accent, #2d6540) 5%, transparent);
	}
	.board-sections .task-card > .task-insertion-marker {
		top: -0.45rem;
	}
	.board-sections .task-card > .task-insertion-marker::before {
		top: -0.16rem;
	}
	.task-card.done {
		opacity: 0.65;
		cursor: default;
	}
	.task-toggle {
		display: grid;
		flex: 0 0 auto;
		place-items: center;
		width: 1.15rem;
		height: 1.15rem;
		padding: 0;
		border: 1.5px solid #909890;
		border-radius: 50%;
		color: #fff;
		background: transparent;
		font-size: 0.68rem;
		cursor: pointer;
	}
	.task-toggle.checked {
		border-color: var(--theme-accent, #477d56);
		background: var(--theme-accent, #477d56);
	}
	.task-copy {
		display: grid;
		min-width: 0;
		flex: 1;
		gap: 0.22rem;
		padding: 0;
		border: 0;
		color: inherit;
		background: transparent;
		text-align: left;
		cursor: pointer;
	}
	.task-copy strong {
		font-size: 0.87rem;
		font-weight: 520;
		line-height: 1.35;
	}
	.done .task-copy strong {
		text-decoration: line-through;
	}
	.description {
		overflow: hidden;
		color: #7e847f;
		font-size: 0.72rem;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.metadata {
		display: flex;
		gap: 0.55rem;
		color: var(--theme-accent, #477d56);
		font-size: 0.68rem;
		font-weight: 650;
	}
	.metadata .overdue {
		color: #c13d33;
	}
	.priority-3,
	.priority-4 {
		color: #b05535;
	}
	.task-drop-end {
		position: relative;
		min-height: 0.4rem;
	}
	.task-drop-end.empty-drop {
		min-height: 0.4rem;
	}
	.task-dragging .task-drop-end {
		min-height: 2.25rem;
		z-index: 2;
	}
	.task-dragging .task-drop-end:not(.empty-drop) {
		position: absolute;
		right: 0;
		bottom: -1.125rem;
		left: 0;
	}
	.task-dragging .task-drop-end.empty-drop {
		position: absolute;
		top: 0;
		right: 0;
		left: 0;
		min-height: 3.25rem;
	}
	.task-dragging .board-sections .project-section :global(.task-quick-add) {
		margin-top: 1.4rem;
	}
	.task-drop-end > .task-insertion-marker {
		top: 50%;
		transform: translateY(-50%);
	}
	.task-drop-end.empty-drop > .task-insertion-marker {
		top: 0.2rem;
		transform: none;
	}
	.add-section form,
	.rename-section {
		gap: 0.45rem;
	}
	.add-section input,
	.rename-section input {
		min-width: 0;
		flex: 1;
		padding: 0.55rem 0.6rem;
		border: 1px solid transparent;
		border-radius: 0.45rem;
		background: transparent;
		outline: none;
	}
	.add-section input:focus,
	.rename-section input:focus {
		border-color: color-mix(in srgb, var(--theme-accent, #2d6540) 42%, transparent);
		background: #fff;
	}
	.add-section button,
	.rename-section button {
		padding: 0.48rem 0.6rem;
		border: 0;
		border-radius: 0.4rem;
		color: #fff;
		background: var(--theme-accent, #2d6540);
		font-size: 0.7rem;
		font-weight: 700;
		cursor: pointer;
	}
	.rename-section button[type='button'] {
		color: #687068;
		background: var(--theme-hover, #edf2eb);
	}
	button:disabled {
		cursor: wait;
		opacity: 0.5;
	}
	.add-section {
		min-height: 3rem;
		padding-top: 0.2rem;
	}
	.board-sections .add-section {
		min-height: 0;
		padding: 0.25rem 0.35rem;
		border: 1px dashed var(--theme-border, #cbd8c9);
		border-radius: 0.65rem;
	}
	.board-sections .add-section input {
		padding: 0.45rem 0.5rem;
	}
	.error {
		margin: 0 0 1rem;
		padding: 0.7rem 0.8rem;
		border-radius: 0.5rem;
		color: #8c2828;
		background: #fff0ee;
		font-size: 0.78rem;
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
	@media (max-width: 42rem) {
		.view-header {
			align-items: flex-start;
		}
		.header-actions {
			align-items: flex-end;
			flex-direction: column;
		}
		.section-actions,
		.task-actions {
			opacity: 1;
		}
	}
</style>
