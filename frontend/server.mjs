import http from 'node:http';
import https from 'node:https';

import { handler } from './build/handler.js';

const backendURL = backendOrigin(process.env.TODAI_BACKEND_URL);
const host = process.env.HOST || '0.0.0.0';
const port = listenPort(process.env.PORT);
const server = http.createServer((request, response) => {
	const pathname = requestPathname(request.url);
	if (pathname === '/health') {
		response.writeHead(200, { 'Content-Type': 'text/plain; charset=utf-8' });
		response.end('ok\n');
		return;
	}
	if (pathname === '/internal/tools' || pathname.startsWith('/internal/tools/')) {
		response.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
		response.end('Not Found\n');
		return;
	}
	if (pathname === '/api' || pathname.startsWith('/api/')) {
		proxyAPI(request, response);
		return;
	}

	try {
		Promise.resolve(handler(request, response)).catch((error) => {
			handleResponseError(
				response,
				error,
				500,
				'frontend request failed',
				'Internal Server Error\n'
			);
		});
	} catch (error) {
		handleResponseError(response, error, 500, 'frontend request failed', 'Internal Server Error\n');
	}
});

server.listen(port, host, () => {
	log('INFO', 'frontend server started', { address: `${host}:${port}` });
});

for (const signal of ['SIGINT', 'SIGTERM']) {
	process.once(signal, () => {
		log('INFO', 'frontend server stopping', { signal });
		server.close((error) => {
			if (error) {
				log('ERROR', 'frontend server shutdown failed', { error: errorMessage(error) });
				process.exitCode = 1;
			}
		});
		setTimeout(() => {
			log('ERROR', 'frontend server shutdown timed out');
			process.exit(1);
		}, 25_000).unref();
	});
}

function proxyAPI(request, response) {
	const headers = { ...request.headers };
	delete headers.connection;
	delete headers['proxy-connection'];
	headers.host = request.headers.host || backendURL.host;
	headers['x-forwarded-host'] = firstHeader(request.headers['x-forwarded-host']) || headers.host;
	headers['x-forwarded-proto'] =
		firstHeader(request.headers['x-forwarded-proto']) ||
		(request.socket.encrypted ? 'https' : 'http');
	const priorForwardedFor = firstHeader(request.headers['x-forwarded-for']);
	const remoteAddress = request.socket.remoteAddress || '';
	headers['x-forwarded-for'] = [priorForwardedFor, remoteAddress].filter(Boolean).join(', ');

	const transport = backendURL.protocol === 'https:' ? https : http;
	const upstream = transport.request(
		{
			protocol: backendURL.protocol,
			hostname: backendURL.hostname,
			port: backendURL.port,
			method: request.method,
			path: request.url,
			headers
		},
		(upstreamResponse) => {
			response.writeHead(upstreamResponse.statusCode || 502, upstreamResponse.headers);
			upstreamResponse.on('error', (error) => {
				handleResponseError(response, error, 502, 'backend proxy response failed', 'Bad Gateway\n');
			});
			upstreamResponse.pipe(response);
		}
	);
	upstream.on('error', (error) => {
		handleResponseError(response, error, 502, 'backend proxy request failed', 'Bad Gateway\n', {
			method: request.method
		});
	});
	request.on('aborted', () => upstream.destroy());
	response.on('close', () => {
		if (!response.writableFinished) {
			upstream.destroy();
		}
	});
	request.pipe(upstream);
}

function backendOrigin(rawValue) {
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

function listenPort(rawValue) {
	const parsed = Number.parseInt(rawValue || '8080', 10);
	if (!Number.isInteger(parsed) || parsed < 1 || parsed > 65_535) {
		throw new Error('PORT must be an integer between 1 and 65535');
	}
	return parsed;
}

function requestPathname(rawValue) {
	try {
		return new URL(rawValue || '/', 'http://localhost').pathname;
	} catch {
		return '/';
	}
}

function firstHeader(value) {
	return Array.isArray(value) ? value[0] : value;
}

function errorMessage(error) {
	return error instanceof Error ? error.message : 'unknown error';
}

function handleResponseError(response, error, status, message, responseBody, attributes = {}) {
	log('ERROR', message, { error: errorMessage(error), ...attributes });
	if (response.headersSent) {
		response.destroy(error instanceof Error ? error : undefined);
		return;
	}
	response.writeHead(status, { 'Content-Type': 'text/plain; charset=utf-8' });
	response.end(responseBody);
}

function log(level, message, attributes = {}) {
	console.log(
		JSON.stringify({
			time: new Date().toISOString(),
			level,
			message,
			component: 'frontend',
			...attributes
		})
	);
}
