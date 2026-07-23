import { AuthenticationRequiredError, getCurrentUser } from '$lib/auth/client';
import { validPostLoginRedirect } from '$lib/tasks/navigation';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const ssr = false;

export const load = (async ({ fetch, url }) => {
	const requested = url.searchParams.get('returnTo');
	const returnTo = validPostLoginRedirect(requested) ? requested : '/';
	try {
		await getCurrentUser(fetch);
		redirect(303, returnTo);
	} catch (error) {
		if (error instanceof AuthenticationRequiredError) {
			return { returnTo };
		}

		throw error;
	}
}) satisfies PageLoad;
