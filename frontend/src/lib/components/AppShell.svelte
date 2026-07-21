<script lang="ts">
	/* Project links are resolved centrally by projectHref; the lint rule only recognizes inline calls. */
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import type { Project, ProjectColorTheme } from '$lib/projects/client';
	import { recordProjectPath, rememberedProjectPath } from '$lib/projects/navigation';
	import { commandPaletteRequestEvent, quickAddRequestEvent } from '$lib/shortcuts/events';
	import {
		ariaShortcut,
		formatShortcut,
		isApplePlatform,
		shortcutCommand
	} from '$lib/shortcuts/registry';
	import type { Snippet } from 'svelte';

	interface Props {
		username: string;
		projects?: Project[];
		activeProject?: Project;
		onLogout: () => Promise<void>;
		currentPath?: string;
		children?: Snippet;
	}

	let {
		username,
		projects = [],
		activeProject,
		onLogout,
		currentPath = '/',
		children
	}: Props = $props();
	let signingOut = $state(false);
	let errorMessage = $state('');
	let sidebarOpen = $state(false);
	let theme = $derived<ProjectColorTheme>(activeProject?.colorTheme ?? 'sage');
	let projectBase = $derived(activeProject ? `/projects/${activeProject.id}` : '/projects');
	let applePlatform = $state(browser && isApplePlatform(window.navigator.platform));
	let quickAddCommand = shortcutCommand('quick-add');
	let quickAddLabel = $derived(formatShortcut(quickAddCommand, applePlatform));
	let paletteCommand = shortcutCommand('command-palette');
	let paletteLabel = $derived(formatShortcut(paletteCommand, applePlatform));

	$effect(() => {
		if (browser) applePlatform = isApplePlatform(window.navigator.platform);
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
				title={`Create task (${quickAddLabel})`}
				aria-label={`Create task (${quickAddLabel})`}
				aria-keyshortcuts={ariaShortcut(quickAddCommand, applePlatform)}
				onclick={openQuickAdd}
			>
				<span aria-hidden="true">＋</span> Create task <kbd>{quickAddLabel}</kbd>
			</button>
			<nav class="primary-navigation" aria-label="Project navigation">
				<a
					href={projectHref(activeProject.id, '/overview')}
					class:active={isActive('/overview')}
					aria-current={isActive('/overview') ? 'page' : undefined}
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
			{#if activeProject}
				<a
					href={projectHref(activeProject.id, '/settings')}
					class:active={isActive('/settings')}
					aria-current={isActive('/settings') ? 'page' : undefined}
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
				onclick={closeSidebar}><span>Manage projects</span></a
			>
			<a
				href={resolve('/settings')}
				class:active={currentPath === '/settings'}
				aria-current={currentPath === '/settings' ? 'page' : undefined}
				onclick={closeSidebar}><span>Account settings</span></a
			>
			<div class="profile"><span class="username">{username}</span></div>
			<button type="button" disabled={signingOut} onclick={signOut}>Log out</button>
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
		--theme-accent: #2d6540;
		--theme-accent-soft: #dfeadf;
		--theme-sidebar: #f1f5ef;
		--theme-canvas: #fbfcfa;
		--theme-border: #dfe5dc;
		--theme-hover: #e6ece4;
		--theme-focus: rgb(45 101 64 / 16%);
		display: grid;
		grid-template-columns: 17rem minmax(0, 1fr);
		min-height: 100vh;
		background: var(--theme-canvas);
	}
	.theme-ocean {
		--theme-accent: #28638c;
		--theme-accent-soft: #dceaf3;
		--theme-sidebar: #eef5f8;
		--theme-canvas: #fbfdfe;
		--theme-border: #d8e4ea;
		--theme-hover: #e3eef3;
		--theme-focus: rgb(40 99 140 / 16%);
	}
	.theme-plum {
		--theme-accent: #6b477d;
		--theme-accent-soft: #ebe1ef;
		--theme-sidebar: #f5f0f6;
		--theme-canvas: #fdfbfe;
		--theme-border: #e5dce8;
		--theme-hover: #eee6f1;
		--theme-focus: rgb(107 71 125 / 16%);
	}
	.theme-sand {
		--theme-accent: #8a643f;
		--theme-accent-soft: #eee3d7;
		--theme-sidebar: #f7f2eb;
		--theme-canvas: #fefcf9;
		--theme-border: #e7ddd1;
		--theme-hover: #efe7dc;
		--theme-focus: rgb(138 100 63 / 16%);
	}
	.theme-rose {
		--theme-accent: #94505e;
		--theme-accent-soft: #f1dfe3;
		--theme-sidebar: #f8f0f2;
		--theme-canvas: #fefbfc;
		--theme-border: #eadce0;
		--theme-hover: #f2e5e8;
		--theme-focus: rgb(148 80 94 / 16%);
	}
	.theme-graphite {
		--theme-accent: #52565d;
		--theme-accent-soft: #e3e5e7;
		--theme-sidebar: #f1f2f3;
		--theme-canvas: #fcfcfc;
		--theme-border: #dfe1e3;
		--theme-hover: #e8e9eb;
		--theme-focus: rgb(82 86 93 / 16%);
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
		color: #292927;
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
		color: #fff;
		background: var(--theme-accent);
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
		color: #74746f;
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
		color: #292927;
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
		color: #555650;
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
		color: #777873;
		font-family: inherit;
		font-size: 0.62rem;
	}
	.global-quick-add:hover {
		filter: brightness(0.98);
	}
	.global-quick-add kbd {
		margin-left: auto;
		color: #6b6b66;
		font-family: inherit;
		font-size: 0.65rem;
		font-weight: 650;
	}
	.primary-navigation {
		display: grid;
		gap: 0.15rem;
	}
	.primary-navigation a,
	.session a,
	.session button,
	.empty-projects {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		min-width: 0;
		padding: 0.5rem 0.6rem;
		border: 0;
		border-radius: 0.38rem;
		color: #53534f;
		background: transparent;
		font-size: 0.84rem;
		font-weight: 600;
		text-decoration: none;
	}
	.primary-navigation a:hover,
	.session a:hover,
	.session button:hover:not(:disabled),
	.empty-projects:hover {
		background: var(--theme-hover);
	}
	.primary-navigation a.active,
	.session a.active {
		color: var(--theme-accent);
		background: var(--theme-accent-soft);
		font-weight: 750;
	}
	.primary-navigation svg,
	.session svg,
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
		gap: 0.1rem;
		margin-top: auto;
		padding-top: 1.5rem;
	}
	.session button {
		width: 100%;
		cursor: pointer;
		text-align: left;
	}
	.session button:disabled {
		cursor: wait;
		opacity: 0.55;
	}
	.profile {
		min-width: 0;
		padding: 0.65rem 0.6rem 0.2rem;
	}
	.username {
		display: block;
		overflow: hidden;
		color: #777;
		font-size: 0.74rem;
		text-overflow: ellipsis;
		white-space: nowrap;
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
		color: #b83f34;
		font-size: 0.76rem;
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
			box-shadow: 1rem 0 3rem rgb(30 29 27 / 14%);
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
			color: #66625f;
			background: transparent;
		}
		.sidebar-backdrop {
			position: fixed;
			z-index: 15;
			inset: 0;
			display: block;
			border: 0;
			background: rgb(23 22 20 / 30%);
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
			color: #4e4d49;
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
