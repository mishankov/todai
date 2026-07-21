import { getProject } from '$lib/projects/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch, params, parent }) => {
	const [{ settings }, project] = await Promise.all([parent(), getProject(fetch, params.id)]);
	return { project, settings };
}) satisfies PageLoad;
