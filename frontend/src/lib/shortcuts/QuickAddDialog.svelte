<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import { onMount, tick, untrack } from 'svelte';

	interface Props {
		projects: Project[];
		initialProjectId: string;
		initialSectionId?: string | null;
		shortcutLabel: string;
		focusRequest: number;
		loadSections: (projectId: string) => Promise<ProjectSection[]>;
		createTask: (title: string, projectId: string, sectionId: string | null) => Promise<Task>;
		updateTask: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		close: () => void;
		saved: (task: Task) => void | Promise<void>;
	}

	let {
		projects,
		initialProjectId,
		initialSectionId = null,
		shortcutLabel,
		focusRequest,
		loadSections,
		createTask,
		updateTask,
		close,
		saved
	}: Props = $props();
	let title = $state('');
	let projectId = $state(untrack(() => initialProjectId));
	let sectionId = $state(untrack(() => initialSectionId ?? ''));
	let sections = $state<ProjectSection[]>([]);
	let priority = $state('0');
	let dueDate = $state('');
	let dueTime = $state('');
	let loadingSections = $state(false);
	let saving = $state(false);
	let errorMessage = $state('');
	let stagedTask = $state<Task | null>(null);
	let titleInput: HTMLInputElement;
	let dialog: HTMLDivElement;
	let sectionLoad = 0;

	onMount(() => {
		void refreshSections(projectId, true);
		void tick().then(() => titleInput?.focus());
	});

	$effect(() => {
		focusTitle(focusRequest);
	});

	function focusTitle(request: number) {
		if (request < 0) return;
		void tick().then(() => titleInput?.focus());
	}

	async function changeProject(event: Event) {
		projectId = (event.currentTarget as HTMLSelectElement).value;
		sectionId = '';
		await refreshSections(projectId, false);
	}

	async function refreshSections(nextProjectId: string, keepInitial: boolean) {
		const request = ++sectionLoad;
		loadingSections = true;
		try {
			const loaded = await loadSections(nextProjectId);
			if (request !== sectionLoad) return;
			sections = loaded;
			if (!keepInitial || !loaded.some((section) => section.id === sectionId)) sectionId = '';
		} catch {
			if (request !== sectionLoad) return;
			sections = [];
			sectionId = '';
			errorMessage = 'Sections could not be loaded. You can still create the task in Inbox.';
		} finally {
			if (request === sectionLoad) loadingSections = false;
		}
	}

	async function save() {
		if (!title.trim() || !projectId || saving) return;
		saving = true;
		errorMessage = '';
		try {
			let task = stagedTask;
			if (!task) {
				task = await createTask(title.trim(), projectId, sectionId || null);
				stagedTask = task;
			}

			const changes = taskChanges(task.version);
			const needsUpdate =
				task.title !== changes.title ||
				task.projectId !== changes.projectId ||
				task.sectionId !== changes.sectionId ||
				task.priority !== changes.priority ||
				task.dueDate !== changes.dueDate ||
				task.dueTime !== changes.dueTime ||
				task.dueTimezone !== changes.dueTimezone;
			if (needsUpdate) task = await updateTask(task.id, changes);
			await saved(task);
		} catch (error) {
			errorMessage =
				error instanceof Error && error.message
					? error.message
					: 'The task could not be saved. Your entries are still here.';
		} finally {
			saving = false;
		}
	}

	function taskChanges(version: number): TaskUpdate & {
		title: string;
		projectId: string;
		sectionId: string | null;
		priority: number;
		dueDate: string | null;
		dueTime: string | null;
		dueTimezone: string | null;
	} {
		const hasTime = Boolean(dueDate && dueTime);
		return {
			version,
			title: title.trim(),
			projectId,
			sectionId: sectionId || null,
			priority: Number(priority),
			dueDate: dueDate || null,
			dueTime: hasTime ? dueTime : null,
			dueTimezone: hasTime ? Intl.DateTimeFormat().resolvedOptions().timeZone : null
		};
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && event.shiftKey) {
			event.preventDefault();
			return;
		}
		if (event.key !== 'Tab') return;
		const focusable = Array.from(
			dialog.querySelectorAll<HTMLElement>(
				'button:not([disabled]), input:not([disabled]), select:not([disabled])'
			)
		);
		if (focusable.length === 0) return;
		const first = focusable[0];
		const last = focusable.at(-1)!;
		if (event.shiftKey && document.activeElement === first) {
			event.preventDefault();
			last.focus();
		} else if (!event.shiftKey && document.activeElement === last) {
			event.preventDefault();
			first.focus();
		}
	}
</script>

<div
	class="backdrop"
	role="presentation"
	onclick={(event) => {
		if (event.target === event.currentTarget) close();
	}}
