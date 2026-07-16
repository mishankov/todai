import { getInbox } from '$lib/tasks/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch }) => {
	return { tasks: await getInbox(fetch, true) };
}) satisfies PageLoad;
