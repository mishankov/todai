<script lang="ts">
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import TaskEditor from './TaskEditor.svelte';

	interface Props {
		initialTasks: Task[];
		create?: (title: string) => Promise<Task>;
		complete: (taskId: string) => Promise<Task>;
		reopen: (taskId: string) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string) => Promise<void>;
		eyebrow: string;
		heading: string;
		countNoun?: string;
		emptyTitle: string;
		emptyMessage: string;
		listLabel: string;
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
		listLabel
	}: Props = $props();
	let tasks = $derived([...initialTasks]);
	let title = $state('');
	let creating = $state(false);
	let busyTaskIds = $state<string[]>([]);
	let errorMessage = $state('');
	let editingTaskId = $state<string | null>(null);
	let activeTasks = $derived(tasks.filter((item) => item.status === 'active'));
	let completedTasks = $derived(tasks.filter((item) => item.status === 'completed'));
	let orderedTasks = $derived([...activeTasks, ...completedTasks]);

	async function addTask() {
		if (!title.trim() || !create) return;

		creating = true;
		errorMessage = '';
		try {
			const created = await create(title);
			tasks = [...tasks, created];
			title = '';
		} catch {
			errorMessage = 'The task could not be created. Please try again.';
		} finally {
			creating = false;
		}
	}

	async function changeStatus(item: Task) {
		const previousItem = item;
		const nextStatus = item.status === 'active' ? 'completed' : 'active';
		const optimisticItem: Task = {
			...item,
			status: nextStatus,
			completedAt: nextStatus === 'completed' ? new Date().toISOString() : null
		};

		busyTaskIds = [...busyTaskIds, item.id];
		errorMessage = '';
		tasks = tasks.map((candidate) => (candidate.id === item.id ? optimisticItem : candidate));
		try {
			const updated = item.status === 'active' ? await complete(item.id) : await reopen(item.id);
			tasks = tasks.map((candidate) => (candidate.id === updated.id ? updated : candidate));
		} catch {
			tasks = tasks.map((candidate) => (candidate.id === item.id ? previousItem : candidate));
			errorMessage = 'The task could not be updated. Please try again.';
		} finally {
			busyTaskIds = busyTaskIds.filter((taskId) => taskId !== item.id);
		}
	}

	async function deleteItem(item: Task) {
		const previousIndex = tasks.findIndex((candidate) => candidate.id === item.id);

		busyTaskIds = [...busyTaskIds, item.id];
		errorMessage = '';
		tasks = tasks.filter((candidate) => candidate.id !== item.id);
		try {
			await remove(item.id);
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

	async function saveItem(item: Task, changes: TaskUpdate) {
		const updated = await update(item.id, changes);
		tasks = tasks.map((candidate) => (candidate.id === updated.id ? updated : candidate));
		editingTaskId = null;
	}

	function dueLabel(item: Task): string {
		if (!item.dueAt) return '';

		const due = new Date(item.dueAt);
		const now = new Date();
		const time = new Intl.DateTimeFormat(undefined, {
			hour: '2-digit',
			minute: '2-digit'
		}).format(due);
		const date = sameLocalDay(due, now)
			? time
			: new Intl.DateTimeFormat(undefined, {
					month: 'short',
					day: 'numeric',
					hour: '2-digit',
					minute: '2-digit'
				}).format(due);
		if (item.status === 'active' && due.getTime() < now.getTime()) {
			return `Overdue · ${date}`;
		}

		return sameLocalDay(due, now) ? `Today · ${time}` : date;
	}

	function isOverdue(item: Task): boolean {
		return item.status === 'active' && item.dueAt !== null && new Date(item.dueAt) < new Date();
	}

	function sameLocalDay(left: Date, right: Date): boolean {
		return (
			left.getFullYear() === right.getFullYear() &&
			left.getMonth() === right.getMonth() &&
			left.getDate() === right.getDate()
		);
	}

	function priorityLabel(priority: number): string {
		return ['', 'Low', 'Medium', 'High', 'Urgent'][priority] ?? '';
	}
</script>

<section class="inbox" aria-labelledby="inbox-heading">
	<header class="inbox-header">
		<div>
			<p class="eyebrow">{eyebrow}</p>
			<h1 id="inbox-heading">{heading}</h1>
		</div>
		<span class="count">{activeTasks.length} {countNoun}</span>
	</header>

	{#if create}
		<form
			class="quick-add"
			onsubmit={(event) => {
				event.preventDefault();
				void addTask();
			}}
		>
			<label class="sr-only" for="new-task">Task title</label>
			<input
				id="new-task"
				name="title"
				placeholder="Add a task…"
				autocomplete="off"
				maxlength="500"
				bind:value={title}
				disabled={creating}
			/>
			<button type="submit" disabled={creating || !title.trim()}>
				{creating ? 'Adding…' : 'Add task'}
			</button>
		</form>
	{/if}

	{#if errorMessage}
		<p class="error" role="alert">{errorMessage}</p>
	{/if}

	<div class="task-space">
		{#if orderedTasks.length > 0}
			<ul class="task-list" aria-label={listLabel}>
				{#each orderedTasks as item, index (item.id)}
					{#if item.status === 'completed' && index === activeTasks.length}
						<li class="section-heading" role="presentation">
							<span>Completed · {completedTasks.length}</span>
						</li>
					{/if}
					{#if editingTaskId === item.id}
						<li class="editor-row">
							<TaskEditor
								task={item}
								save={(changes) => saveItem(item, changes)}
								cancel={() => (editingTaskId = null)}
							/>
						</li>
					{:else}
						<li class:completed-task={item.status === 'completed'}>
							<button
								class:checked={item.status === 'completed'}
								class="task-toggle"
								type="button"
								aria-label={`${item.status === 'active' ? 'Complete' : 'Reopen'} ${item.title}`}
								disabled={busyTaskIds.includes(item.id)}
								onclick={() => void changeStatus(item)}
							>
								{item.status === 'completed' ? '✓' : ''}
							</button>
							<span class="task-copy">
								<span class="task-title">{item.title}</span>
								{#if item.dueAt || item.priority > 0}
									<span class="task-metadata">
										{#if item.dueAt}
											<span class:overdue={isOverdue(item)} class="due">{dueLabel(item)}</span>
										{/if}
										{#if item.priority > 0}
											<span class={`priority priority-${item.priority}`}>
												{priorityLabel(item.priority)}
											</span>
										{/if}
									</span>
								{/if}
							</span>
							<button
								class="edit-task"
								type="button"
								aria-label={`Edit ${item.title}`}
								onclick={() => (editingTaskId = item.id)}>Edit</button
							>
							<button
								class="delete-task"
								type="button"
								aria-label={`Delete ${item.title}`}
								disabled={busyTaskIds.includes(item.id)}
								onclick={() => void deleteItem(item)}>Delete</button
							>
						</li>
					{/if}
				{/each}
			</ul>
		{:else}
			<div class="empty-state">
				<p>{emptyTitle}</p>
				<span>{emptyMessage}</span>
			</div>
		{/if}
	</div>
</section>

<style>
	.inbox {
		width: min(46rem, 100%);
		margin: 0 auto;
	}

	.inbox-header {
		display: flex;
		align-items: end;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 2rem;
	}

	.eyebrow {
		margin: 0 0 0.4rem;
		color: #52705a;
		font-size: 0.72rem;
		font-weight: 750;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: clamp(2.25rem, 7vw, 3.5rem);
		line-height: 1;
		letter-spacing: -0.055em;
	}

	.count {
		padding: 0.45rem 0.65rem;
		border-radius: 999px;
		color: #52705a;
		background: #edf3ec;
		font-size: 0.76rem;
		font-weight: 700;
	}

	.quick-add {
		display: grid;
		grid-template-columns: 1fr auto;
		gap: 0.65rem;
		padding: 0.6rem;
		border: 1px solid #d8e0d6;
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 0.75rem 2.5rem rgb(24 56 34 / 6%);
	}

	.quick-add input {
		min-width: 0;
		padding: 0.7rem;
		border: 0;
		color: #17211a;
		background: transparent;
		outline: none;
	}

	.quick-add button {
		padding: 0.7rem 0.9rem;
		border: 0;
		border-radius: 0.7rem;
		color: #fff;
		background: #2d6540;
		font-size: 0.82rem;
		font-weight: 750;
		cursor: pointer;
	}

	.quick-add button:disabled {
		cursor: default;
		opacity: 0.45;
	}

	.task-list {
		margin: 1.5rem 0 0;
		padding: 0;
		list-style: none;
	}

	.task-list li {
		display: flex;
		align-items: center;
		gap: 0.85rem;
		min-height: 3.4rem;
		border-bottom: 1px solid #e1e6df;
	}

	.task-list .editor-row {
		padding: 0.85rem 0;
		border-bottom: 0;
	}

	.task-space {
		min-height: 12rem;
	}

	.task-list .section-heading {
		min-height: auto;
		padding: 1.75rem 0 0.55rem;
		border-bottom: 0;
		color: #657168;
		font-size: 0.78rem;
		font-weight: 750;
		letter-spacing: 0.04em;
	}

	.task-toggle {
		display: grid;
		flex: 0 0 auto;
		width: 1.35rem;
		height: 1.35rem;
		place-items: center;
		padding: 0;
		border: 1.5px solid #8ea092;
		border-radius: 50%;
		color: #fff;
		background: transparent;
		font-size: 0.72rem;
		cursor: pointer;
	}

	.task-toggle:hover:not(:disabled) {
		border-color: #2d6540;
		box-shadow: inset 0 0 0 0.25rem #edf5ed;
	}

	.task-toggle.checked {
		border-color: #5b8165;
		background: #5b8165;
	}

	.task-toggle:disabled {
		cursor: wait;
		opacity: 0.55;
	}

	.task-copy {
		display: grid;
		flex: 1;
		gap: 0.2rem;
		min-width: 0;
	}

	.task-title {
		overflow-wrap: anywhere;
	}

	.task-metadata {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		color: #718076;
		font-size: 0.72rem;
	}

	.due.overdue {
		color: #a13f38;
		font-weight: 700;
	}

	.priority {
		font-weight: 700;
	}

	.priority-3 {
		color: #a35d23;
	}

	.priority-4 {
		color: #a13f38;
	}

	.edit-task,
	.delete-task {
		padding: 0.4rem 0.5rem;
		border: 0;
		border-radius: 0.45rem;
		background: transparent;
		font-size: 0.74rem;
		font-weight: 650;
		cursor: pointer;
		opacity: 0.7;
	}

	.edit-task {
		color: #526b58;
	}

	.edit-task:hover {
		color: #2d6540;
		background: #edf5ed;
		opacity: 1;
	}

	.delete-task {
		color: #8a514f;
	}

	.delete-task:hover:not(:disabled) {
		color: #8c2828;
		background: #fff0ef;
		opacity: 1;
	}

	.delete-task:disabled {
		cursor: wait;
		opacity: 0.35;
	}

	.empty-state {
		padding: 4rem 1rem;
		text-align: center;
	}

	.empty-state p {
		margin: 0 0 0.4rem;
		font-weight: 750;
	}

	.empty-state span {
		color: #718076;
		font-size: 0.9rem;
	}

	.completed-task .task-title {
		text-decoration: line-through;
		opacity: 0.75;
	}

	.error {
		margin: 0.85rem 0 0;
		color: #8c2828;
		font-size: 0.86rem;
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

	@media (max-width: 34rem) {
		.quick-add {
			grid-template-columns: 1fr;
		}
	}
</style>
