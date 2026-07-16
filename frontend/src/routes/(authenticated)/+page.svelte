<script lang="ts">
	import { logout } from '$lib/auth/client';
	import {
		completeTask,
		createTask,
		deleteTask,
		reopenTask,
		type Task,
		type TaskUpdate,
		updateTask
	} from '$lib/tasks/client';
	import AppShell from '$lib/components/AppShell.svelte';
	import Inbox from '$lib/components/Inbox.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function signOut() {
		await logout(fetch);
		await goto(resolve('/login'), { invalidateAll: true });
	}

	function create(title: string): Promise<Task> {
		return createTask(fetch, title);
	}

	function complete(taskId: string): Promise<Task> {
		return completeTask(fetch, taskId);
	}

	function reopen(taskId: string): Promise<Task> {
		return reopenTask(fetch, taskId);
	}

	function remove(taskId: string): Promise<void> {
		return deleteTask(fetch, taskId);
	}

	function update(taskId: string, changes: TaskUpdate): Promise<Task> {
		return updateTask(fetch, taskId, changes);
	}
</script>

<svelte:head>
	<title>Todai</title>
	<meta
		name="description"
		content="A personal-first task tracker designed for people and their agents."
	/>
</svelte:head>

<AppShell username={data.user.username} onLogout={signOut}>
	<Inbox initialTasks={data.tasks} {create} {complete} {reopen} {update} {remove} />
</AppShell>
