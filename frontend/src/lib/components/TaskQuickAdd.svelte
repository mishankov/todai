<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import type { Task, TaskCreateDraft } from '$lib/tasks/client';
	import { cleanTaskTitle } from '$lib/tasks/rich-title';
	import { untrack } from 'svelte';
	import RichTaskTitle from './RichTaskTitle.svelte';

	interface Props {
		create: (draft: TaskCreateDraft) => Promise<Task>;
		oncreated: (task: Task) => void;
		initialProjectId: string;
		initialSectionId?: string | null;
		label?: string;
		projects?: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
	}

	let {
		create,
		oncreated,
		initialProjectId,
		initialSectionId = null,
		label = 'Task title',
		projects = [],
		sections = [],
		loadSections
	}: Props = $props();
	let title = $state('');
	let projectId = $state(untrack(() => initialProjectId));
	let sectionId = $state<string | null>(untrack(() => initialSectionId));
	let priority = $state(0);
	let dueDate = $state<string | null>(null);
	let dueTime = $state<string | null>(null);
	let dueTimezone = $state<string | null>(null);
	let creating = $state(false);
	let errorMessage = $state('');

	async function submit() {
		const trimmedTitle = cleanTaskTitle(title);
		if (!trimmedTitle || !projectId || creating) return;
		creating = true;
		errorMessage = '';
		try {
			const created = await create({
				title: trimmedTitle,
				projectId,
				sectionId,
				priority,
				dueDate,
				dueTime,
				dueTimezone
			});
			oncreated(created);
			title = '';
			projectId = initialProjectId;
			sectionId = initialSectionId;
			priority = 0;
			dueDate = null;
			dueTime = null;
			dueTimezone = null;
		} catch {
			errorMessage = 'The task could not be created. Please try again.';
		} finally {
			creating = false;
		}
	}
</script>

<form
	class="task-quick-add"
	onsubmit={(event) => {
		event.preventDefault();
		void submit();
	}}
>
	<div class="title-row">
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
			{loadSections}
			{label}
			disabled={creating}
		/>
		<button
			type="submit"
			aria-label={label.startsWith('Add task') ? label : 'Add task'}
			disabled={creating || !cleanTaskTitle(title) || !projectId}
		>
			{creating ? 'Adding…' : 'Add'}
		</button>
	</div>

	{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}
</form>

<style>
	.task-quick-add {
		display: grid;
		gap: 0.45rem;
		padding: 0.25rem 0 0.7rem;
		border-bottom: 1px solid #e7e7e4;
	}
	.title-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.55rem;
	}
	.title-row button {
		padding: 0.5rem 0.75rem;
		border: 0;
		border-radius: 0.35rem;
		color: #fff;
		background: var(--theme-accent, #2d6540);
		font-size: 0.76rem;
		font-weight: 720;
		cursor: pointer;
	}
	.title-row button:disabled {
		color: #aaa7a1;
		background: #efefec;
		cursor: default;
	}
	.error {
		margin: 0.25rem 0 0;
		color: #b83f34;
		font-size: 0.8rem;
	}
</style>
