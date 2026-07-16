<script lang="ts">
	import { TaskConflictError, type Task, type TaskUpdate } from '$lib/tasks/client';

	interface Props {
		task: Task;
		save: (update: TaskUpdate) => Promise<void>;
		cancel: () => void;
	}

	let { task, save, cancel }: Props = $props();
	let title = $derived(task.title);
	let description = $derived(task.description ?? '');
	let priority = $derived(task.priority);
	let dueAt = $derived(localDateTime(task.dueAt));
	let saving = $state(false);
	let errorMessage = $state('');

	async function submit() {
		saving = true;
		errorMessage = '';
		try {
			await save({
				version: task.version,
				title,
				description: description.trim() || null,
				priority,
				dueAt: dueAt ? new Date(dueAt).toISOString() : null,
				dueTimezone: dueAt ? Intl.DateTimeFormat().resolvedOptions().timeZone : null
			});
		} catch (error) {
			errorMessage =
				error instanceof TaskConflictError
					? 'This task changed elsewhere. Reload it before saving again.'
					: 'The task could not be saved. Please try again.';
			saving = false;
		}
	}

	function localDateTime(value: string | null): string {
		if (!value) return '';

		const date = new Date(value);
		const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
		return local.toISOString().slice(0, 16);
	}
</script>

<form
	class="editor"
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

	<div class="fields">
		<label>
			<span>Priority</span>
			<select bind:value={priority}>
				<option value={0}>None</option>
				<option value={1}>Low</option>
				<option value={2}>Medium</option>
				<option value={3}>High</option>
				<option value={4}>Urgent</option>
			</select>
		</label>

		<label>
			<span>Due date</span>
			<input type="datetime-local" bind:value={dueAt} />
		</label>
	</div>

	{#if errorMessage}
		<p class="error" role="alert">{errorMessage}</p>
	{/if}

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
		width: 100%;
		gap: 1rem;
		padding: 1.25rem;
		border: 1px solid #cad8c9;
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 0.75rem 2.5rem rgb(24 56 34 / 7%);
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
	textarea,
	select {
		width: 100%;
		padding: 0.7rem 0.75rem;
		border: 1px solid #ccd6ca;
		border-radius: 0.65rem;
		color: #17211a;
		background: #fff;
		outline: none;
	}

	input:focus,
	textarea:focus,
	select:focus {
		border-color: #477d56;
		box-shadow: 0 0 0 0.2rem rgb(71 125 86 / 12%);
	}

	textarea {
		resize: vertical;
	}

	.fields {
		display: grid;
		grid-template-columns: minmax(8rem, 0.7fr) minmax(12rem, 1.3fr);
		gap: 0.85rem;
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
		border: 1px solid #ccd6ca;
		color: #4f5d53;
		background: #fff;
	}

	.save {
		border: 1px solid #2d6540;
		color: #fff;
		background: #2d6540;
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

	@media (max-width: 34rem) {
		.fields {
			grid-template-columns: 1fr;
		}
	}
</style>
