import { AuthenticationRequiredError, getCurrentUser } from '$lib/auth/client';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const ssr = false;

export const load = (async ({ fetch }) => {
	try {
		await getCurrentUser(fetch);
		redirect(303, '/');
	} catch (error) {
		if (error instanceof AuthenticationRequiredError) {
			return {};
		}

		throw error;
	}
}) satisfies PageLoad;
