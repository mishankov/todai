import { listProjects } from '$lib/projects/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch }) => {
	return { projects: await listProjects(fetch, true) };
}) satisfies PageLoad;
