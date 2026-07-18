import { getToday } from '$lib/tasks/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch, parent }) => {
	const { settings } = await parent();
	const timezone = settings.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
	return { tasks: await getToday(fetch, timezone, true) };
}) satisfies PageLoad;
