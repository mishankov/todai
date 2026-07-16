import { AuthenticationRequiredError, getCurrentUser } from '$lib/auth/client';
import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const ssr = false;

export const load = (async ({ fetch }) => {
	try {
		return { user: await getCurrentUser(fetch) };
	} catch (error) {
		if (error instanceof AuthenticationRequiredError) {
			redirect(303, '/login');
		}

		throw error;
	}
}) satisfies LayoutLoad;
