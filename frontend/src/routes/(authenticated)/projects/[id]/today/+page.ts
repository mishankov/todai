import { getProject, listProjectSections } from '$lib/projects/client';
import { getToday } from '$lib/tasks/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch, params, parent }) => {
	const { settings } = await parent();
	const timezone = settings.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
	const [project, sections, tasks] = await Promise.all([
		getProject(fetch, params.id),
		listProjectSections(fetch, params.id),
		getToday(fetch, params.id, timezone, true)
	]);
	return { project, sections, tasks };
}) satisfies PageLoad;
