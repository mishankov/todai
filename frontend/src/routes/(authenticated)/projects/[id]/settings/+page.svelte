<script lang="ts">
	import { invalidate } from '$app/navigation';
	import {
		ProjectConflictError,
		projectColorThemes,
		type AgentThinkingEffort,
		type Project,
		type ProjectColorTheme,
		type ProjectLayout,
		updateProject
	} from '$lib/projects/client';
	import { untrack } from 'svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const initial = initialForm();
	let current = $state<Project>(initial.project);
	let name = $state(initial.name);
	let layout = $state<ProjectLayout>(initial.layout);
	let colorTheme = $state<ProjectColorTheme>(initial.colorTheme);
	let agentModel = $state(initial.agentModel);
	let agentThinkingEffort = $state<AgentThinkingEffort>(initial.agentThinkingEffort);
	let saving = $state(false);
	let saved = $state(false);
	let errorMessage = $state('');

	function initialForm() {
		const project = data.project;
		return {
			project,
			name: project.name,
			layout: project.layout,
			colorTheme: project.colorTheme,
			agentModel: project.agentModel,
			agentThinkingEffort: project.agentThinkingEffort
		};
	}

	$effect(() => {
		const next = data.project;
		untrack(() => {
			if (next.id === current.id && next.version === current.version) return;
			current = next;
			name = next.name;
			layout = next.layout;
			colorTheme = next.colorTheme;
			agentModel = next.agentModel;
			agentThinkingEffort = next.agentThinkingEffort;
			saved = false;
		});
	});

	async function save() {
		if (!name.trim()) return;
		saving = true;
		saved = false;
		errorMessage = '';
		try {
			current = await updateProject(fetch, current.id, {
				version: current.version,
				name: name.trim(),
				layout,
				colorTheme,
				agentModel,
				agentThinkingEffort
			});
			name = current.name;
			await invalidate((url) => url.pathname === '/api/projects');
			saved = true;
		} catch (error) {
			errorMessage =
				error instanceof ProjectConflictError
					? 'This project changed elsewhere. Reload before saving.'
					: error instanceof Error
						? error.message
						: 'Could not save project settings.';
		} finally {
			saving = false;
		}
	}

	function thinkingEffortLabel(effort: string): string {
		return (
			{
				off: 'Off',
				minimal: 'Minimal',
				low: 'Low',
				medium: 'Medium',
				high: 'High',
				xhigh: 'Extra high',
				max: 'Maximum'
			}[effort] ?? effort
		);
	}
</script>

<svelte:head><title>Settings · {current.name} — Todai</title></svelte:head>

