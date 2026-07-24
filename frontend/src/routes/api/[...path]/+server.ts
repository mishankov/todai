import { env } from '$env/dynamic/private';
import { proxyBackendRequest } from '$lib/server/backend-proxy';
import type { RequestHandler } from './$types';

export const fallback: RequestHandler = (event) =>
	proxyBackendRequest(event, env.TODAI_BACKEND_URL);
