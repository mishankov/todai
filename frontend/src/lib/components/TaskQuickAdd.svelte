<script module lang="ts">
	let nextTaskQuickAddId = 0;
	function createTaskQuickAddId(): number {
		nextTaskQuickAddId += 1;
		return nextTaskQuickAddId;
	}
</script>

<script lang="ts">
	import type { Task, TaskCreateDraft } from '$lib/tasks/client';

	interface Props {
		create: (draft: TaskCreateDraft) => Promise<Task>;
		oncreated: (task: Task) => void;
		initialProjectId: string;
		initialSectionId?: string | null;
		label?: string;
	}

	let {
		create,
		oncreated,
		initialProjectId,
		initialSectionId = null,
		label = 'Task title'
	}: Props = $props();
	let title = $state('');
	let creating = $state(false);
	let errorMessage = $state('');
	const inputId = `task-quick-add-${createTaskQuickAddId()}`;

	async function submit() {
		const trimmedTitle = title.trim();
		if (!trimmedTitle || creating) return;
		creating = true;
		errorMessage = '';
		try {
			const created = await create({
				title: trimmedTitle,
				projectId: initialProjectId,
				sectionId: initialSectionId,
				priority: 0,
				dueDate: null,
				dueTime: null,
				dueTimezone: null
			});
			oncreated(created);
			title = '';
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
		<label class="sr-only" for={inputId}>{label}</label>
		<input
			id={inputId}
			name="title"
			placeholder="Add task"
			autocomplete="off"
			maxlength="500"
			bind:value={title}
			disabled={creating}
		/>
		<button
			type="submit"
			aria-label={label.startsWith('Add task') ? label : 'Add task'}
			disabled={creating || !title.trim()}
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
	.title-row input {
		min-width: 0;
		padding: 0.55rem 0.2rem;
		border: 0;
		color: #292927;
		background: transparent;
		outline: none;
	}
	.title-row input::placeholder {
		color: #85847f;
	}
	.title-row input:focus::placeholder {
		color: var(--theme-accent, #2d6540);
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
</style>
