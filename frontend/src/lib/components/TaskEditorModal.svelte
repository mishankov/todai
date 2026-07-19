<script lang="ts">
	import type { ActivityEvent } from '$lib/activity/client';
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { subscribeActivityEvents } from '$lib/realtime/events';
	import {
		completeTask as requestCompleteTask,
		createTaskComment as requestCreateComment,
		createTask as requestCreateTask,
		deleteTaskComment as requestDeleteComment,
		getTaskComments as requestComments,
		getTaskSubtasks as requestSubtasks,
		reopenTask as requestReopenTask,
		type Task,
		type TaskComment,
		type TaskUpdate
	} from '$lib/tasks/client';
	import { onMount } from 'svelte';
	import TaskEditor from './TaskEditor.svelte';

	interface Props {
		task: Task;
		projects: Project[];
		sections?: ProjectSection[];
		currentProjectId?: string;
		save: (update: TaskUpdate) => Promise<void>;
		close: () => void;
		loadSubtasks?: (taskId: string) => Promise<Task[]>;
		addSubtask?: (title: string) => Promise<Task>;
		completeSubtask?: (taskId: string, version: number) => Promise<Task>;
		reopenSubtask?: (taskId: string, version: number) => Promise<Task>;
		loadComments?: (taskId: string) => Promise<TaskComment[]>;
		addComment?: (body: string) => Promise<TaskComment>;
		removeComment?: (commentId: string, version: number) => Promise<void>;
	}

	let {
		task,
		projects,
		sections,
		currentProjectId,
		save,
		close,
		loadSubtasks = (taskId) => requestSubtasks(fetch, taskId),
		addSubtask = (title) => requestCreateTask(fetch, title, undefined, undefined, task.id),
		completeSubtask = (taskId, version) => requestCompleteTask(fetch, taskId, version),
		reopenSubtask = (taskId, version) => requestReopenTask(fetch, taskId, version),
		loadComments = (taskId) => requestComments(fetch, taskId),
		addComment = (body) => requestCreateComment(fetch, task.id, body),
		removeComment = (commentId, version) => requestDeleteComment(fetch, task.id, commentId, version)
	}: Props = $props();

	let dialog: HTMLElement;
	let subtasks = $state<Task[]>([]);
	let comments = $state<TaskComment[]>([]);
	let newSubtaskTitle = $state('');
	let newCommentBody = $state('');
	let loadingSubtasks = $state(true);
	let loadingComments = $state(true);
	let addingSubtask = $state(false);
	let addingComment = $state(false);
	let busySubtaskIds = $state<string[]>([]);
	let deletingCommentIds = $state<string[]>([]);
	let subtaskError = $state('');
	let commentError = $state('');
	let reloadTimer: number | undefined;
	let subtaskLoadVersion = 0;
	let commentLoadVersion = 0;
	let completedSubtasks = $derived(subtasks.filter((item) => item.status === 'completed').length);

	onMount(() => {
		const previousOverflow = document.documentElement.style.overflow;
		const previouslyFocused =
			document.activeElement instanceof HTMLElement ? document.activeElement : null;
		document.documentElement.style.overflow = 'hidden';
		dialog.querySelector<HTMLInputElement>('input')?.focus();
		void refreshRelatedData(true);
		const unsubscribe = subscribeActivityEvents(handleActivity);

		return () => {
			unsubscribe();
			if (reloadTimer !== undefined) window.clearTimeout(reloadTimer);
			document.documentElement.style.overflow = previousOverflow;
			queueMicrotask(() => {
				if (previouslyFocused?.isConnected) previouslyFocused.focus();
			});
		};
	});

	async function refreshRelatedData(showLoading = false) {
		await Promise.all([refreshSubtasks(showLoading), refreshComments(showLoading)]);
	}

	async function refreshSubtasks(showLoading: boolean) {
		const loadVersion = ++subtaskLoadVersion;
		if (showLoading) loadingSubtasks = true;
		subtaskError = '';
		try {
			const loaded = await loadSubtasks(task.id);
			if (loadVersion === subtaskLoadVersion) subtasks = loaded;
		} catch {
			if (loadVersion === subtaskLoadVersion) subtaskError = 'Subtasks could not be loaded.';
		} finally {
			if (loadVersion === subtaskLoadVersion) loadingSubtasks = false;
		}
	}

	async function refreshComments(showLoading: boolean) {
		const loadVersion = ++commentLoadVersion;
		if (showLoading) loadingComments = true;
		commentError = '';
		try {
			const loaded = await loadComments(task.id);
			if (loadVersion === commentLoadVersion) comments = loaded;
		} catch {
			if (loadVersion === commentLoadVersion) commentError = 'Comments could not be loaded.';
		} finally {
			if (loadVersion === commentLoadVersion) loadingComments = false;
		}
	}

	function handleActivity(event: ActivityEvent) {
		if (!event.type.startsWith('task.') || !eventAffectsTask(event)) return;
		if (reloadTimer !== undefined) window.clearTimeout(reloadTimer);
		reloadTimer = window.setTimeout(() => void refreshRelatedData(), 80);
	}

	function eventAffectsTask(event: ActivityEvent): boolean {
		if (event.aggregateId === task.id) return true;
		const payload = event.payload;
		if (payload.taskId === task.id || payload.parentId === task.id) return true;
		for (const key of ['task', 'before', 'after', 'comment']) {
			const value = asRecord(payload[key]);
			if (value?.taskId === task.id || value?.parentId === task.id || value?.id === task.id) {
				return true;
			}
		}
		return false;
	}

	function asRecord(value: unknown): Record<string, unknown> | null {
		return typeof value === 'object' && value !== null && !Array.isArray(value)
			? (value as Record<string, unknown>)
			: null;
	}

	async function submitSubtask() {
		const title = newSubtaskTitle.trim();
		if (!title || addingSubtask) return;
		addingSubtask = true;
		subtaskError = '';
		try {
			const created = await addSubtask(title);
			subtasks = [...subtasks.filter((item) => item.id !== created.id), created];
			newSubtaskTitle = '';
		} catch {
			subtaskError = 'The subtask could not be added.';
		} finally {
			addingSubtask = false;
		}
	}

	async function toggleSubtask(item: Task) {
		if (busySubtaskIds.includes(item.id)) return;
		busySubtaskIds = [...busySubtaskIds, item.id];
		subtaskError = '';
		try {
			const updated =
				item.status === 'completed'
					? await reopenSubtask(item.id, item.version)
					: await completeSubtask(item.id, item.version);
			subtasks = subtasks.map((candidate) => (candidate.id === updated.id ? updated : candidate));
		} catch {
			subtaskError = 'The subtask could not be updated.';
		} finally {
			busySubtaskIds = busySubtaskIds.filter((id) => id !== item.id);
		}
	}

	async function submitComment() {
		const body = newCommentBody.trim();
		if (!body || addingComment) return;
		addingComment = true;
		commentError = '';
		try {
			const created = await addComment(body);
			comments = [...comments.filter((item) => item.id !== created.id), created];
			newCommentBody = '';
		} catch {
			commentError = 'The comment could not be added.';
		} finally {
			addingComment = false;
		}
	}

	async function deleteComment(item: TaskComment) {
		if (deletingCommentIds.includes(item.id)) return;
		deletingCommentIds = [...deletingCommentIds, item.id];
		commentError = '';
		try {
			await removeComment(item.id, item.version);
			comments = comments.filter((candidate) => candidate.id !== item.id);
		} catch {
			commentError = 'The comment could not be deleted.';
		} finally {
			deletingCommentIds = deletingCommentIds.filter((id) => id !== item.id);
		}
	}

	function formatTimestamp(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return 'Unknown time';
		return new Intl.DateTimeFormat(undefined, {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		}).format(date);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			close();
			return;
		}
		if (event.key !== 'Tab') return;

		const focusable = Array.from(
			dialog.querySelectorAll<HTMLElement>(
				'button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])'
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

<svelte:window onkeydown={handleKeydown} />

<div
	class="modal-backdrop"
	role="presentation"
	onclick={(event) => {
		if (event.target === event.currentTarget) close();
	}}
>
	<div
		bind:this={dialog}
		class="task-editor-modal"
		role="dialog"
		aria-modal="true"
		aria-label={`Edit task: ${task.title}`}
	>
		<header class="modal-header">
			<div>
				<p>Task details</p>
				<h2>{task.title}</h2>
			</div>
			<button class="close" type="button" aria-label="Close task editor" onclick={close}>×</button>
		</header>

		<div class="modal-content">
			<div class="details-column">
				<TaskEditor
					{task}
					{projects}
					{sections}
					{currentProjectId}
					{save}
					cancel={close}
					variant="dialog"
				/>

				<section class="subtasks" aria-labelledby="subtasks-heading" aria-busy={loadingSubtasks}>
					<div class="section-heading">
						<div>
							<h3 id="subtasks-heading">Subtasks</h3>
							<span>{completedSubtasks} of {subtasks.length} complete</span>
						</div>
						<progress
							value={completedSubtasks}
							max={Math.max(subtasks.length, 1)}
							aria-label={`${completedSubtasks} of ${subtasks.length} subtasks complete`}
						></progress>
					</div>

					{#if loadingSubtasks}
						<p class="loading" role="status">Loading subtasks…</p>
					{:else if subtasks.length === 0}
						<p class="empty">No subtasks yet.</p>
					{:else}
						<ul class="subtask-list">
							{#each subtasks as item (item.id)}
								<li class:completed={item.status === 'completed'}>
									<button
										class="status-toggle"
										type="button"
										disabled={busySubtaskIds.includes(item.id)}
										aria-label={`${item.status === 'completed' ? 'Reopen' : 'Complete'} ${item.title}`}
										onclick={() => void toggleSubtask(item)}
									>
										<span aria-hidden="true">✓</span>
									</button>
									<span>{item.title}</span>
								</li>
							{/each}
						</ul>
					{/if}

					<form
						class="inline-composer"
						onsubmit={(event) => {
							event.preventDefault();
							void submitSubtask();
						}}
					>
						<label for="new-subtask">Add a subtask</label>
						<div>
							<input
								id="new-subtask"
								bind:value={newSubtaskTitle}
								maxlength="500"
								placeholder="Add a subtask"
							/>
							<button type="submit" disabled={addingSubtask || !newSubtaskTitle.trim()}>
								{addingSubtask ? 'Adding…' : 'Add subtask'}
							</button>
						</div>
					</form>
					{#if subtaskError}<p class="error" role="alert">{subtaskError}</p>{/if}
				</section>
			</div>

			<aside class="comments-column" aria-labelledby="comments-heading" aria-busy={loadingComments}>
				<div class="comments-heading">
					<div>
						<p>Discussion</p>
						<h3 id="comments-heading">Comments</h3>
					</div>
					<span>{comments.length}</span>
				</div>

				<div class="comments-thread">
					{#if loadingComments}
						<p class="loading" role="status">Loading comments…</p>
					{:else if comments.length === 0}
						<div class="comments-empty">
							<p>No comments yet</p>
							<span>Use comments to keep context with the task.</span>
						</div>
					{:else}
						<ol>
							{#each comments as item (item.id)}
								<li>
									<div class="comment-meta">
										<div>
											<strong>You</strong>
											<time datetime={item.createdAt}>{formatTimestamp(item.createdAt)}</time>
										</div>
										<button
											type="button"
											disabled={deletingCommentIds.includes(item.id)}
											aria-label="Delete comment"
											onclick={() => void deleteComment(item)}
										>
											{deletingCommentIds.includes(item.id) ? 'Deleting…' : 'Delete'}
										</button>
									</div>
									<p>{item.body}</p>
								</li>
							{/each}
						</ol>
					{/if}
				</div>

				<form
					class="comment-composer"
					onsubmit={(event) => {
						event.preventDefault();
						void submitComment();
					}}
				>
					<label for="new-comment">Add a comment</label>
					<textarea
						id="new-comment"
						bind:value={newCommentBody}
						maxlength="10000"
						rows="3"
						placeholder="Add context or a note…"></textarea>
					<div>
						{#if commentError}<p class="error" role="alert">{commentError}</p>{/if}
						<button type="submit" disabled={addingComment || !newCommentBody.trim()}>
							{addingComment ? 'Sending…' : 'Send comment'}
						</button>
					</div>
				</form>
			</aside>
		</div>
	</div>
</div>

<style>
	.modal-backdrop {
		position: fixed;
		z-index: 100;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 1.25rem;
		background: rgb(24 34 27 / 38%);
		backdrop-filter: blur(2px);
	}

	.task-editor-modal {
		display: flex;
		width: min(70rem, 100%);
		max-height: calc(100dvh - 2.5rem);
		flex-direction: column;
		overflow: hidden;
		border: 1px solid #d5dfd3;
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 1.5rem 4rem rgb(18 42 26 / 22%);
	}

	.modal-header {
		display: flex;
		flex: none;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		padding: 1rem 1.25rem;
		border-bottom: 1px solid #e2e7e0;
	}

	.modal-header p,
	.modal-header h2,
	.section-heading h3,
	.comments-heading p,
	.comments-heading h3 {
		margin: 0;
	}

	.modal-header p,
	.comments-heading p {
		margin-bottom: 0.18rem;
		color: #52705a;
		font-size: 0.67rem;
		font-weight: 750;
		letter-spacing: 0.09em;
		text-transform: uppercase;
	}

	.modal-header h2 {
		max-width: 42rem;
		overflow: hidden;
		font-size: 1.08rem;
		line-height: 1.3;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.close {
		display: grid;
		width: 2rem;
		height: 2rem;
		place-items: center;
		padding: 0;
		border: 0;
		border-radius: 0.45rem;
		color: #667068;
		background: transparent;
		font-size: 1.35rem;
		line-height: 1;
		cursor: pointer;
	}

	.close:hover,
	.close:focus-visible {
		color: #245937;
		background: #eef4ed;
		outline: none;
	}

	.modal-content {
		display: grid;
		min-height: 0;
		grid-template-columns: minmax(0, 1fr) minmax(22rem, 24rem);
	}

	.details-column {
		display: grid;
		align-content: start;
		gap: 1.7rem;
		min-width: 0;
		padding: 1.35rem;
		overflow-y: auto;
	}

	.subtasks {
		display: grid;
		gap: 0.9rem;
		padding-top: 1.3rem;
		border-top: 1px solid #e2e7e0;
	}

	.section-heading {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(7rem, 11rem);
		align-items: center;
		gap: 1rem;
	}

	.section-heading > div {
		display: flex;
		align-items: baseline;
		gap: 0.6rem;
	}

	.section-heading h3,
	.comments-heading h3 {
		font-size: 0.96rem;
	}

	.section-heading span,
	.comments-heading > span {
		color: #858b84;
		font-size: 0.72rem;
		font-weight: 650;
	}

	progress {
		width: 100%;
		height: 0.38rem;
		border: 0;
		border-radius: 999px;
		background: #e7ece5;
		accent-color: #3f7951;
	}

	progress::-webkit-progress-bar {
		border-radius: 999px;
		background: #e7ece5;
	}

	progress::-webkit-progress-value {
		border-radius: 999px;
		background: #3f7951;
	}

	.subtask-list,
	.comments-thread ol {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.subtask-list {
		display: grid;
		gap: 0.15rem;
	}

	.subtask-list li {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		align-items: center;
		gap: 0.7rem;
		min-height: 2.55rem;
		padding: 0.35rem 0.45rem;
		border-bottom: 1px solid #edf0ec;
		color: #303631;
		font-size: 0.86rem;
	}

	.subtask-list li.completed > span {
		color: #8a908a;
		text-decoration: line-through;
	}

	.status-toggle {
		display: grid;
		width: 1.25rem;
		height: 1.25rem;
		place-items: center;
		padding: 0;
		border: 1.5px solid #95a097;
		border-radius: 50%;
		color: transparent;
		background: #fff;
		cursor: pointer;
	}

	.completed .status-toggle {
		border-color: #3f7951;
		color: #fff;
		background: #3f7951;
		font-size: 0.67rem;
	}

	.status-toggle:focus-visible {
		outline: 3px solid rgb(71 125 86 / 18%);
		outline-offset: 2px;
	}

	.inline-composer {
		display: grid;
		gap: 0.38rem;
	}

	.inline-composer label,
	.comment-composer label {
		color: #526058;
		font-size: 0.73rem;
		font-weight: 700;
	}

	.inline-composer > div {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 0.55rem;
	}

	.inline-composer input,
	.comment-composer textarea {
		width: 100%;
		padding: 0.66rem 0.72rem;
		border: 1px solid #ccd6ca;
		border-radius: 0.62rem;
		color: #17211a;
		background: #fff;
		outline: none;
	}

	.inline-composer input:focus,
	.comment-composer textarea:focus {
		border-color: #477d56;
		box-shadow: 0 0 0 0.2rem rgb(71 125 86 / 12%);
	}

	.inline-composer button,
	.comment-composer button {
		padding: 0.62rem 0.78rem;
		border: 1px solid #2d6540;
		border-radius: 0.58rem;
		color: #fff;
		background: #2d6540;
		font-size: 0.77rem;
		font-weight: 700;
		cursor: pointer;
	}

	button:disabled {
		cursor: wait;
		opacity: 0.5;
	}

	.comments-column {
		display: flex;
		min-width: 0;
		min-height: 0;
		flex-direction: column;
		border-left: 1px solid #e2e7e0;
		background: #f8faf7;
	}

	.comments-heading {
		display: flex;
		flex: none;
		align-items: center;
		justify-content: space-between;
		padding: 1.15rem 1.2rem;
		border-bottom: 1px solid #e2e7e0;
	}

	.comments-thread {
		min-height: 10rem;
		flex: 1;
		padding: 1rem 1.2rem;
		overflow-y: auto;
	}

	.comments-thread ol {
		display: grid;
		gap: 0.8rem;
	}

	.comments-thread li {
		padding: 0.78rem 0.85rem;
		border: 1px solid #dde5da;
		border-radius: 0.75rem;
		background: #fff;
		box-shadow: 0 0.35rem 1rem rgb(30 52 35 / 4%);
	}

	.comment-meta {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		margin-bottom: 0.45rem;
	}

	.comment-meta > div {
		display: flex;
		align-items: baseline;
		gap: 0.4rem;
		min-width: 0;
	}

	.comment-meta strong {
		color: #455047;
		font-size: 0.7rem;
	}

	.comment-meta time {
		color: #858b84;
		font-size: 0.67rem;
		font-weight: 650;
	}

	.comment-meta button {
		padding: 0;
		border: 0;
		color: #7b837c;
		background: transparent;
		font-size: 0.67rem;
		cursor: pointer;
	}

	.comment-meta button:hover:not(:disabled) {
		color: #a13d34;
	}

	.comments-thread li > p {
		margin: 0;
		color: #343a35;
		font-size: 0.82rem;
		line-height: 1.5;
		white-space: pre-wrap;
	}

	.comments-empty {
		display: grid;
		place-items: center;
		min-height: 9rem;
		padding: 1rem;
		text-align: center;
	}

	.comments-empty p,
	.comments-empty span {
		margin: 0;
	}

	.comments-empty p {
		color: #4f5b52;
		font-size: 0.84rem;
		font-weight: 700;
	}

	.comments-empty span {
		max-width: 13rem;
		color: #8a908a;
		font-size: 0.73rem;
		line-height: 1.45;
	}

	.comment-composer {
		position: sticky;
		bottom: 0;
		display: grid;
		flex: none;
		gap: 0.45rem;
		padding: 0.9rem 1.2rem 1.1rem;
		border-top: 1px solid #e2e7e0;
		background: #f8faf7;
	}

	.comment-composer textarea {
		min-height: 4.5rem;
		resize: vertical;
	}

	.comment-composer > div {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.7rem;
	}

	.loading,
	.empty,
	.error {
		margin: 0;
		font-size: 0.78rem;
	}

	.loading,
	.empty {
		color: #858b84;
	}

	.error {
		color: #8c2828;
	}

	@media (max-width: 50rem) {
		.modal-content {
			display: block;
			overflow-y: auto;
		}

		.details-column {
			overflow: visible;
		}

		.comments-column {
			min-height: 28rem;
			border-top: 1px solid #e2e7e0;
			border-left: 0;
		}

		.comments-thread {
			max-height: 22rem;
		}
	}

	@media (max-width: 44rem) {
		.modal-backdrop {
			align-items: end;
			padding: 0;
		}

		.task-editor-modal {
			max-height: calc(100dvh - 0.5rem);
			border-right: 0;
			border-bottom: 0;
			border-left: 0;
			border-radius: 1rem 1rem 0 0;
		}

		.modal-header,
		.details-column {
			padding-right: 1rem;
			padding-left: 1rem;
		}

		.section-heading {
			grid-template-columns: 1fr;
			gap: 0.55rem;
		}

		.inline-composer > div {
			grid-template-columns: 1fr;
		}

		.inline-composer button {
			justify-self: end;
		}
	}
</style>
