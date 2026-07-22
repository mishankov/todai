<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import AgentChat from '$lib/agent/AgentChat.svelte';
	import AppearanceController from '$lib/appearance/AppearanceController.svelte';
	import { logout } from '$lib/auth/client';
	import AppShell from '$lib/components/AppShell.svelte';
	import RealtimeSync from '$lib/realtime/RealtimeSync.svelte';
	import GlobalShortcuts from '$lib/shortcuts/GlobalShortcuts.svelte';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
	let activeProject = $derived.by(() => {
		const match = page.url.pathname.match(/^\/projects\/([^/]+)/);
		if (!match) return undefined;
		return data.projects.find((project) => project.id === decodeURIComponent(match[1]));
	});

	async function signOut() {
		await logout(fetch);
		await goto(resolve('/login'), { invalidateAll: true });
	}
</script>

<svelte:head>
	<meta
		name="description"
		content="A personal-first task tracker designed for people and their agents."
	/>
</svelte:head>

<div class={`project-context theme-${activeProject?.colorTheme ?? 'sage'}`}>
	<AppearanceController appearance={data.settings.settings.appearance} />
	<AppShell
		username={data.user.username}
		projects={data.projects}
		{activeProject}
		onLogout={signOut}
		currentPath={page.url.pathname}
	>
		{@render children()}
	</AppShell>

	<GlobalShortcuts {activeProject} projects={data.projects} currentPath={page.url.pathname} />
	<RealtimeSync />

	{#if activeProject}
		{#key activeProject.id}
			<AgentChat projectId={activeProject.id} />
			<RealtimeSync projectId={activeProject.id} />
		{/key}
	{/if}
</div>
