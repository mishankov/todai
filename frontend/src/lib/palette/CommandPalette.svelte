<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { commandRegistry, formatShortcut, type ProductCommand } from '$lib/shortcuts/registry';
	import type { Task } from '$lib/tasks/client';
	import { onMount, tick } from 'svelte';
	import { searchTasks as requestTaskSearch } from './client';
	import {
		buildLocalResults,
		buildTaskResults,
		normalizePaletteQuery,
		type PaletteResult
	} from './model';

	interface Props {
		projects: Project[];
		activeProject?: Project;
		applePlatform: boolean;
		close: (restoreFocus?: boolean) => void;
		executeCommand: (command: ProductCommand) => void | Promise<void>;
		switchProject: (project: Project) => void | Promise<void>;
		selectTask: (task: Task, sections?: ProjectSection[]) => void | Promise<void>;
		search?: (query: string, projectId: string, signal: AbortSignal) => Promise<Task[]>;
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		debounceMs?: number;
	}

	let {
		projects,
		activeProject,
		applePlatform,
		close,
		executeCommand,
		switchProject,
		selectTask,
		search = (query, projectId, signal) =>
			requestTaskSearch(fetch, query, projectId, { limit: 20, signal }),
		loadSections = async () => [],
		debounceMs = 180
	}: Props = $props();

	let dialog: HTMLDivElement;
	let input: HTMLInputElement;
	let query = $state('');
	let tasks = $state<Task[]>([]);
	let sections = $state<ProjectSection[] | undefined>();
	let sectionsPromise: Promise<ProjectSection[] | undefined> = Promise.resolve(undefined);
	let loading = $state(false);
	let taskError = $state(false);
	let activeIndex = $state(0);
	let requestVersion = 0;
	let sectionVersion = 0;
	let localResults = $derived(
		buildLocalResults(query, commandRegistry, projects, activeProject?.id)
	);
	let taskResults = $derived(buildTaskResults(tasks, sections ?? []));
	let results = $derived<PaletteResult[]>([...localResults, ...taskResults]);
	let activeResult = $derived(results[activeIndex]);
	let announcement = $derived.by(() => {
		if (loading) return `Searching tasks. ${results.length} local results available.`;
		if (taskError) return `Task search failed. ${results.length} local results available.`;
		if (results.length === 0) return 'No results.';
		return `${results.length} result${results.length === 1 ? '' : 's'}. ${activeResult?.label ?? ''}`;
	});

	onMount(() => {
		const previousOverflow = document.documentElement.style.overflow;
		document.documentElement.style.overflow = 'hidden';
		void tick().then(() => input.focus());
		return () => {
			requestVersion += 1;
			sectionVersion += 1;
			document.documentElement.style.overflow = previousOverflow;
		};
	});

	$effect(() => {
		const projectId = activeProject?.id;
		const version = ++sectionVersion;
		sections = undefined;
		if (!projectId) return;
		sectionsPromise = loadSections(projectId)
			.then((loaded) => {
				if (version === sectionVersion) sections = loaded;
				return loaded;
			})
			.catch(() => {
				if (version === sectionVersion) sections = undefined;
				return undefined;
			});
	});

	$effect(() => {
		const normalized = normalizePaletteQuery(query);
		const projectId = activeProject?.id;
		const version = ++requestVersion;
		activeIndex = 0;
		tasks = [];
		taskError = false;
		if (!normalized || !projectId) {
			loading = false;
			return;
		}

		loading = true;
		const controller = new AbortController();
		const timer = window.setTimeout(() => {
			void search(normalized, projectId, controller.signal)
				.then((found) => {
					if (version !== requestVersion || controller.signal.aborted) return;
					tasks = found;
					loading = false;
				})
				.catch((error: unknown) => {
					if (version !== requestVersion || controller.signal.aborted) return;
					if (error instanceof DOMException && error.name === 'AbortError') return;
					taskError = true;
					loading = false;
				});
		}, debounceMs);

		return () => {
			window.clearTimeout(timer);
			controller.abort();
		};
	});

	$effect(() => {
		if (results.length === 0) activeIndex = 0;
		else if (activeIndex >= results.length) activeIndex = results.length - 1;
	});

	$effect(() => {
		const optionId = activeOptionId();
		void tick().then(() => {
			document.getElementById(optionId)?.scrollIntoView({ block: 'nearest' });
		});
	});

	function activeOptionId(): string {
		return activeResult ? `palette-${safeId(activeResult.id)}` : '';
	}

	function safeId(value: string): string {
		return value.replace(/[^a-zA-Z0-9_-]/g, '-');
	}

	function groupedResults(group: PaletteResult['group']): PaletteResult[] {
		return results.filter((result) => result.group === group);
	}

	function moveActive(delta: number) {
		if (results.length === 0) return;
		activeIndex = (activeIndex + delta + results.length) % results.length;
	}

	async function choose(result: PaletteResult | undefined) {
		if (!result) return;
		close(false);
		await tick();
		if (result.kind === 'command') await executeCommand(result.command);
		else if (result.kind === 'project') await switchProject(result.project);
		else await selectTask(result.task, sections ?? (await sectionsPromise));
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
			close();
			return;
		}
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			moveActive(event.key === 'ArrowDown' ? 1 : -1);
			return;
		}
		if (event.key === 'Home' || event.key === 'End') {
			event.preventDefault();
			activeIndex = event.key === 'Home' ? 0 : Math.max(0, results.length - 1);
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			void choose(activeResult);
			return;
		}
		if (event.key !== 'Tab') return;

		const focusable = Array.from(
			dialog.querySelectorAll<HTMLElement>(
				'input:not([disabled]), button:not([disabled]):not([tabindex="-1"])'
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
	class="palette-backdrop"
	role="presentation"
	onclick={(event) => {
		if (event.target === event.currentTarget) close();
	}}
>
	<div
		bind:this={dialog}
		class="palette-dialog"
		role="dialog"
		aria-modal="true"
		aria-labelledby="command-palette-title"
	>
		<h2 id="command-palette-title" class="sr-only">Command palette</h2>
		<div class="search-row">
			<svg viewBox="0 0 24 24" aria-hidden="true"
				><circle cx="11" cy="11" r="6" /><path d="m16 16 4 4" /></svg
			>
			<input
				bind:this={input}
				bind:value={query}
				role="combobox"
				aria-label="Search commands, projects, and tasks"
				aria-autocomplete="list"
				aria-expanded="true"
				aria-controls="command-palette-results"
				aria-activedescendant={activeOptionId() || undefined}
				placeholder="Search commands, projects, and tasks"
				autocomplete="off"
			/>
			<button type="button" aria-label="Close command palette" onclick={() => close()}>×</button>
		</div>

		<div
			id="command-palette-results"
			class="results"
			role="listbox"
			aria-label="Palette results"
			aria-busy={loading}
		>
			{#each ['Commands', 'Projects', 'Tasks'] as group (group)}
				{@const groupResults = groupedResults(group as PaletteResult['group'])}
				{#if groupResults.length > 0}
					<div class="group" role="group" aria-labelledby={`palette-group-${group}`}>
						<p id={`palette-group-${group}`} class="group-label">{group}</p>
						{#each groupResults as result (result.id)}
							{@const index = results.indexOf(result)}
							<button
								id={`palette-${safeId(result.id)}`}
								class:active={index === activeIndex}
								class:current={result.kind === 'project' && result.active}
								type="button"
								role="option"
								tabindex="-1"
								aria-selected={index === activeIndex}
								onmouseenter={() => (activeIndex = index)}
								onclick={() => void choose(result)}
							>
								<span class="result-copy">
									<strong>{result.label}</strong>
									<small>{result.description}</small>
								</span>
								{#if result.kind === 'command' && result.command.shortcut}
									<kbd
										>{formatShortcut(
											{
												...result.command,
												shortcut: result.command.shortcut,
												code: result.command.shortcut.code,
												keyLabel: result.command.shortcut.keyLabel
											},
											applePlatform
										)}</kbd
									>
								{:else if result.kind === 'project' && result.active}
									<span class="badge">Active</span>
								{/if}
							</button>
						{/each}
					</div>
				{/if}
			{/each}

			{#if loading}<p class="state" role="status">Searching tasks…</p>{/if}
			{#if taskError}<p class="state error" role="alert">
					Task search failed. Change the search to try again.
				</p>{/if}
			{#if !loading && results.length === 0}<p class="state">
					No matching commands, projects, or tasks.
				</p>{/if}
		</div>
		<p class="sr-only" role="status" aria-live="polite" aria-atomic="true">{announcement}</p>
		<footer>
			<span><kbd>↑</kbd><kbd>↓</kbd> Navigate</span><span><kbd>Enter</kbd> Open</span><span
				><kbd>Esc</kbd> Close</span
			>
		</footer>
	</div>
</div>

<style>
	.palette-backdrop {
		position: fixed;
		z-index: 110;
		inset: 0;
		display: grid;
		align-items: start;
		justify-items: center;
		padding: max(8vh, 1rem) 1rem 1rem;
		background: rgb(24 25 23 / 46%);
		backdrop-filter: blur(3px);
	}
	.palette-dialog {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr) auto;
		width: min(44rem, 100%);
		max-height: min(42rem, calc(100dvh - max(16vh, 2rem)));
		border: 1px solid var(--theme-border, #dfe5dc);
		border-radius: 1rem;
		background: var(--theme-canvas, #fff);
		box-shadow: 0 1.5rem 5rem rgb(20 28 21 / 28%);
		overflow: hidden;
	}
	.search-row {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		padding: 0.8rem 1rem;
		border-bottom: 1px solid var(--theme-border, #dfe5dc);
		background: #fff;
	}
	.search-row svg {
		width: 1.25rem;
		height: 1.25rem;
		fill: none;
		stroke: var(--theme-accent, #2d6540);
		stroke-width: 1.8;
		stroke-linecap: round;
	}
	.search-row input {
		min-width: 0;
		flex: 1;
		border: 0;
		outline: 0;
		color: #282927;
		background: transparent;
		font: inherit;
		font-size: 1rem;
	}
	.search-row button {
		width: 2rem;
		height: 2rem;
		padding: 0;
		border: 0;
		border-radius: 50%;
		color: #686963;
		background: transparent;
		font-size: 1.35rem;
		cursor: pointer;
	}
	.search-row button:hover,
	.search-row button:focus-visible {
		background: var(--theme-hover, #e6ece4);
		outline: none;
	}
	.results {
		min-height: 10rem;
		padding: 0.45rem;
		overflow-y: auto;
		overscroll-behavior: contain;
	}
	.group-label {
		margin: 0.45rem 0.65rem 0.25rem;
		color: #777973;
		font-size: 0.65rem;
		font-weight: 800;
		letter-spacing: 0.09em;
		text-transform: uppercase;
	}
	.group button {
		display: flex;
		align-items: center;
		gap: 0.8rem;
		width: 100%;
		padding: 0.65rem 0.75rem;
		border: 0;
		border-radius: 0.55rem;
		color: #30312e;
		background: transparent;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	.group button.active {
		background: var(--theme-accent-soft, #dfeadf);
		box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--theme-accent, #2d6540) 24%, transparent);
	}
	.result-copy {
		display: grid;
		min-width: 0;
		flex: 1;
		gap: 0.12rem;
	}
	.result-copy strong,
	.result-copy small {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.result-copy strong {
		font-size: 0.86rem;
	}
	.result-copy small {
		color: #71736d;
		font-size: 0.72rem;
	}
	kbd,
	.badge {
		flex: none;
		padding: 0.16rem 0.36rem;
		border: 1px solid var(--theme-border, #dfe5dc);
		border-radius: 0.3rem;
		color: #62645e;
		background: #fff;
		font-family: inherit;
		font-size: 0.62rem;
		font-weight: 700;
	}
	.badge {
		color: var(--theme-accent, #2d6540);
		background: var(--theme-accent-soft, #dfeadf);
	}
	.state {
		margin: 0.9rem 0.7rem;
		color: #6d6f69;
		font-size: 0.78rem;
	}
	.state.error {
		color: #a33f37;
	}
	footer {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
		padding: 0.65rem 1rem;
		border-top: 1px solid var(--theme-border, #dfe5dc);
		color: #777973;
		background: color-mix(in srgb, var(--theme-canvas, #fff) 88%, var(--theme-sidebar, #f1f5ef));
		font-size: 0.66rem;
	}
	footer span {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}
	footer kbd {
		padding: 0.05rem 0.22rem;
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
	@media (max-width: 40rem) {
		.palette-backdrop {
			align-items: stretch;
			padding: max(env(safe-area-inset-top), 0.5rem) max(env(safe-area-inset-right), 0.5rem)
				max(env(safe-area-inset-bottom), 0.5rem) max(env(safe-area-inset-left), 0.5rem);
		}
		.palette-dialog {
			width: 100%;
			max-height: 100%;
			border-radius: 0.8rem;
		}
		footer {
			display: none;
		}
	}
</style>
