import { getActivity } from '$lib/activity/client';
import { getProject } from '$lib/projects/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch, params }) => {
	const [project, events] = await Promise.all([
		getProject(fetch, params.id),
		getActivity(fetch, params.id)
	]);
	return { project, events };
}) satisfies PageLoad;
