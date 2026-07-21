<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { TaskConflictError, type Task, type TaskUpdate } from '$lib/tasks/client';
	import TaskPropertyPickers from './TaskPropertyPickers.svelte';

	interface Props {
		task: Task;
		save: (update: TaskUpdate) => Promise<void>;
		cancel: () => void;
		projects?: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		variant?: 'card' | 'dialog';
	}

	let {
		task,
		save,
		cancel,
		projects = [],
		sections,
		loadSections,
		variant = 'card'
	}: Props = $props();
	let title = $derived(task.title);
	let description = $derived(task.description ?? '');
	let priority = $derived(task.priority);
	let projectId = $derived(task.projectId);
	let sectionId = $derived(task.sectionId ?? '');
	let dueDate = $derived(task.dueDate ?? '');
	let dueTime = $derived(task.dueTime ?? '');
	let dueTimezone = $derived(task.dueTimezone ?? '');
	let saving = $state(false);
	let errorMessage = $state('');

	async function submit() {
		saving = true;
		errorMessage = '';
		try {
			const update: TaskUpdate = {
				version: task.version,
				title,
				description: description.trim() || null,
				priority,
				dueDate: dueDate || null,
				dueTime: dueDate && dueTime ? dueTime : null,
				dueTimezone: dueDate && dueTime ? dueTimezone || null : null
			};
			if (projectId !== task.projectId) update.projectId = projectId;
			if (sectionId !== (task.sectionId ?? '') || projectId !== task.projectId) {
				update.sectionId = sectionId || null;
			}
			await save(update);
		} catch (error) {
			errorMessage =
				error instanceof TaskConflictError
					? 'This task changed elsewhere. Reload it before saving again.'
					: 'The task could not be saved. Please try again.';
			saving = false;
		}
	}
</script>

<form
	class="editor"
	class:dialog={variant === 'dialog'}
	onsubmit={(event) => {
		event.preventDefault();
		void submit();
	}}
>
	<label>
		<span>Title</span>
		<input bind:value={title} maxlength="500" required />
	</label>

	<label>
		<span>Description</span>
		<textarea bind:value={description} maxlength="10000" rows="3"></textarea>
	</label>

	<TaskPropertyPickers
		bind:projectId
		bind:sectionId
		bind:priority
		bind:dueDate
		bind:dueTime
		bind:dueTimezone
		{projects}
		{sections}
		{loadSections}
	/>

	{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}

	<div class="actions">
		<button class="cancel" type="button" disabled={saving} onclick={cancel}>Cancel</button>
		<button class="save" type="submit" disabled={saving || !title.trim()}>
			{saving ? 'Saving…' : 'Save changes'}
		</button>
	</div>
</form>

<style>
	.editor {
		display: grid;
		container-type: inline-size;
		box-sizing: border-box;
		width: 100%;
		gap: 1rem;
		padding: 1.25rem;
		border: 1px solid var(--theme-border, #cad8c9);
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 0.75rem 2.5rem color-mix(in srgb, var(--theme-accent, #2d6540) 7%, transparent);
	}
	.editor.dialog {
		padding: 0;
		border: 0;
		border-radius: 0;
		background: transparent;
		box-shadow: none;
	}
	label {
		display: grid;
		gap: 0.4rem;
	}
	label span {
		color: #526058;
		font-size: 0.75rem;
		font-weight: 700;
	}
	input,
	textarea {
		box-sizing: border-box;
		width: 100%;
		padding: 0.7rem 0.75rem;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.65rem;
		color: #17211a;
		background: #fff;
		outline: none;
	}
	input:focus,
	textarea:focus {
		border-color: var(--theme-accent, #477d56);
		box-shadow: 0 0 0 0.2rem var(--theme-focus, rgb(71 125 86 / 12%));
	}
	textarea {
		resize: vertical;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.65rem;
	}
	button {
		padding: 0.65rem 0.8rem;
		border-radius: 0.6rem;
		font-size: 0.8rem;
		font-weight: 700;
		cursor: pointer;
	}
	.cancel {
		border: 1px solid var(--theme-border, #ccd6ca);
		color: #4f5d53;
		background: #fff;
	}
	.save {
		border: 1px solid var(--theme-accent, #2d6540);
		color: #fff;
		background: var(--theme-accent, #2d6540);
	}
	button:disabled {
		cursor: wait;
		opacity: 0.5;
	}
	.error {
		margin: 0;
		color: #8c2828;
		font-size: 0.82rem;
	}
</style>
