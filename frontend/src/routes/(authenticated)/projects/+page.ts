import { listProjects } from '$lib/projects/client';
import { activeProjectFromLocation } from '$lib/projects/navigation';
import type { PageLoad } from './$types';

export const load = (async ({ fetch, url }) => {
	const projects = await listProjects(fetch, true);
	return {
		projects,
		contextualProjectId: activeProjectFromLocation(projects, url.pathname, url.search)?.id
	};
}) satisfies PageLoad;
