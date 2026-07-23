<script lang="ts">
	/* Project links are resolved centrally by projectHref; the lint rule only recognizes inline calls. */
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import type { Appearance } from '$lib/appearance/theme';
	import type { Project, ProjectColorTheme } from '$lib/projects/client';
	import { recordProjectPath, rememberedProjectPath } from '$lib/projects/navigation';
	import { commandPaletteRequestEvent, quickAddRequestEvent } from '$lib/shortcuts/events';
	import {
		ariaShortcut,
		formatShortcut,
		formatShortcutHint,
		formatShortcuts,
		isApplePlatform,
		shortcutCommand
	} from '$lib/shortcuts/registry';
	import { untrack, type Snippet } from 'svelte';

	interface Props {
		username: string;
		projects?: Project[];
		activeProject?: Project;
		appearance: Appearance;
		onAppearanceChange: (appearance: Appearance) => Promise<void>;
		onLogout: () => Promise<void>;
		currentPath?: string;
		children?: Snippet;
	}

	let {
		username,
		projects = [],
		activeProject,
		appearance,
		onAppearanceChange,
		onLogout,
		currentPath = '/',
		children
	}: Props = $props();
	let signingOut = $state(false);
	let errorMessage = $state('');
	let appearanceError = $state('');
	let appearanceStatus = $state('');
	let selectedAppearance = $state<Appearance>(untrack(() => appearance));
	let savingAppearance = $state<Appearance | null>(null);
	let sidebarOpen = $state(false);
	let theme = $derived<ProjectColorTheme>(activeProject?.colorTheme ?? 'sage');
	let projectBase = $derived(activeProject ? `/projects/${activeProject.id}` : '/projects');
	let applePlatform = $state(browser && isApplePlatform(window.navigator.platform));
	let quickAddCommand = shortcutCommand('quick-add');
	let quickAddHint = $derived(formatShortcutHint(quickAddCommand, applePlatform));
	let paletteCommand = shortcutCommand('command-palette');
	let paletteLabel = $derived(formatShortcut(paletteCommand, applePlatform));
	let quickAddDescription = $derived(formatShortcuts(quickAddCommand, applePlatform).join(' / '));
	const appearanceOptions: Appearance[] = ['system', 'light', 'dark'];

	$effect(() => {
		if (browser) applePlatform = isApplePlatform(window.navigator.platform);
	});

	$effect(() => {
		const savedAppearance = appearance;
		untrack(() => (selectedAppearance = savedAppearance));
	});

	$effect(() => {
		if (!browser || !activeProject || !currentPath.startsWith(`/projects/${activeProject.id}`))
			return;
		recordProjectPath(activeProject.id, currentPath);
	});

	async function signOut() {
		signingOut = true;
		errorMessage = '';
		try {
			await onLogout();
		} catch {
			errorMessage = 'Sign out failed. Please try again.';
			signingOut = false;
		}
	}

	async function switchProject(event: Event) {
		const projectId = (event.currentTarget as HTMLSelectElement).value;
		if (!projectId || projectId === activeProject?.id) return;
		sidebarOpen = false;
		await goto(rememberedProjectPath(projectId));
	}

	function projectHref(projectId: string, suffix: ProjectViewSuffix = ''): string {
		const params = { id: projectId };
		switch (suffix) {
			case '/overview':
				return resolve('/(authenticated)/projects/[id]/overview', params);
			case '/today':
				return resolve('/(authenticated)/projects/[id]/today', params);
			case '/tasks':
				return resolve('/(authenticated)/projects/[id]/tasks', params);
			case '/activity':
				return resolve('/(authenticated)/projects/[id]/activity', params);
			case '/settings':
				return resolve('/(authenticated)/projects/[id]/settings', params);
			default:
				return resolve('/(authenticated)/projects/[id]', params);
		}
	}

	function isActive(suffix = ''): boolean {
		return Boolean(activeProject && currentPath === `${projectBase}${suffix}`);
	}

	function closeSidebar() {
		sidebarOpen = false;
	}

	function openQuickAdd() {
		window.dispatchEvent(new CustomEvent(quickAddRequestEvent));
	}

	function openCommandPalette() {
		sidebarOpen = false;
		window.dispatchEvent(new CustomEvent(commandPaletteRequestEvent));
	}

	async function changeAppearance(nextAppearance: Appearance) {
		if (savingAppearance || nextAppearance === selectedAppearance) return;
		savingAppearance = nextAppearance;
		appearanceError = '';
		appearanceStatus = '';
		try {
			await onAppearanceChange(nextAppearance);
			selectedAppearance = nextAppearance;
			appearanceStatus = `Appearance set to ${appearanceLabel(nextAppearance)}.`;
		} catch (error) {
			appearanceError =
				error instanceof Error ? error.message : 'Could not save appearance. Please try again.';
		} finally {
			savingAppearance = null;
		}
	}

	function appearanceLabel(value: Appearance): string {
		return value[0].toUpperCase() + value.slice(1);
	}

	type ProjectViewSuffix = '' | '/overview' | '/today' | '/tasks' | '/activity' | '/settings';
