import { AuthenticationRequiredError, getCurrentUser } from '$lib/auth/client';
import { listProjects } from '$lib/projects/client';
import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const ssr = false;

export const load = (async ({ fetch }) => {
	try {
		const user = await getCurrentUser(fetch);
		const projects = await listProjects(fetch);
		return { user, projects };
	} catch (error) {
		if (error instanceof AuthenticationRequiredError) {
			redirect(303, '/login');
		}

		throw error;
	}
}) satisfies LayoutLoad;
