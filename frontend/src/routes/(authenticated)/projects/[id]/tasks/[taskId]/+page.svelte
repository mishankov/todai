<script lang="ts">
	import TasksPage from '../+page.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<TasksPage {data} params={{ id: data.project.id }} />
