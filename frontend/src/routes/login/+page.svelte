<script lang="ts">
	import LoginForm from '$lib/components/LoginForm.svelte';
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

<LoginForm onAuthenticated={openWorkspace} />
