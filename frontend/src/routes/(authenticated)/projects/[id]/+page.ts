import { getProject } from '$lib/projects/client';
import { getInbox } from '$lib/tasks/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch, params }) => {
	const [project, tasks] = await Promise.all([
		getProject(fetch, params.id),
		getInbox(fetch, params.id, true)
	]);
	return { project, tasks };
}) satisfies PageLoad;
