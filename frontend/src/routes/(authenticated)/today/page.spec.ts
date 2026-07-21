import { describe, expect, it, vi } from 'vitest';
import { load } from '../projects/[id]/today/+page';

describe('Project Today page loader', () => {
	it('uses the account timezone while scoping tasks to the project', async () => {
		const fetcher = vi.fn(async (input: string | URL | Request) => {
			const path = String(input);
			const body = path.includes('/api/projects/')
				? {
						id: 'work/id',
						name: 'Work',
						layout: 'list',
						colorTheme: 'ocean',
						agentModel: 'gpt-default',
						agentThinkingEffort: 'medium',
						position: 1024,
						version: 1,
						archivedAt: null,
						createdAt: '2026-07-21T10:00:00Z',
						updatedAt: '2026-07-21T10:00:00Z',
						lastModifiedBy: 'user-id'
					}
				: { tasks: [] };
			return new Response(JSON.stringify(body), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			});
		}) as unknown as typeof fetch;

		await load({
			fetch: fetcher,
			params: { id: 'work/id' },
			parent: async () => ({ settings: { settings: { timezone: 'Asia/Tokyo' } } })
		} as never);

		expect(fetcher).toHaveBeenCalledWith(
			'/api/views/projects/work%2Fid/today?timezone=Asia%2FTokyo&include_completed=true',
			expect.any(Object)
		);
	});
});
