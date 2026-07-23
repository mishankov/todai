import { createConnection, createServer, type Server, type Socket } from 'node:net';

import { defineConfig } from 'vitest/config';
import type { ViteDevServer } from 'vite';
import { playwright } from '@vitest/browser-playwright';
import adapter from '@sveltejs/adapter-auto';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	server: {
		proxy: {
			'/api': process.env.TODAI_API_URL ?? 'http://localhost:8080'
		}
	},
	plugins: [
		bunVitestBrowserRpc(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// adapter-auto only supports some environments, see https://svelte.dev/docs/kit/adapter-auto for a list.
			// If your environment is not supported, or you settled on a specific environment, switch out the adapter.
			// See https://svelte.dev/docs/kit/adapters for more information about adapters.
			adapter: adapter()
		})
	],
	test: {
		expect: { requireAssertions: true },
		projects: [
			{
				extends: './vite.config.ts',
				test: {
					name: 'client',
					setupFiles: ['./src/test/bun-browser-interactions.ts'],
					browser: {
						enabled: true,
						api: { host: '127.0.0.1' },
						provider: playwright(),
						instances: [{ browser: 'chromium', headless: true }]
					},
					include: ['src/**/*.svelte.{test,spec}.{js,ts}'],
					exclude: ['src/lib/server/**']
				}
			},

			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			}
		]
	}
});

function bunVitestBrowserRpc() {
	let viteServer: ViteDevServer | undefined;
	let proxy: Server | undefined;
	let proxyStart: Promise<number> | undefined;
	const sockets = new Set<Socket>();

	return {
		name: 'todai:bun-vitest-browser-rpc',
		enforce: 'pre' as const,
		configureServer(server: ViteDevServer) {
			viteServer = server;
		},
		closeBundle: closeProxy,
		async transform(source: string, id: string) {
			if (!id.includes('/@vitest/browser/dist/client.js')) return;
			const marker = 'const PORT = location.port;';
			if (!source.includes(marker)) {
				throw new Error('Unsupported @vitest/browser client: port declaration changed');
			}
			const proxyPort = await (proxyStart ??= startProxy());
			return source.replace(
				marker,
				`const PORT = PAGE_TYPE === "tester" ? "${proxyPort}" : location.port;`
			);
		}
	};

	async function startProxy(): Promise<number> {
		const address = viteServer?.httpServer?.address();
		if (address === null || address === undefined || typeof address === 'string') {
			throw new Error('Vitest browser server is not listening');
		}
		proxy = createServer((downstream) => {
			sockets.add(downstream);
			const upstream = createConnection({ host: '127.0.0.1', port: address.port });
			sockets.add(upstream);
			downstream.once('close', () => sockets.delete(downstream));
			upstream.once('close', () => sockets.delete(upstream));
			downstream.once('error', () => upstream.destroy());
			upstream.once('error', () => downstream.destroy());
			downstream.pipe(upstream).pipe(downstream);
		});
		await new Promise<void>((resolve, reject) => {
			proxy?.once('error', reject);
			proxy?.listen(0, '127.0.0.1', resolve);
		});
		proxy.unref();
		const proxyAddress = proxy.address();
		if (proxyAddress === null || typeof proxyAddress === 'string') {
			throw new Error('Vitest browser RPC proxy is not listening');
		}
		return proxyAddress.port;
	}

	function closeProxy() {
		for (const socket of sockets) socket.destroy();
		proxy?.close();
	}
}
