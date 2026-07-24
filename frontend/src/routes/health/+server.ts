import type { RequestHandler } from './$types';

export const fallback: RequestHandler = () =>
	new Response('ok\n', {
		status: 200,
		headers: { 'Content-Type': 'text/plain; charset=utf-8' }
	});
