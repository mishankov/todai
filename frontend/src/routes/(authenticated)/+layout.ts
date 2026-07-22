import { AuthenticationRequiredError, getCurrentUser } from '$lib/auth/client';
import { listProjects } from '$lib/projects/client';
import { getSettings } from '$lib/settings/client';
import { validPostLoginRedirect } from '$lib/tasks/navigation';
import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const ssr = false;

export const load = (async ({ fetch, url }) => {
	try {
		const user = await getCurrentUser(fetch);
		const [projects, settings] = await Promise.all([listProjects(fetch), getSettings(fetch)]);
		return { user, projects, settings };
	} catch (error) {
		if (error instanceof AuthenticationRequiredError) {
			const requested = `${url.pathname}${url.search}`;
			const returnTo = validPostLoginRedirect(requested) ? requested : '/';
			redirect(
				303,
				returnTo === '/' ? '/login' : `/login?returnTo=${encodeURIComponent(returnTo)}`
			);
		}

		throw error;
	}
}) satisfies LayoutLoad;