</script>

<main class={`shell theme-${theme}`}>
	<button
		class:visible={sidebarOpen}
		class="sidebar-backdrop"
		type="button"
		aria-label="Close navigation"
		tabindex={sidebarOpen ? 0 : -1}
		onclick={closeSidebar}
	></button>

	<aside class:open={sidebarOpen} aria-label="Application sidebar">
		<div class="sidebar-heading">
			<a
				class="brand"
				href={activeProject ? projectHref(activeProject.id) : resolve('/projects')}
				onclick={closeSidebar}
			>
				<span class="mark" aria-hidden="true">T</span><span>Todai</span>
			</a>
			<button class="close-sidebar" type="button" aria-label="Close sidebar" onclick={closeSidebar}>
				<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m7 7 10 10M17 7 7 17" /></svg>
			</button>
		</div>

		<div class="project-switcher">
			<label for="active-project">Project</label>
			<select id="active-project" value={activeProject?.id ?? ''} onchange={switchProject}>
				{#if projects.length === 0}<option value="">No projects</option>{/if}
				{#if projects.length > 0 && !activeProject}
					<option value="" disabled>Select a project</option>
				{/if}
				{#each projects as project (project.id)}
					<option value={project.id}>{project.name}</option>
				{/each}
			</select>
		</div>

		<button
			class="global-command-palette"
			type="button"
			title={`Open command palette (${paletteLabel})`}
			aria-label={`Open command palette (${paletteLabel})`}
			aria-keyshortcuts={ariaShortcut(paletteCommand, applePlatform)}
			onclick={openCommandPalette}
		>
			<svg viewBox="0 0 24 24" aria-hidden="true"
				><circle cx="11" cy="11" r="6" /><path d="m16 16 4 4" /></svg
			>
			<span>Search</span><kbd>{paletteLabel}</kbd>
		</button>

		{#if activeProject}
			<button
				class="global-quick-add"
				type="button"
				title={`Create task (${quickAddDescription})`}
				aria-label={`Create task (${quickAddDescription})`}
				aria-keyshortcuts={ariaShortcut(quickAddCommand, applePlatform)}
				data-shortcut-hint={quickAddHint}
				onclick={openQuickAdd}
			>
				<span aria-hidden="true">＋</span> Create task
			</button>
			<nav class="primary-navigation" aria-label="Project navigation">
				<a
					href={projectHref(activeProject.id, '/overview')}
					class:active={isActive('/overview')}
					aria-current={isActive('/overview') ? 'page' : undefined}
					aria-keyshortcuts={ariaShortcut(shortcutCommand('project-overview'), applePlatform)}
					data-shortcut-hint={formatShortcutHint(
						shortcutCommand('project-overview'),
						applePlatform
					)}
					onclick={closeSidebar}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M4 5h7v6H4zM13 5h7v10h-7zM4 13h7v6H4zM13 17h7v2h-7z" /></svg
					><span>Overview</span>
				</a>
				<a
					href={projectHref(activeProject.id)}
					class:active={isActive()}
					aria-current={isActive() ? 'page' : undefined}
					aria-keyshortcuts={ariaShortcut(shortcutCommand('project-inbox'), applePlatform)}
					data-shortcut-hint={formatShortcutHint(shortcutCommand('project-inbox'), applePlatform)}
					onclick={closeSidebar}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M4 7.5h16v11H4zM7 4h10l3 3.5H4zM8 13h8" /></svg
					><span>Inbox</span>
				</a>
				<a
					href={projectHref(activeProject.id, '/today')}
					class:active={isActive('/today')}
					aria-current={isActive('/today') ? 'page' : undefined}
					aria-keyshortcuts={ariaShortcut(shortcutCommand('project-today'), applePlatform)}
					data-shortcut-hint={formatShortcutHint(shortcutCommand('project-today'), applePlatform)}
					onclick={closeSidebar}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><rect x="4" y="5" width="16" height="15" rx="2" /><path
							d="M8 3v4M16 3v4M4 9h16"
						/></svg
					><span>Today</span>
				</a>
				<a
					href={projectHref(activeProject.id, '/tasks')}
					class:active={isActive('/tasks')}
					aria-current={isActive('/tasks') ? 'page' : undefined}
					aria-keyshortcuts={ariaShortcut(shortcutCommand('project-tasks'), applePlatform)}
					data-shortcut-hint={formatShortcutHint(shortcutCommand('project-tasks'), applePlatform)}
					onclick={closeSidebar}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M5 6h14M5 12h14M5 18h14M3 6h.01M3 12h.01M3 18h.01" /></svg
					><span>Tasks</span>
				</a>
				<a
					href={projectHref(activeProject.id, '/activity')}
					class:active={isActive('/activity')}
					aria-current={isActive('/activity') ? 'page' : undefined}
					aria-keyshortcuts={ariaShortcut(shortcutCommand('project-activity'), applePlatform)}
					data-shortcut-hint={formatShortcutHint(
						shortcutCommand('project-activity'),
						applePlatform
					)}
					onclick={closeSidebar}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M12 7v5l3 2M21 12a9 9 0 1 1-3-6.7M21 4v5h-5" /></svg
					><span>Activity</span>
				</a>
			</nav>
		{:else}
			<a class="empty-projects" href={resolve('/projects')} onclick={closeSidebar}
				>Create your first project</a
			>
		{/if}

		<div class="session">
			{#if currentPath !== '/settings'}
				<fieldset class="appearance-switcher" class:saving={savingAppearance !== null}>
					<legend>Appearance</legend>
					<div>
						{#each appearanceOptions as option (option)}
							<button
								type="button"
								aria-pressed={selectedAppearance === option}
								aria-label={`Use ${option} appearance`}
								disabled={savingAppearance !== null}
								onclick={() => void changeAppearance(option)}
							>
								{#if option === 'system'}
									<svg viewBox="0 0 24 24" aria-hidden="true"
										><rect x="3" y="4" width="18" height="13" rx="2" /><path
											d="M8 21h8M12 17v4"
										/></svg
									>
								{:else if option === 'light'}
									<svg viewBox="0 0 24 24" aria-hidden="true"
										><circle cx="12" cy="12" r="4" /><path
											d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"
										/></svg
									>
								{:else}
									<svg viewBox="0 0 24 24" aria-hidden="true"
										><path d="M20 15.3A8.5 8.5 0 0 1 8.7 4 8.5 8.5 0 1 0 20 15.3z" /></svg
									>
								{/if}
								<span>{appearanceLabel(option)}</span>
							</button>
						{/each}
					</div>
				</fieldset>
				{#if appearanceError}<p class="appearance-error" role="alert">{appearanceError}</p>{/if}
				{#if appearanceStatus}<p class="sr-only" role="status">{appearanceStatus}</p>{/if}
			{/if}
			<nav class="utility-navigation" aria-label="Settings">
				{#if activeProject}
					<a
						href={projectHref(activeProject.id, '/settings')}
						class:active={isActive('/settings')}
						aria-current={isActive('/settings') ? 'page' : undefined}
						aria-keyshortcuts={ariaShortcut(shortcutCommand('project-settings'), applePlatform)}
						data-shortcut-hint={formatShortcutHint(
							shortcutCommand('project-settings'),
							applePlatform
						)}
						onclick={closeSidebar}
					>
						<svg viewBox="0 0 24 24" aria-hidden="true"
							><circle cx="12" cy="12" r="3" /><path
								d="M19 13.5v-3l-2-.7-.6-1.4.9-1.9-2.1-2.1-1.9.9-1.4-.6-.7-2H8.5l-.7 2-1.4.6-1.9-.9-2.1 2.1.9 1.9-.6 1.4-2 .7v3l2 .7.6 1.4-.9 1.9 2.1 2.1 1.9-.9 1.4.6.7 2h3l.7-2 1.4-.6 1.9.9 2.1-2.1-.9-1.9.6-1.4z"
							/></svg
						><span>Project settings</span>
					</a>
				{/if}
				<a
					href={resolve('/projects')}
					class:active={currentPath === '/projects'}
					aria-current={currentPath === '/projects' ? 'page' : undefined}
					onclick={closeSidebar}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M3 7.5h7l2 2h9v9.5H3zM3 7.5V5h7l2 2.5" /></svg
					><span>Manage projects</span>
				</a>
				<a
					href={resolve('/settings')}
					class:active={currentPath === '/settings'}
					aria-current={currentPath === '/settings' ? 'page' : undefined}
					onclick={closeSidebar}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><circle cx="12" cy="8" r="4" /><path d="M4.5 21a7.5 7.5 0 0 1 15 0" /></svg
					><span>Account settings</span>
				</a>
			</nav>
			<div class="profile">
				<span class="avatar" aria-hidden="true">{username.slice(0, 1).toUpperCase()}</span>
				<span class="username">{username}</span>
				<button class="logout" type="button" disabled={signingOut} onclick={signOut}>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M10 5H5v14h5M14 8l4 4-4 4M8 12h10" /></svg
					><span>{signingOut ? 'Logging out…' : 'Log out'}</span>
				</button>
			</div>
			{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}
		</div>
	</aside>

	<section class="content">
		<header class="mobile-bar">
			<button type="button" aria-label="Open navigation" onclick={() => (sidebarOpen = true)}
				><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg
				></button
			>
			<a class="brand" href={activeProject ? projectHref(activeProject.id) : resolve('/projects')}
				><span class="mark" aria-hidden="true">T</span><span>{activeProject?.name ?? 'Todai'}</span
				></a
			>
			<button
				type="button"
				aria-label={`Open command palette (${paletteLabel})`}
				aria-keyshortcuts={ariaShortcut(paletteCommand, applePlatform)}
				onclick={openCommandPalette}
				><svg viewBox="0 0 24 24" aria-hidden="true"
					><circle cx="11" cy="11" r="6" /><path d="m16 16 4 4" /></svg
				></button
			>
		</header>
		<section class="workspace">
			{#if children}{@render children()}{/if}
		</section>
	</section>
</main>

<style>
	.shell {
		display: grid;
		grid-template-columns: 17rem minmax(0, 1fr);
		min-height: 100vh;
		background: var(--theme-canvas);
	}
	aside {
		position: sticky;
		top: 0;
		display: flex;
		flex-direction: column;
		height: 100vh;
		padding: 1rem 0.75rem;
		border-right: 1px solid var(--theme-border);
		background: var(--theme-sidebar);
		overflow-y: auto;
	}
	.sidebar-heading,
	.mobile-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		padding: 0.25rem 0.4rem;
		color: var(--color-text);
		font-weight: 760;
		letter-spacing: -0.025em;
		text-decoration: none;
	}
	.mark {
		display: grid;
		width: 1.75rem;
		height: 1.75rem;
		place-items: center;
		border-radius: 0.5rem;
		color: var(--color-on-accent);
		background: var(--theme-accent-solid, var(--theme-accent));
	}
	.close-sidebar {
		display: none;
	}
	.project-switcher {
		display: grid;
		gap: 0.4rem;
		margin: 1.35rem 0 1rem;
	}
	.project-switcher label {
		margin: 0;
		padding: 0 0.3rem;
		color: var(--color-text-secondary);
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.09em;
		text-transform: uppercase;
	}
	.project-switcher select {
		width: 100%;
		min-height: 2.65rem;
		padding: 0 2rem 0 0.7rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.6rem;
		color: var(--color-text);
		background: var(--theme-canvas);
		font: inherit;
		font-size: 0.9rem;
		font-weight: 720;
	}
	.project-switcher select:focus {
		outline: 3px solid var(--theme-focus);
		border-color: var(--theme-accent);
	}
	.global-quick-add {
		position: relative;
		display: flex;
		align-items: center;
		gap: 0.55rem;
		width: 100%;
		margin: 0 0 0.85rem;
		padding: 0.65rem 0.7rem;
		border: 1px solid color-mix(in srgb, var(--theme-accent) 35%, var(--theme-border));
		border-radius: 0.55rem;
		color: var(--theme-accent);
		background: var(--theme-accent-soft);
		font: inherit;
		font-size: 0.82rem;
		font-weight: 750;
		cursor: pointer;
	}
	.global-command-palette {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		width: 100%;
		margin: 0 0 0.65rem;
		padding: 0.58rem 0.7rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.55rem;
		color: var(--color-text-secondary);
		background: var(--theme-canvas);
		font: inherit;
		font-size: 0.8rem;
		font-weight: 650;
		cursor: pointer;
	}
	.global-command-palette:hover {
		background: var(--theme-hover);
	}
	.global-command-palette svg {
		width: 1rem;
		height: 1rem;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.7;
	}
	.global-command-palette kbd {
		margin-left: auto;
		color: var(--color-text-muted);
		font-family: inherit;
		font-size: 0.62rem;
	}
	.global-quick-add:hover {
		filter: brightness(0.98);
	}
	.primary-navigation {
		display: grid;
		gap: 0.15rem;
	}
	.primary-navigation a,
	.utility-navigation a,
	.empty-projects {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		min-width: 0;
		padding: 0.5rem 0.6rem;
		border: 0;
		border-radius: 0.38rem;
		color: var(--color-text-secondary);
		background: transparent;
		font-size: 0.84rem;
		font-weight: 600;
		text-decoration: none;
	}
	.primary-navigation a,
	.utility-navigation a {
		position: relative;
	}
	.primary-navigation a:hover,
	.utility-navigation a:hover,
	.empty-projects:hover {
		background: var(--theme-hover);
	}
	.primary-navigation a.active,
	.utility-navigation a.active {
		color: var(--theme-accent);
		background: var(--theme-accent-soft);
		font-weight: 750;
	}
	.primary-navigation svg,
	.utility-navigation svg,
	.mobile-bar svg,
	.close-sidebar svg {
		width: 1.2rem;
		height: 1.2rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.6;
	}
	.session {
		display: grid;
		gap: 0.75rem;
		margin-top: auto;
		padding-top: 0.9rem;
		border-top: 1px solid var(--theme-border);
	}
	.appearance-switcher {
		display: grid;
		gap: 0.35rem;
		margin: 0;
		padding: 0;
		border: 0;
	}
	.appearance-switcher legend {
		padding: 0 0.2rem;
		color: var(--color-text-secondary);
		font-size: 0.68rem;
		font-weight: 700;
	}
	.appearance-switcher > div {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 0.15rem;
		padding: 0.18rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.65rem;
		background: var(--theme-control);
	}
	.appearance-switcher button {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.3rem;
		min-width: 0;
		padding: 0.42rem 0.16rem;
		border: 0;
		border-radius: 0.46rem;
		color: var(--color-text-muted);
		background: transparent;
		font: inherit;
		font-size: 0.66rem;
		font-weight: 700;
		text-align: center;
		cursor: pointer;
		transition:
			color 120ms ease,
			background 120ms ease,
			box-shadow 120ms ease;
	}
	.appearance-switcher button svg {
		width: 0.92rem;
		height: 0.92rem;
		flex: 0 0 auto;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.7;
	}
	.appearance-switcher button:hover:not(:disabled) {
		color: var(--color-text);
		background: color-mix(in srgb, var(--theme-surface) 72%, transparent);
	}
	.appearance-switcher button[aria-pressed='true'] {
		color: var(--theme-accent);
		background: var(--theme-surface-elevated);
		box-shadow:
			0 1px 2px color-mix(in srgb, var(--color-text) 12%, transparent),
			inset 0 0 0 1px color-mix(in srgb, var(--theme-accent) 25%, var(--theme-border));
	}
	.appearance-switcher.saving {
		opacity: 0.62;
	}
	.appearance-switcher.saving button {
		cursor: wait;
	}
	.appearance-error {
		margin: -0.35rem 0.4rem 0.55rem;
		color: var(--color-error);
		font-size: 0.7rem;
		line-height: 1.35;
	}
	.utility-navigation {
		display: grid;
		gap: 0.08rem;
	}
	.profile {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		min-width: 0;
		padding: 0.7rem 0.2rem 0;
		border-top: 1px solid var(--theme-border);
	}
	.avatar {
		display: grid;
		width: 1.65rem;
		height: 1.65rem;
		flex: 0 0 auto;
		place-items: center;
		border-radius: 50%;
		color: var(--theme-accent);
		background: var(--theme-accent-soft);
		font-size: 0.68rem;
		font-weight: 800;
	}
	.username {
		display: block;
		min-width: 0;
		overflow: hidden;
		color: var(--color-text-secondary);
		font-size: 0.74rem;
		font-weight: 650;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.logout {
		display: flex;
		align-items: center;
		gap: 0.3rem;
		width: auto;
		margin-left: auto;
		padding: 0.35rem 0.4rem;
		border: 0;
		border-radius: 0.38rem;
		color: var(--color-text-muted);
		background: transparent;
		font: inherit;
		font-size: 0.68rem;
		font-weight: 650;
		cursor: pointer;
	}
	.logout:hover:not(:disabled) {
		color: var(--color-text);
		background: var(--theme-hover);
	}
	.logout:disabled {
		cursor: wait;
		opacity: 0.55;
	}
	.logout svg {
		width: 0.95rem;
		height: 0.95rem;
		flex: 0 0 auto;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.7;
	}
	.content {
		min-width: 0;
	}
	.workspace {
		padding: clamp(3rem, 7vw, 5.5rem) clamp(1.5rem, 6vw, 6rem);
	}
	.mobile-bar,
	.sidebar-backdrop {
		display: none;
	}
	.error {
		margin: 0.6rem 0.5rem 0;
		color: var(--color-error);
		font-size: 0.76rem;
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
	@media (max-width: 48rem) {
		.shell {
			display: block;
		}
		aside {
			position: fixed;
			z-index: 20;
			left: 0;
			width: min(19rem, 86vw);
			transform: translateX(-102%);
			box-shadow: var(--shadow-elevated);
			transition: transform 160ms ease;
		}
		aside.open {
			transform: translateX(0);
		}
		.close-sidebar {
			display: grid;
			width: 2rem;
			height: 2rem;
			place-items: center;
			padding: 0;
			border: 0;
			border-radius: 0.4rem;
			color: var(--color-text-secondary);
			background: transparent;
		}
		.sidebar-backdrop {
			position: fixed;
			z-index: 15;
			inset: 0;
			display: block;
			border: 0;
			background: var(--color-overlay);
			opacity: 0;
			pointer-events: none;
			transition: opacity 160ms ease;
		}
		.sidebar-backdrop.visible {
			opacity: 1;
			pointer-events: auto;
		}
		.mobile-bar {
			position: sticky;
			z-index: 10;
			top: 0;
			display: flex;
			height: 3.5rem;
			padding: 0 1rem;
			border-bottom: 1px solid var(--theme-border);
			background: color-mix(in srgb, var(--theme-canvas) 92%, transparent);
			backdrop-filter: blur(12px);
		}
		.mobile-bar > button {
			display: grid;
			width: 2.2rem;
			height: 2.2rem;
			place-items: center;
			padding: 0;
			border: 0;
			border-radius: 0.45rem;
			color: var(--color-text-secondary);
			background: transparent;
		}
		.mobile-bar .brand {
			margin-right: auto;
			margin-left: 0.4rem;
		}
		.mobile-bar .mark {
			width: 1.5rem;
			height: 1.5rem;
			font-size: 0.8rem;
		}
		.workspace {
			padding: 2.5rem 1.1rem 4rem;
		}
	}
</style>
