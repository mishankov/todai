import { getToday } from '$lib/tasks/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch }) => {
	const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	return { tasks: await getToday(fetch, timezone, true) };
}) satisfies PageLoad;
