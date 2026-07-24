import { describe, expect, it, vi } from 'vitest';
import { backendOrigin, proxyBackendRequest } from './backend-proxy';

describe('backend proxy', () => {
	it('forwards the request and preserves response cookies and body streaming', async () => {
		let forwardedBody = '';
		const upstreamBody = new ReadableStream({
			start(controller) {
				controller.enqueue(new TextEncoder().encode('event: completed\n\n'));
				controller.close();
			}
		});
		const fetcher = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
			forwardedBody = await new Response(init?.body).text();
			const headers = new Headers({ 'Content-Type': 'text/event-stream' });
			headers.append('Set-Cookie', 'todai_session=token; Path=/; HttpOnly');
			headers.append('Set-Cookie', 'theme=dark; Path=/');
			expect(String(input)).toBe('http://backend:8080/api/agent/events?after=7');
			expect(new Headers(init?.headers).get('cookie')).toBe('todai_session=token');
			expect(new Headers(init?.headers).get('host')).toBe('todai.example');
			expect(new Headers(init?.headers).get('x-forwarded-host')).toBe('todai.example');
			expect(new Headers(init?.headers).get('x-forwarded-proto')).toBe('https');
			expect(new Headers(init?.headers).get('x-forwarded-for')).toBe('192.0.2.10');
			return new Response(upstreamBody, { status: 202, headers });
		}) as typeof fetch;
		const request = new Request('https://todai.example/api/agent/events?after=7', {
			method: 'POST',
			headers: { Cookie: 'todai_session=token' },
			body: 'message'
		});

		const response = await proxyBackendRequest(
			{
				request,
				url: new URL(request.url),
				getClientAddress: () => '192.0.2.10'
			},
			'http://backend:8080',
			fetcher
		);

		expect(response.status).toBe(202);
		expect(response.body).toBe(upstreamBody);
		expect(response.headers.getSetCookie()).toEqual([
			'todai_session=token; Path=/; HttpOnly',
			'theme=dark; Path=/'
		]);
		expect(forwardedBody).toBe('message');
	});

	it('returns a structured Bad Gateway response when the backend is unavailable', async () => {
		const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined);
		const request = new Request('http://todai.example/api/tasks');

		const response = await proxyBackendRequest(
			{
				request,
				url: new URL(request.url),
				getClientAddress: () => '192.0.2.10'
			},
			'http://backend:8080',
			vi.fn(async () => {
				throw new Error('connection refused');
			})
		);

		expect(response.status).toBe(502);
		await expect(response.text()).resolves.toBe('Bad Gateway\n');
		expect(consoleError).toHaveBeenCalledOnce();
		consoleError.mockRestore();
	});

	it.each([
		'not-a-url',
		'file:///tmp/backend.sock',
		'http://user:password@backend:8080',
		'http://backend:8080/base',
		'http://backend:8080/?token=secret'
	])('rejects unsafe backend origin %s', (value) => {
		expect(() => backendOrigin(value)).toThrow(/TODAI_BACKEND_URL/);
	});
});
