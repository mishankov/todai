<script lang="ts">
	import ActivityFeed from '$lib/activity/ActivityFeed.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Activity · {data.project.name} — Todai</title></svelte:head>

<ActivityFeed initialEvents={data.events} />
