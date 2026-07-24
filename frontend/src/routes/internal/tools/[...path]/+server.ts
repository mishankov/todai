import type { RequestHandler } from './$types';

export const fallback: RequestHandler = () =>
	new Response('Not Found\n', {
		status: 404,
		headers: { 'Content-Type': 'text/plain; charset=utf-8' }
	});
