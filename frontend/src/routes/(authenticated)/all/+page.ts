import { getAllTasks } from '$lib/tasks/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch }) => {
	return { tasks: await getAllTasks(fetch, true) };
}) satisfies PageLoad;
