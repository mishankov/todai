<script lang="ts">
	import { TaskConflictError, type Task, type TaskUpdate } from '$lib/tasks/client';
	import type { Project, ProjectSection } from '$lib/projects/client';

	interface Props {
		task: Task;
		save: (update: TaskUpdate) => Promise<void>;
		cancel: () => void;
		projects?: Project[];
		sections?: ProjectSection[];
		currentProjectId?: string;
		variant?: 'card' | 'dialog';
	}

	let {
		task,
		save,
		cancel,
		projects = [],
		sections,
		currentProjectId,
		variant = 'card'
	}: Props = $props();
	let title = $derived(task.title);
	let description = $derived(task.description ?? '');
	let priority = $derived(task.priority);
	let projectId = $derived(task.projectId ?? '');
	let sectionId = $derived(task.sectionId ?? '');
	let dueDate = $derived(task.dueDate ?? '');
	let dueTime = $derived(task.dueTime ?? '');
	let locationOpen = $state(false);
	let showDueTime = $derived(Boolean(task.dueTime));
	let saving = $state(false);
	let errorMessage = $state('');
	let projectName = $derived(projects.find((project) => project.id === projectId)?.name ?? 'Inbox');
	let sectionName = $derived(
		sections?.find((section) => section.id === sectionId)?.name ?? 'No section'
	);
	let locationName = $derived(
		sections !== undefined && projectId === currentProjectId
			? `${projectName} / ${sectionName}`
			: projectName
	);

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
				dueTimezone: dueDate && dueTime ? Intl.DateTimeFormat().resolvedOptions().timeZone : null
			};
			if (projectId !== (task.projectId ?? '')) {
				update.projectId = projectId || null;
			}
			if (sections !== undefined && projectId === currentProjectId) {
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

	{#if variant === 'dialog'}
		<div class="property-bar" role="group" aria-label="Task properties">
			<div
				class="property-location"
				onfocusout={(event) => {
					if (!event.currentTarget.contains(event.relatedTarget as Node | null))
						locationOpen = false;
				}}
			>
				<button
					class="property-trigger"
					type="button"
					aria-label={`Location: ${locationName}`}
					aria-expanded={locationOpen}
					onclick={() => (locationOpen = !locationOpen)}
				>
					<svg aria-hidden="true" viewBox="0 0 24 24">
						<path d="M3.5 7.5h6l1.7 2h9.3v9h-17zM3.5 7.5V5h6l1.7 2.5" />
					</svg>
					<span>{locationName}</span>
					<svg class="chevron" aria-hidden="true" viewBox="0 0 24 24">
						<path d="m8 10 4 4 4-4" />
					</svg>
				</button>

				{#if locationOpen}
					<div class="location-popover" role="group" aria-label="Task location">
						<label>
							<span>Project</span>
							<select aria-label="Project" bind:value={projectId}>
								<option value="">Inbox</option>
								{#each projects as project (project.id)}
									<option value={project.id}>{project.name}</option>
								{/each}
							</select>
						</label>

						{#if sections !== undefined && projectId === currentProjectId}
							<label>
								<span>Section</span>
								<select aria-label="Section" bind:value={sectionId}>
									<option value="">No section</option>
									{#each sections as section (section.id)}
										<option value={section.id}>{section.name}</option>
									{/each}
								</select>
							</label>
						{/if}
					</div>
				{/if}
			</div>

			<label class="property-segment priority-property">
				<svg aria-hidden="true" viewBox="0 0 24 24">
					<path d="M5 21V4m0 1h10l-1.5 3L15 11H5" />
				</svg>
				<span class="visually-hidden">Priority</span>
				<span class="property-prefix" aria-hidden="true">Priority:</span>
				<select aria-label="Priority" bind:value={priority}>
					<option value={0}>None</option>
					<option value={1}>Low</option>
					<option value={2}>Medium</option>
					<option value={3}>High</option>
					<option value={4}>Urgent</option>
				</select>
			</label>

			<label class="property-segment date-property">
				<span class="visually-hidden">Due date</span>
				<input aria-label="Due date" type="date" bind:value={dueDate} />
			</label>

			{#if showDueTime}
				<div class="property-segment time-property">
					<label>
						<span class="visually-hidden">Due time</span>
						<input aria-label="Due time" type="time" bind:value={dueTime} disabled={!dueDate} />
					</label>
					<button
						class="clear-time"
						type="button"
						aria-label="Remove due time"
						onclick={() => {
							dueTime = '';
							showDueTime = false;
						}}>×</button
					>
				</div>
			{:else}
				<button
					class="property-segment add-time"
					type="button"
					title={dueDate ? 'Add a due time' : 'Choose a due date first'}
					disabled={!dueDate}
					onclick={() => (showDueTime = true)}
				>
					<svg aria-hidden="true" viewBox="0 0 24 24">
						<circle cx="12" cy="12" r="8.5" />
						<path d="M12 7.5V12l3 2" />
					</svg>
					<span>+ Time</span>
				</button>
			{/if}
		</div>
	{:else}
		<div class="fields">
			<label>
				<span>Project</span>
				<select bind:value={projectId}>
					<option value="">Inbox</option>
					{#each projects as project (project.id)}
						<option value={project.id}>{project.name}</option>
					{/each}
				</select>
			</label>

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

			{#if sections !== undefined && projectId === currentProjectId}
				<label>
					<span>Section</span>
					<select bind:value={sectionId}>
						<option value="">No section</option>
						{#each sections as section (section.id)}
							<option value={section.id}>{section.name}</option>
						{/each}
					</select>
				</label>
			{/if}

			<label>
				<span>Due date</span>
				<input type="date" bind:value={dueDate} />
			</label>

			<label>
				<span>Due time <small>optional</small></span>
				<input type="time" bind:value={dueTime} disabled={!dueDate} />
			</label>
		</div>
	{/if}

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

	label small {
		color: #899087;
		font-size: inherit;
		font-weight: 500;
	}

	input,
	textarea,
	select {
		box-sizing: border-box;
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

	.property-bar {
		display: grid;
		grid-template-columns: minmax(0, 1.45fr) minmax(9rem, 1fr) minmax(9.5rem, 1fr) minmax(
				6.75rem,
				0.7fr
			);
		min-width: 0;
		min-height: 2.85rem;
		border: 1px solid #ccd6ca;
		border-radius: 0.75rem;
		background: #fff;
	}

	.property-location {
		position: relative;
		min-width: 0;
	}

	.property-trigger,
	.property-segment {
		display: flex;
		min-width: 0;
		min-height: 2.75rem;
		align-items: center;
		gap: 0.48rem;
		padding: 0.45rem 0.68rem;
		border: 0;
		border-left: 1px solid #dce4da;
		border-radius: 0;
		color: #29332c;
		background: transparent;
		font-size: 0.78rem;
		font-weight: 600;
	}

	.property-trigger {
		width: 100%;
		border-left: 0;
		cursor: pointer;
		text-align: left;
	}

	.property-trigger > span {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.property-trigger svg,
	.property-segment > svg {
		width: 1rem;
		height: 1rem;
		flex: none;
		fill: none;
		stroke: #657269;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.7;
	}

	.property-trigger .chevron {
		width: 0.85rem;
		height: 0.85rem;
	}

	.property-trigger:hover,
	.property-trigger:focus-visible,
	.property-segment:focus-within,
	.add-time:hover:not(:disabled),
	.add-time:focus-visible {
		background: #f3f7f2;
		outline: none;
	}

	.property-segment select,
	.property-segment input {
		min-width: 0;
		padding: 0;
		border: 0;
		border-radius: 0;
		background-color: transparent;
		box-shadow: none;
		font-size: inherit;
		font-weight: inherit;
	}

	.property-segment select:focus,
	.property-segment input:focus {
		border: 0;
		box-shadow: none;
	}

	.priority-property select {
		width: auto;
		flex: 1;
	}

	.property-prefix {
		flex: none;
		color: #657269;
		font-size: 0.72rem;
		font-weight: 600;
	}

	.date-property input,
	.time-property input {
		width: 100%;
	}

	.time-property label {
		min-width: 0;
		flex: 1;
	}

	.add-time {
		cursor: pointer;
	}

	.add-time:disabled {
		cursor: not-allowed;
	}

	.clear-time {
		display: grid;
		width: 1.25rem;
		height: 1.25rem;
		flex: none;
		place-items: center;
		padding: 0;
		border: 0;
		border-radius: 50%;
		color: #737d75;
		background: transparent;
		font-size: 1rem;
		font-weight: 500;
		line-height: 1;
	}

	.clear-time:hover,
	.clear-time:focus-visible {
		color: #8c2828;
		background: #f8ecea;
		outline: none;
	}

	.location-popover {
		position: absolute;
		z-index: 5;
		top: calc(100% + 0.45rem);
		left: 0;
		display: grid;
		width: min(19rem, calc(100vw - 3rem));
		gap: 0.7rem;
		padding: 0.8rem;
		border: 1px solid #ccd6ca;
		border-radius: 0.75rem;
		background: #fff;
		box-shadow: 0 0.8rem 2.2rem rgb(28 52 34 / 14%);
	}

	.location-popover label {
		gap: 0.3rem;
	}

	.location-popover select {
		padding: 0.55rem 0.62rem;
	}

	.visually-hidden {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
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

	@media (max-width: 46rem) {
		.property-bar {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
			gap: 0.45rem;
			border: 0;
			background: transparent;
		}

		.property-location,
		.property-segment {
			border: 1px solid #ccd6ca;
			border-radius: 0.7rem;
			background: #fff;
		}

		.property-trigger {
			border-left: 0;
		}
	}

	@media (max-width: 28rem) {
		.property-bar {
			grid-template-columns: 1fr;
		}
	}
</style>
