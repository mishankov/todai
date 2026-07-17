import { getActivity } from '$lib/activity/client';
import type { PageLoad } from './$types';

export const load = (async ({ fetch }) => {
	return { events: await getActivity(fetch) };
}) satisfies PageLoad;
