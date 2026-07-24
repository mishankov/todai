import type { RequestEvent } from '@sveltejs/kit';

type ProxyEvent = Pick<RequestEvent, 'getClientAddress' | 'request' | 'url'>;
type StreamingRequestInit = RequestInit & { duplex?: 'half' };

const hopByHopHeaders = [
	'connection',
	'keep-alive',
	'proxy-authenticate',
	'proxy-authorization',
	'proxy-connection',
	'te',
	'trailer',
	'transfer-encoding',
	'upgrade'
];

export async function proxyBackendRequest(
	event: ProxyEvent,
	rawBackendURL: string | undefined,
	fetcher: typeof fetch = fetch
): Promise<Response> {
	const backendURL = backendOrigin(rawBackendURL);
	const upstreamURL = new URL(event.url.pathname + event.url.search, backendURL);
	const headers = proxyRequestHeaders(event);
	const init: StreamingRequestInit = {
		method: event.request.method,
		headers,
		redirect: 'manual',
		signal: event.request.signal
	};
	if (event.request.body) {
		init.body = event.request.body;
		init.duplex = 'half';
	}

	try {
		const upstream = await fetcher(upstreamURL, init);
		return new Response(upstream.body, {
			status: upstream.status,
			statusText: upstream.statusText,
			headers: withoutHopByHopHeaders(upstream.headers)
		});
	} catch (error) {
		logProxyError(error, event.request.method);
		return new Response('Bad Gateway\n', {
			status: 502,
			headers: { 'Content-Type': 'text/plain; charset=utf-8' }
		});
	}
}

export function backendOrigin(rawValue: string | undefined): URL {
	const raw = rawValue || 'http://backend:8080';
	let parsed;
	try {
		parsed = new URL(raw);
	} catch {
		throw new Error('TODAI_BACKEND_URL must be an absolute HTTP(S) URL');
	}
	if (
		(parsed.protocol !== 'http:' && parsed.protocol !== 'https:') ||
		!parsed.hostname ||
		parsed.username ||
		parsed.password ||
		parsed.pathname !== '/' ||
		parsed.search ||
		parsed.hash
	) {
		throw new Error('TODAI_BACKEND_URL must be an HTTP(S) origin without credentials or a path');
	}
	return parsed;
}

function proxyRequestHeaders(event: ProxyEvent): Headers {
	const headers = withoutHopByHopHeaders(event.request.headers);
	const forwardedHost = headers.get('x-forwarded-host') || headers.get('host') || event.url.host;
	const forwardedProtocol =
		headers.get('x-forwarded-proto') || event.url.protocol.replace(/:$/, '');
	const priorForwardedFor = headers.get('x-forwarded-for');
	const remoteAddress = event.getClientAddress();

	headers.set('accept-encoding', 'identity');
	headers.set('host', forwardedHost);
	headers.set('x-forwarded-host', forwardedHost);
	headers.set('x-forwarded-proto', forwardedProtocol);
	headers.set('x-forwarded-for', [priorForwardedFor, remoteAddress].filter(Boolean).join(', '));
	return headers;
}

function withoutHopByHopHeaders(source: Headers): Headers {
	const headers = new Headers(source);
	const connectionHeaders =
		headers
			.get('connection')
			?.split(',')
			.map((header) => header.trim())
			.filter(Boolean) ?? [];
	for (const header of [...hopByHopHeaders, ...connectionHeaders]) {
		headers.delete(header);
	}
	return headers;
}

function logProxyError(error: unknown, method: string): void {
	console.error(
		JSON.stringify({
			time: new Date().toISOString(),
			level: 'ERROR',
			message: 'backend proxy request failed',
			component: 'frontend',
			error: error instanceof Error ? error.message : 'unknown error',
			method
		})
	);
}