>
	<div
		bind:this={dialog}
		class="dialog"
		role="dialog"
		tabindex="-1"
		aria-modal="true"
		aria-labelledby="quick-add-title"
		onkeydown={handleKeydown}
	>
		<header>
			<div>
				<p>QUICK ADD · {shortcutLabel}</p>
				<h2 id="quick-add-title">Create a task</h2>
			</div>
			<button type="button" class="close" aria-label="Close quick add" onclick={close}>×</button>
		</header>

		<form
			onsubmit={(event) => {
				event.preventDefault();
				void save();
			}}
		>
			<label class="title-field">
				<span>Title</span>
				<input
					bind:this={titleInput}
					bind:value={title}
					name="title"
					maxlength="500"
					autocomplete="off"
					required
				/>
			</label>

			<div class="field-grid">
				<label>
					<span>Project</span>
					<select
						value={projectId}
						onchange={changeProject}
						disabled={saving || stagedTask !== null}
					>
						{#each projects as project (project.id)}
							<option value={project.id}>{project.name}</option>
						{/each}
					</select>
				</label>
				<label>
					<span>Section</span>
					<select
						bind:value={sectionId}
						disabled={loadingSections || saving || stagedTask !== null}
					>
						<option value="">Inbox (no section)</option>
						{#each sections as section (section.id)}
							<option value={section.id}>{section.name}</option>
						{/each}
					</select>
				</label>
				<label>
					<span>Priority</span>
					<select bind:value={priority}>
						<option value="0">None</option>
						<option value="1">Low</option>
						<option value="2">Medium</option>
						<option value="3">High</option>
						<option value="4">Urgent</option>
					</select>
				</label>
				<label>
					<span>Due date</span>
					<input type="date" bind:value={dueDate} />
				</label>
				<label>
					<span>Due time</span>
					<input type="time" bind:value={dueTime} disabled={!dueDate} />
				</label>
			</div>

			{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}

			<footer>
				<span>Enter to save · Shift+Enter does not save</span>
				<button type="submit" disabled={saving || !title.trim() || !projectId}>
					{saving ? 'Saving…' : 'Create task'}
				</button>
			</footer>
		</form>
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 80;
		display: grid;
		place-items: start center;
		padding: min(12vh, 6rem) 1rem 1rem;
		background: rgb(24 25 23 / 45%);
		backdrop-filter: blur(2px);
	}
	.dialog {
		width: min(42rem, 100%);
		border: 1px solid var(--theme-border, #dfe5dc);
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 1.5rem 5rem rgb(20 28 21 / 24%);
		overflow: hidden;
	}
	header,
	footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
	}
	header {
		padding: 1.25rem 1.35rem;
		border-bottom: 1px solid var(--theme-border, #dfe5dc);
		background: var(--theme-canvas, #fbfcfa);
	}
	header p,
	h2 {
		margin: 0;
	}
	header p {
		margin-bottom: 0.35rem;
		color: var(--theme-accent, #2d6540);
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.1em;
	}
	h2 {
		font-size: 1.35rem;
		letter-spacing: -0.025em;
	}
	.close {
		width: 2.25rem;
		height: 2.25rem;
		border: 0;
		border-radius: 50%;
		background: transparent;
		font-size: 1.5rem;
		cursor: pointer;
	}
	.close:hover {
		background: var(--theme-hover, #e6ece4);
	}
	form {
		display: grid;
		gap: 1.1rem;
		padding: 1.35rem;
	}
	label {
		display: grid;
		gap: 0.4rem;
		color: #4b4b47;
		font-size: 0.75rem;
		font-weight: 750;
	}
	input,
	select {
		width: 100%;
		min-height: 2.7rem;
		box-sizing: border-box;
		padding: 0 0.75rem;
		border: 1px solid var(--theme-border, #dfe5dc);
		border-radius: 0.55rem;
		background: var(--theme-canvas, #fbfcfa);
		color: #292927;
		font: inherit;
		font-size: 0.88rem;
	}
	.title-field input {
		min-height: 3.25rem;
		font-size: 1.05rem;
	}
	input:focus,
	select:focus,
	button:focus-visible {
		outline: 3px solid var(--theme-focus, rgb(45 101 64 / 16%));
		border-color: var(--theme-accent, #2d6540);
	}
	.field-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.85rem;
	}
	.error {
		margin: 0;
		color: #b83f34;
		font-size: 0.82rem;
	}
	footer {
		padding-top: 0.3rem;
	}
	footer span {
		color: #777772;
		font-size: 0.72rem;
	}
	footer button {
		padding: 0.75rem 1rem;
		border: 0;
		border-radius: 0.55rem;
		background: var(--theme-accent, #2d6540);
		color: #fff;
		font-weight: 750;
		cursor: pointer;
	}
	button:disabled,
	select:disabled,
	input:disabled {
		cursor: not-allowed;
		opacity: 0.55;
	}
	@media (max-width: 36rem) {
		.backdrop {
			place-items: end center;
			padding: 0;
		}
		.dialog {
			border-radius: 1rem 1rem 0 0;
		}
		.field-grid {
			grid-template-columns: 1fr;
		}
		footer {
			align-items: stretch;
			flex-direction: column;
		}
	}
</style>
