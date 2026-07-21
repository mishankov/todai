<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import AgentChat from '$lib/agent/AgentChat.svelte';
	import { logout } from '$lib/auth/client';
	import AppShell from '$lib/components/AppShell.svelte';
	import RealtimeSync from '$lib/realtime/RealtimeSync.svelte';
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
	<AppShell
		username={data.user.username}
		projects={data.projects}
		{activeProject}
		onLogout={signOut}
		currentPath={page.url.pathname}
	>
		{@render children()}
	</AppShell>

	{#if activeProject}
		{#key activeProject.id}
			<AgentChat projectId={activeProject.id} />
			<RealtimeSync projectId={activeProject.id} />
		{/key}
	{/if}
</div>

<style>
	.project-context {
		--theme-accent: #2d6540;
		--theme-accent-soft: #dfeadf;
		--theme-sidebar: #f1f5ef;
		--theme-canvas: #fbfcfa;
		--theme-border: #dfe5dc;
		--theme-hover: #e6ece4;
		--theme-focus: rgb(45 101 64 / 16%);
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
</style>
