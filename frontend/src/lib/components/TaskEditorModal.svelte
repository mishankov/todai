<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import { onMount } from 'svelte';
	import TaskEditor from './TaskEditor.svelte';

	interface Props {
		task: Task;
		projects: Project[];
		sections?: ProjectSection[];
		currentProjectId?: string;
		save: (update: TaskUpdate) => Promise<void>;
		close: () => void;
	}

	let { task, projects, sections, currentProjectId, save, close }: Props = $props();
	let dialog: HTMLElement;

	onMount(() => {
		const previousOverflow = document.documentElement.style.overflow;
		const previouslyFocused =
			document.activeElement instanceof HTMLElement ? document.activeElement : null;
		document.documentElement.style.overflow = 'hidden';
		dialog.querySelector<HTMLInputElement>('input')?.focus();
		return () => {
			document.documentElement.style.overflow = previousOverflow;
			queueMicrotask(() => {
				if (previouslyFocused?.isConnected) previouslyFocused.focus();
			});
		};
	});

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
		<header>
			<div>
				<p>Task details</p>
				<h2>Edit task</h2>
			</div>
			<button class="close" type="button" aria-label="Close task editor" onclick={close}>×</button>
		</header>

		<TaskEditor
			{task}
			{projects}
			{sections}
			{currentProjectId}
			{save}
			cancel={close}
			variant="dialog"
		/>
	</div>
</div>

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 1.25rem;
		background: rgb(24 34 27 / 38%);
		backdrop-filter: blur(2px);
		z-index: 100;
	}
	.task-editor-modal {
		width: min(42rem, 100%);
		max-height: calc(100dvh - 2.5rem);
		overflow-y: auto;
		padding: 1.35rem;
		border: 1px solid #d5dfd3;
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 1.5rem 4rem rgb(18 42 26 / 22%);
	}
	header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1.25rem;
		padding-bottom: 0.9rem;
		border-bottom: 1px solid #e2e7e0;
	}
	header p,
	header h2 {
		margin: 0;
	}
	header p {
		margin-bottom: 0.2rem;
		color: #52705a;
		font-size: 0.68rem;
		font-weight: 720;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	header h2 {
		font-size: 1.15rem;
		line-height: 1.3;
	}
	.close {
		display: grid;
		place-items: center;
		width: 2rem;
		height: 2rem;
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

	@media (max-width: 44rem) {
		.modal-backdrop {
			align-items: end;
			padding: 0;
		}
		.task-editor-modal {
			max-height: calc(100dvh - 1rem);
			padding: 1.1rem;
			border-right: 0;
			border-bottom: 0;
			border-left: 0;
			border-radius: 1rem 1rem 0 0;
		}
	}
</style>
