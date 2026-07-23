<script lang="ts">
	/* The destination was validated as a safe internal path by the page loader. */
	/* eslint-disable svelte/no-navigation-without-resolve */
	import LoginForm from '$lib/components/LoginForm.svelte';
	import SystemAppearanceController from '$lib/appearance/SystemAppearanceController.svelte';
	import { goto } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function openWorkspace() {
		await goto(data.returnTo, { invalidateAll: true });
	}
</script>

<svelte:head>
	<title>Sign in · Todai</title>
	<meta name="description" content="Sign in to your Todai workspace." />
</svelte:head>

<SystemAppearanceController forceSystem />
<LoginForm onAuthenticated={openWorkspace} />
