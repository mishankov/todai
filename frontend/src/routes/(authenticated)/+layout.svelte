<script lang="ts">
	import { browser } from '$app/environment';
	import { afterNavigate, goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import AgentChat from '$lib/agent/AgentChat.svelte';
	import AppearanceController from '$lib/appearance/AppearanceController.svelte';
	import { publishSavedAppearance, type Appearance } from '$lib/appearance/theme';
	import { logout } from '$lib/auth/client';
	import AppShell from '$lib/components/AppShell.svelte';
	import RealtimeSync from '$lib/realtime/RealtimeSync.svelte';
	import { updateSettings } from '$lib/settings/client';
	import GlobalShortcuts from '$lib/shortcuts/GlobalShortcuts.svelte';
	import TaskRouteModal from '$lib/tasks/TaskRouteModal.svelte';
	import { taskNavigationEvent } from '$lib/tasks/navigation';
	import { onMount } from 'svelte';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
	let locationPath = $state(browser ? window.location.pathname : '/');
	let activeProject = $derived.by(() => {
		const match = locationPath.match(/^\/projects\/([^/]+)/);
		if (!match) return undefined;
		return data.projects.find((project) => project.id === decodeURIComponent(match[1]));
	});

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

	async function signOut() {
		await logout(fetch);
		await goto(resolve('/login'), { invalidateAll: true });
	}

	async function saveAppearance(appearance: Appearance) {
		const current = data.settings.settings;
		const updated = await updateSettings(fetch, {
			timezone: current.timezone ?? detectedTimezone(),
			agentModel: current.agentModel,
			agentThinkingEffort: current.agentThinkingEffort,
			appearance,
			version: current.version
		});
		publishSavedAppearance(updated.settings.appearance);
		await invalidateAll();
	}

	function detectedTimezone(): string {
		return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
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
		appearance={data.settings.settings.appearance}
		onAppearanceChange={saveAppearance}
		onLogout={signOut}
		currentPath={locationPath}
	>
		{@render children()}
	</AppShell>

	<GlobalShortcuts {activeProject} projects={data.projects} currentPath={locationPath} />
	<TaskRouteModal projects={data.projects} />
	<RealtimeSync />

	{#if activeProject}
		{#key activeProject.id}
			<AgentChat projectId={activeProject.id} />
			<RealtimeSync projectId={activeProject.id} />
		{/key}
	{/if}
</div>
