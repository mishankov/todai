import { describe, expect, it, vi } from 'vitest';
import { searchTasks, TaskSearchError } from './client';

describe('task palette search client', () => {
	it('sends an authenticated project-scoped limited query and forwards cancellation', async () => {
		const controller = new AbortController();
		const fetcher = vi.fn(async (...request: [string | URL | Request, RequestInit?]) => {
			void request;
			return new Response(JSON.stringify({ tasks: [] }), { status: 200 });
		});

		await searchTasks(fetcher as typeof fetch, 'milk & tea', 'work/id', {
			status: 'active',
			limit: 7,
			signal: controller.signal
		});

		const [url, init] = fetcher.mock.calls[0];
		expect(String(url)).toContain('/api/tasks/search?');
		expect(String(url)).toContain('query=milk+%26+tea');
		expect(String(url)).toContain('project_id=work%2Fid');
		expect(String(url)).toContain('status=active');
		expect(init).toMatchObject({ credentials: 'same-origin', signal: controller.signal });
	});

	it('maps unsuccessful responses to a search-specific error', async () => {
		const fetcher = vi.fn(async () => new Response(null, { status: 500 }));
		await expect(searchTasks(fetcher as typeof fetch, 'milk', 'work')).rejects.toBeInstanceOf(
			TaskSearchError
		);
	});
});
