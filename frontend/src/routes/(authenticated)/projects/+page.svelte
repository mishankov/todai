<script lang="ts">
	import ProjectList from '$lib/projects/ProjectList.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Projects — Todai</title></svelte:head>

<ProjectList initialProjects={data.projects} contextualProjectId={data.contextualProjectId} />
