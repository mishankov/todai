import { getProject } from '$lib/projects/client';
import { getProjectTasks } from '$lib/tasks/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch, params, parent }) => {
	const [{ projects }, project, tasks] = await Promise.all([
		parent(),
		getProject(fetch, params.id),
		getProjectTasks(fetch, params.id, true)
	]);
	return { projects, project, tasks };
}) satisfies PageLoad;
