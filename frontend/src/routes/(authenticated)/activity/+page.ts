import { initialProjectPath } from '$lib/projects/navigation';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load = (async ({ parent }) => {
	const { projects } = await parent();
	redirect(303, initialProjectPath(projects, '/activity'));
}) satisfies PageLoad;
