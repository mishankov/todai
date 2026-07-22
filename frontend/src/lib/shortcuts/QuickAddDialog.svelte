<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import type { Task, TaskCreateDraft } from '$lib/tasks/client';
	import { cleanTaskTitle } from '$lib/tasks/rich-title';
	import RichTaskTitle from '$lib/components/RichTaskTitle.svelte';
	import TaskPropertyPickers from '$lib/components/TaskPropertyPickers.svelte';
	import { untrack } from 'svelte';

	interface Props {
		projects: Project[];
		initialProjectId: string;
		initialSectionId?: string | null;
		shortcutLabel: string;
		focusRequest: number;
		loadSections: (projectId: string) => Promise<ProjectSection[]>;
		createTask: (draft: TaskCreateDraft) => Promise<Task>;
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
		close,
		saved
	}: Props = $props();
	let title = $state('');
	let projectId = $state(untrack(() => initialProjectId));
	let sectionId = $state<string | null>(untrack(() => initialSectionId));
	let sections = $state<ProjectSection[]>([]);
	let priority = $state(0);
	let dueDate = $state<string | null>(null);
	let dueTime = $state<string | null>(null);
	let dueTimezone = $state<string | null>(null);
	let saving = $state(false);
	let errorMessage = $state('');
	let dialog: HTMLDivElement;

	async function loadAndCacheSections(nextProjectId: string) {
		try {
			const loaded = await loadSections(nextProjectId);
			sections = [...sections.filter((section) => section.projectId !== nextProjectId), ...loaded];
			return loaded;
		} catch {
			errorMessage = 'Sections could not be loaded. You can still create the task in Inbox.';
			return [];
		}
	}

	async function save() {
		const cleanTitle = cleanTaskTitle(title);
		if (!cleanTitle || !projectId || saving) return;
		saving = true;
		errorMessage = '';
		try {
			const task = await createTask({
				title: cleanTitle,
				projectId,
				sectionId,
				priority,
				dueDate,
				dueTime: dueDate ? dueTime : null,
				dueTimezone: dueDate && dueTime ? dueTimezone : null
			});
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
			<div class="title-field">
				<span>Title</span>
				<RichTaskTitle
					bind:title
					bind:projectId
					bind:sectionId
					bind:priority
					bind:dueDate
					bind:dueTime
					bind:dueTimezone
					{projects}
					{sections}
					loadSections={loadAndCacheSections}
					label="Title"
					placeholder="Task title"
					disabled={saving}
					{focusRequest}
				/>
			</div>

			<TaskPropertyPickers
				bind:projectId
				bind:sectionId
				bind:priority
				bind:dueDate
				bind:dueTime
				bind:dueTimezone
				{projects}
				{sections}
				loadSections={loadAndCacheSections}
			/>

			{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}

			<footer>
				<span>Enter to save · Shift+Enter does not save</span>
				<button type="submit" disabled={saving || !cleanTaskTitle(title) || !projectId}>
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
	.title-field {
		display: grid;
		gap: 0.4rem;
		color: #4b4b47;
		font-size: 0.75rem;
		font-weight: 750;
	}
	button:focus-visible {
		outline: 3px solid var(--theme-focus, rgb(45 101 64 / 16%));
		border-color: var(--theme-accent, #2d6540);
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
	button:disabled {
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
		footer {
			align-items: stretch;
			flex-direction: column;
		}
	}
</style>
