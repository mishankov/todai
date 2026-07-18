import { describe, expect, it, vi } from 'vitest';
import { load } from './+page';

describe('Today page loader', () => {
	it('uses the saved user timezone', async () => {
		const fetcher = vi.fn(
			async () =>
				new Response(JSON.stringify({ tasks: [] }), {
					status: 200,
					headers: { 'Content-Type': 'application/json' }
				})
		) as unknown as typeof fetch;

		await load({
			fetch: fetcher,
			parent: async () => ({ settings: { settings: { timezone: 'Asia/Tokyo' } } })
		} as never);

		expect(fetcher).toHaveBeenCalledWith(
			'/api/views/today?timezone=Asia%2FTokyo&include_completed=true',
			expect.any(Object)
		);
	});
});
