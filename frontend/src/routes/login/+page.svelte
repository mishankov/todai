<script lang="ts">
	import LoginForm from '$lib/components/LoginForm.svelte';
	import SystemAppearanceController from '$lib/appearance/SystemAppearanceController.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';

	async function openWorkspace() {
		await goto(resolve('/'), { invalidateAll: true });
	}
</script>

<svelte:head>
	<title>Sign in · Todai</title>
	<meta name="description" content="Sign in to your Todai workspace." />
</svelte:head>

<SystemAppearanceController forceSystem />
<LoginForm onAuthenticated={openWorkspace} />
