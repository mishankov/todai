import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load = (() => {
	error(404, 'Page not found');
}) satisfies PageLoad;
