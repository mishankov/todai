import { describe, expect, it, vi } from 'vitest';
import {
	AgentRequestError,
	abortAgentRun,
	createAgentSession,
	getAgentSession,
	postAgentMessage,
	streamAgentEvents,
	type AgentEvent
} from './client';

describe('agent client', () => {
	it('uses the authenticated session and exact agent endpoints', async () => {
		const fetcher = vi
			.fn<typeof fetch>()
			.mockResolvedValueOnce(jsonResponse({ id: 'session-id' }, 201))
			.mockResolvedValueOnce(
				jsonResponse({ session: { id: 'session-id' }, messages: [], runs: [], lastStreamOffset: 0 })
			)
			.mockResolvedValueOnce(
				jsonResponse({ message: { id: 'message-id' }, run: { id: 'run-id' } }, 202)
			)
			.mockResolvedValueOnce(jsonResponse({ id: 'run-id', status: 'aborted' }));

		await createAgentSession(fetcher);
		await getAgentSession(fetcher, 'session/id');
		await postAgentMessage(fetcher, 'session/id', 'Plan my day');
		await abortAgentRun(fetcher, 'run/id');

		expect(fetcher).toHaveBeenNthCalledWith(
			1,
			'/api/agent/sessions',
			expect.objectContaining({ method: 'POST', credentials: 'same-origin' })
		);
		expect(fetcher).toHaveBeenNthCalledWith(
			2,
			'/api/agent/sessions/session%2Fid',
			expect.objectContaining({ credentials: 'same-origin' })
		);
		expect(fetcher).toHaveBeenNthCalledWith(
			3,
			'/api/agent/sessions/session%2Fid/messages',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ message: 'Plan my day' }) })
		);
		expect(fetcher).toHaveBeenNthCalledWith(
			4,
			'/api/agent/runs/run%2Fid/abort',
			expect.objectContaining({ method: 'POST' })
		);
	});

	it('parses fragmented SSE, ignores heartbeats and deduplicates offsets', async () => {
		const first = testEvent({ streamOffset: 8, sequence: 1, type: 'agent.run.started' });
		const duplicate = testEvent({ streamOffset: 8, sequence: 1, type: 'agent.run.started' });
		const second = testEvent({
			streamOffset: 9,
			sequence: 2,
			type: 'agent.message.delta',
			payload: { delta: 'Hello' }
		});
		const body = [
			'retry: 1000\n\n: keep-alive\n\n',
			`id: 8\nevent: ${first.type}\ndata: ${JSON.stringify(first).slice(0, 30)}`,
			`${JSON.stringify(first).slice(30)}\n\nid: 8\ndata: ${JSON.stringify(duplicate)}\n\n`,
			`id: 9\nevent: ${second.type}\ndata: ${JSON.stringify(second)}\n\n`
		];
		const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(textStream(body), {
				status: 200,
				headers: { 'Content-Type': 'text/event-stream' }
			})
		);
		const events: AgentEvent[] = [];
		const cursor = await streamAgentEvents(
			fetcher,
			'session-id',
			7,
			(event) => {
				events.push(event);
			},
			new AbortController().signal
		);

		expect(cursor).toBe(9);
		expect(events.map((event) => event.streamOffset)).toEqual([8, 9]);
		expect(fetcher).toHaveBeenCalledWith(
			'/api/agent/sessions/session-id/events',
			expect.objectContaining({ headers: { Accept: 'text/event-stream', 'Last-Event-ID': '7' } })
		);
	});

	it('returns a typed status error', async () => {
		const fetcher = vi
			.fn<typeof fetch>()
			.mockResolvedValue(new Response('missing', { status: 404 }));

		await expect(getAgentSession(fetcher, 'missing')).rejects.toEqual(
			expect.objectContaining<Partial<AgentRequestError>>({ status: 404 })
		);
	});
});

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function textStream(chunks: string[]): ReadableStream<Uint8Array> {
	const encoder = new TextEncoder();
	return new ReadableStream({
		start(controller) {
			for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
			controller.close();
		}
	});
}

function testEvent(overrides: Partial<AgentEvent> = {}): AgentEvent {
	return {
		streamOffset: 1,
		runId: 'run-id',
		sessionId: 'session-id',
		sequence: 1,
		type: 'agent.run.started',
		occurredAt: '2026-07-18T10:00:00Z',
		payload: {},
		...overrides
	};
}