<section class="settings-page" aria-labelledby="project-settings-title">
	<header>
		<p>PROJECT</p>
		<h1 id="project-settings-title">Settings</h1>
		<span>Shape this workspace without affecting your other projects.</span>
	</header>

	<form
		onsubmit={(event) => {
			event.preventDefault();
			void save();
		}}
	>
		<section class="settings-group" aria-labelledby="workspace-settings">
			<div>
				<h2 id="workspace-settings">Workspace</h2>
				<p>Name and default view for this project's tasks.</p>
			</div>
			<div class="controls">
				<label><span>Name</span><input bind:value={name} maxlength="200" required /></label>
				<label>
					<span>Tasks layout</span>
					<select bind:value={layout}>
						<option value="list">List</option>
						<option value="board">Board</option>
					</select>
				</label>
			</div>
		</section>

		<section class="settings-group" aria-labelledby="appearance-settings">
			<div>
				<h2 id="appearance-settings">Color theme</h2>
				<p>A visual cue that follows you throughout this project.</p>
			</div>
			<div class="theme-grid">
				{#each projectColorThemes as theme (theme.id)}
					<label class={`theme-option theme-${theme.id}`}>
						<input type="radio" name="color-theme" value={theme.id} bind:group={colorTheme} />
						<span class="swatch" aria-hidden="true"></span>
						<strong>{theme.name}</strong>
						<small>{theme.description}</small>
					</label>
				{/each}
			</div>
		</section>

		<section class="settings-group" aria-labelledby="agent-settings">
			<div>
				<h2 id="agent-settings">Agent</h2>
				<p>These choices apply whenever the agent works in this project.</p>
			</div>
			<div class="controls">
				<label>
					<span>Model</span>
					<select
						bind:value={agentModel}
						disabled={data.settings.availableAgentModels.length === 0}
					>
						{#each data.settings.availableAgentModels as model (model)}
							<option value={model}>{model}</option>
						{/each}
					</select>
				</label>
				<label>
					<span>Thinking effort</span>
					<select
						bind:value={agentThinkingEffort}
						disabled={data.settings.availableAgentThinkingEfforts.length === 0}
					>
						{#each data.settings.availableAgentThinkingEfforts as effort (effort)}
							<option value={effort}>{thinkingEffortLabel(effort)}</option>
						{/each}
					</select>
				</label>
			</div>
		</section>

		<footer>
			{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}
			{#if saved}<p class="success" role="status">Project settings saved.</p>{/if}
			<button type="submit" disabled={saving || !name.trim() || !agentModel}>
				{saving ? 'Saving…' : 'Save changes'}
			</button>
		</footer>
	</form>
</section>

<style>
	.settings-page {
		width: min(50rem, 100%);
		margin: 0 auto;
		color: var(--color-text);
	}
	header {
		margin-bottom: 2.5rem;
	}
	header p {
		margin: 0 0 0.55rem;
		color: var(--theme-accent);
		font-size: 0.75rem;
		font-weight: 800;
		letter-spacing: 0.12em;
	}
	h1 {
		margin: 0 0 0.65rem;
		font-size: clamp(2.2rem, 6vw, 3.4rem);
		letter-spacing: -0.055em;
		line-height: 1;
	}
	header span,
	.settings-group p {
		color: var(--color-text-secondary);
		font-size: 0.9rem;
	}
	form,
	.controls {
		display: grid;
		gap: 1rem;
	}
	.settings-group {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(17rem, 22rem);
		gap: 2.5rem;
		align-items: start;
		padding: 1.5rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.9rem;
		background: var(--color-surface);
	}
	h2 {
		margin: 0 0 0.4rem;
		font-size: 1.05rem;
	}
	.settings-group p {
		margin: 0;
		line-height: 1.5;
	}
	label {
		display: grid;
		gap: 0.5rem;
		font-size: 0.78rem;
		font-weight: 750;
	}
	input,
	select {
		width: 100%;
		min-height: 2.8rem;
		padding: 0 0.8rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.55rem;
		color: var(--color-text);
		background: var(--theme-canvas);
		font: inherit;
		font-size: 0.88rem;
	}
	input:focus,
	select:focus {
		outline: 3px solid var(--theme-focus);
		border-color: var(--theme-accent);
	}
	.theme-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.65rem;
	}
	.theme-option {
		position: relative;
		display: grid;
		grid-template-columns: 1.6rem 1fr;
		gap: 0.05rem 0.65rem;
		padding: 0.7rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.6rem;
		cursor: pointer;
	}
	.theme-option:has(input:checked) {
		border-color: var(--preview-accent);
		box-shadow: 0 0 0 2px color-mix(in srgb, var(--preview-accent) 18%, transparent);
	}
	.theme-option input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}
	.swatch {
		grid-row: 1 / span 2;
		width: 1.6rem;
		height: 1.6rem;
		border-radius: 50%;
		background: var(--preview-accent);
	}
	.theme-option strong {
		font-size: 0.82rem;
	}
	.theme-option small {
		color: var(--color-text-muted);
		font-size: 0.7rem;
		font-weight: 500;
	}
	footer {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 1rem;
		min-height: 3rem;
	}
	footer p {
		margin: 0 auto 0 0;
		font-size: 0.82rem;
	}
	.error {
		color: var(--color-error);
	}
	.success {
		color: var(--theme-accent);
	}
	footer button {
		padding: 0.72rem 1rem;
		border: 0;
		border-radius: 0.55rem;
		color: var(--color-on-accent);
		background: var(--theme-accent-solid, var(--theme-accent));
		font-weight: 750;
		cursor: pointer;
	}
	button:disabled {
		cursor: wait;
		opacity: 0.55;
	}
	@media (max-width: 44rem) {
		.settings-group {
			grid-template-columns: 1fr;
			gap: 1.2rem;
		}
	}
</style>
