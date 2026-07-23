<script lang="ts">
	import { browser } from '$app/environment';
	import { afterNavigate, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import TaskEditorModal from '$lib/components/TaskEditorModal.svelte';
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { listProjectSections } from '$lib/projects/client';
	import { getTask, type Task, type TaskUpdate, updateTask } from './client';
	import {
		closeTask,
		consumeTaskNavigationSnapshot,
		parseTaskPath,
		replaceMismatchedTaskRoute,
		taskNavigationEvent,
		type TaskNavigationSnapshot,
		type TaskRoute
	} from './navigation';
	import { onMount } from 'svelte';

	interface Props {
		projects: Project[];
		loadTask?: (taskId: string) => Promise<Task>;
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		saveTask?: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		refresh?: () => Promise<void>;
		routeOverride?: TaskRoute;
		closeRoute?: (route: TaskRoute) => void;
		readNavigationSnapshot?: (route: TaskRoute) => TaskNavigationSnapshot | undefined;
	}

	let {
		projects,
		loadTask = (taskId) => getTask(fetch, taskId),
		loadSections = (projectId) => listProjectSections(fetch, projectId),
		saveTask = (taskId, changes) => updateTask(fetch, taskId, changes),
		refresh = () => invalidateAll(),
		routeOverride,
		closeRoute = closeTask,
		readNavigationSnapshot = consumeTaskNavigationSnapshot
	}: Props = $props();

	let task = $state<Task | undefined>();
	let sections = $state<ProjectSection[] | undefined>();
	let loading = $state(false);
	let errorMessage = $state('');
	let requestVersion = 0;
	let locationPath = $state(browser ? window.location.pathname : page.url.pathname);
	let route = $derived(routeOverride ?? parseTaskPath(locationPath));

	afterNavigate(() => {
		locationPath = window.location.pathname;
	});

	onMount(() => {
		const updateLocation = () => (locationPath = window.location.pathname);
		window.addEventListener('popstate', updateLocation);
		window.addEventListener(taskNavigationEvent, updateLocation);
		return () => {
			window.removeEventListener('popstate', updateLocation);
			window.removeEventListener(taskNavigationEvent, updateLocation);
		};
	});

	$effect(() => {
		const nextRoute = route;
		const version = ++requestVersion;
		errorMessage = '';
		if (!nextRoute) {
			task = undefined;
			sections = undefined;
			loading = false;
			return;
		}
		const snapshot = readNavigationSnapshot(nextRoute);
		if (snapshot && validTask(snapshot.task)) {
			task = snapshot.task;
			sections = snapshot.sections;
			loading = false;
			return;
		}
		task = undefined;
		sections = undefined;
		loading = true;
		void prepareTask(nextRoute, version);
	});

	async function prepareTask(nextRoute: TaskRoute, version: number) {
		try {
			const loadedTask = await loadTask(nextRoute.taskId);
			if (version !== requestVersion) return;
			if (!validTask(loadedTask)) {
				throw new Error('Task is not an available top-level task.');
			}
			if (await replaceMismatchedTaskRoute(loadedTask, nextRoute)) return;
			const loadedSections = await loadSections(loadedTask.projectId);
			if (version !== requestVersion) return;
			task = loadedTask;
			sections = loadedSections;
		} catch {
			if (version === requestVersion) {
				errorMessage = 'This task or project is unavailable.';
			}
		} finally {
			if (version === requestVersion) loading = false;
		}
	}

	function validTask(candidate: Task): boolean {
		return candidate.parentId === null && projects.some((item) => item.id === candidate.projectId);
	}

	async function save(changes: TaskUpdate) {
		if (!task || !route) return;
		await saveTask(task.id, changes);
		closeRoute(route);
		window.setTimeout(() => void refresh(), 200);
	}
</script>

{#if route && loading}
	<div class="route-modal-backdrop" role="presentation">
		<div class="route-state" role="dialog" aria-modal="true" aria-label="Loading task editor">
			<p role="status">Loading task…</p>
		</div>
	</div>
{:else if route && errorMessage}
	<div class="route-modal-backdrop" role="presentation">
		<div
			class="route-state error-state"
			role="dialog"
			aria-modal="true"
			aria-label="Task unavailable"
		>
			<h2>Task unavailable</h2>
			<p role="alert">{errorMessage}</p>
			<button type="button" onclick={() => closeRoute(route!)}>Return to tasks</button>
		</div>
	</div>
{:else if route && task && sections}
	{#key task.id}
		<TaskEditorModal
			{task}
			{projects}
			{sections}
			{loadSections}
			{save}
			close={() => closeRoute(route!)}
		/>
	{/key}
{/if}

<style>
	.route-modal-backdrop {
		position: fixed;
		z-index: 100;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 1.25rem;
		background: color-mix(in srgb, var(--theme-accent, #18221b) 18%, rgb(18 18 18 / 38%));
		backdrop-filter: blur(2px);
	}
	.route-state {
		width: min(26rem, 100%);
		padding: 1.5rem;
		border: 1px solid var(--theme-border, #d5dfd3);
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 1.5rem 4rem rgb(18 18 18 / 18%);
		text-align: center;
	}
	.route-state p,
	.route-state h2 {
		margin: 0;
	}
	.error-state {
		display: grid;
		gap: 0.75rem;
	}
	.error-state p {
		color: #7d3933;
	}
	.error-state button {
		justify-self: center;
		padding: 0.65rem 0.8rem;
		border: 1px solid var(--theme-accent, #2d6540);
		border-radius: 0.6rem;
		color: #fff;
		background: var(--theme-accent, #2d6540);
		font: inherit;
		font-weight: 700;
		cursor: pointer;
	}
</style>
