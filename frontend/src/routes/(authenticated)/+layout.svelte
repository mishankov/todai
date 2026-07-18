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

<AppShell
	username={data.user.username}
	projects={data.projects}
	onLogout={signOut}
	currentPath={page.url.pathname}
>
	{@render children()}
</AppShell>

<AgentChat />
<RealtimeSync />
