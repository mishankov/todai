import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import AgentChat from './AgentChat.svelte';
import {
	AgentRequestError,
	type AgentAPI,
	type AgentConversation,
	type AgentEvent
} from './client';

describe('AgentChat', () => {
	it('restores a chat, streams an answer and shows task-tool activity', async () => {
		const stream = controllableStream();
		const empty = testConversation();
		const completed = testConversation({
			messages: [userMessage(), assistantMessage('Inbox is clear.')],
			runs: [testRun({ status: 'completed', completedAt: '2026-07-18T10:00:02Z' })],
			lastStreamOffset: 4
		});
		const api = testAPI({
			getSession: vi.fn().mockResolvedValueOnce(empty).mockResolvedValueOnce(completed),
			postMessage: vi.fn().mockResolvedValue({ message: userMessage(), run: testRun() }),
			streamEvents: stream.streamEvents
		});

		render(AgentChat, { api, storage: testStorage('session-id') });
		await page.getByRole('button', { name: 'Open assistant' }).click();
		await expect
			.element(page.getByRole('heading', { name: 'What would you like to get done?' }))
			.toBeVisible();
		await page.getByLabelText('Message the assistant').fill('Is my inbox clear?');
		await page.getByRole('button', { name: 'Send' }).click();
		await expect.element(page.getByText('Is my inbox clear?', { exact: true })).toBeVisible();

		await stream.emit(testEvent({ type: 'agent.run.started', streamOffset: 1, payload: {} }));
		await stream.emit(
			testEvent({
				type: 'agent.tool.started',
				streamOffset: 2,
				sequence: 2,
				payload: { toolCallId: 'call-id', toolName: 'task_search' }
			})
		);
		await stream.emit(
			testEvent({
				type: 'agent.message.delta',
				streamOffset: 3,
				sequence: 3,
				payload: { messageId: 'message-id', delta: 'Inbox is clear.' }
			})
		);
		await expect.element(page.getByText('Searching tasks…', { exact: true })).toBeVisible();
		await expect.element(page.getByText('Inbox is clear.', { exact: true })).toBeVisible();

		await stream.emit(
			testEvent({ type: 'agent.run.completed', streamOffset: 4, sequence: 4, payload: {} })
		);
		await expect.element(page.getByLabelText('Message the assistant')).toBeEnabled();
		expect(api.postMessage).toHaveBeenCalledWith('session-id', { message: 'Is my inbox clear?' });
	});

	it('replaces a missing stored session', async () => {
		const storage = testStorage('missing-session');
		const api = testAPI({
			getSession: vi.fn().mockRejectedValue(new AgentRequestError('Could not load the chat.', 404)),
			createSession: vi.fn().mockResolvedValue(testSession({ id: 'new-session' }))
		});

		render(AgentChat, { api, storage });
		await page.getByRole('button', { name: 'Open assistant' }).click();

		await expect
			.element(page.getByRole('heading', { name: 'What would you like to get done?' }))
			.toBeVisible();
		expect(storage.removeItem).toHaveBeenCalledWith('todai.agent.session-id');
		expect(storage.setItem).toHaveBeenCalledWith('todai.agent.session-id', 'new-session');
	});

	it('stops an active run', async () => {
		const api = testAPI({
			getSession: vi.fn().mockResolvedValue(testConversation({ runs: [testRun()] })),
			abortRun: vi.fn().mockResolvedValue(testRun({ status: 'aborted' }))
		});

		render(AgentChat, { api, storage: testStorage('session-id') });
		await page.getByRole('button', { name: 'Open assistant' }).click();
		await page.getByRole('button', { name: 'Stop' }).click();

		expect(api.abortRun).toHaveBeenCalledWith('run-id');
		await expect.element(page.getByRole('button', { name: 'Stopping…' })).toBeDisabled();
	});

	it('loads lazily and keeps the conversation when reopened', async () => {
		const api = testAPI();

		render(AgentChat, { api, storage: testStorage('session-id') });
		expect(api.getSession).not.toHaveBeenCalled();

		await page.getByRole('button', { name: 'Open assistant' }).click();
		await expect.element(page.getByRole('dialog', { name: 'Assistant' })).toBeVisible();
		expect(api.getSession).toHaveBeenCalledOnce();

		await page.getByRole('button', { name: 'Close assistant' }).click();
		await expect.element(page.getByRole('button', { name: 'Open assistant' })).toBeVisible();
		await page.getByRole('button', { name: 'Open assistant' }).click();
		expect(api.getSession).toHaveBeenCalledOnce();
	});
});

function controllableStream() {
	let handler: ((event: AgentEvent) => void | Promise<void>) | undefined;
	return {
		streamEvents: vi.fn(
			async (
				_sessionId: string,
				_after: number,
				onEvent: (event: AgentEvent) => void | Promise<void>,
				signal: AbortSignal
			) => {
				handler = onEvent;
				return new Promise<number>((resolve) =>
					signal.addEventListener('abort', () => resolve(0), { once: true })
				);
			}
		),
		emit: async (event: AgentEvent) => {
			if (!handler) throw new Error('stream is not connected');
			await handler(event);
		}
	};
}

function testAPI(overrides: Partial<AgentAPI> = {}): AgentAPI {
	return {
		createSession: vi.fn().mockResolvedValue(testSession()),
		getSession: vi.fn().mockResolvedValue(testConversation()),
		startContextRun: vi.fn().mockResolvedValue(testRun()),
		postMessage: vi.fn().mockResolvedValue({ message: userMessage(), run: testRun() }),
		abortRun: vi.fn().mockResolvedValue(testRun({ status: 'aborted' })),
		streamEvents: vi.fn(
			async (_sessionId, _after, _onEvent, signal) =>
				new Promise<number>((resolve) =>
					signal.addEventListener('abort', () => resolve(0), { once: true })
				)
		),
		streamContextRunEvents: vi.fn(
			async (_runId, _after, _onEvent, signal) =>
				new Promise<number>((resolve) =>
					signal.addEventListener('abort', () => resolve(0), { once: true })
				)
		),
		...overrides
	};
}

function testStorage(sessionId: string | null) {
	return {
		getItem: vi.fn(() => sessionId),
		setItem: vi.fn(),
		removeItem: vi.fn()
	};
}

function testConversation(overrides: Partial<AgentConversation> = {}): AgentConversation {
	return {
		session: testSession(),
		messages: [],
		runs: [],
		lastStreamOffset: 0,
		...overrides
	};
}

function testSession(overrides = {}) {
	return {
		id: 'session-id',
		createdAt: '2026-07-18T10:00:00Z',
		updatedAt: '2026-07-18T10:00:00Z',
		...overrides
	};
}

function testRun(overrides = {}) {
	return {
		id: 'run-id',
		sessionId: 'session-id',
		status: 'running' as const,
		correlationId: 'correlation-id',
		startedAt: '2026-07-18T10:00:00Z',
		completedAt: null,
		error: null,
		createdAt: '2026-07-18T10:00:00Z',
		updatedAt: '2026-07-18T10:00:00Z',
		...overrides
	};
}

function userMessage() {
	return {
		id: 'user-message',
		sessionId: 'session-id',
		runId: 'run-id',
		role: 'user' as const,
		content: 'Is my inbox clear?',
		createdAt: '2026-07-18T10:00:00Z',
		updatedAt: '2026-07-18T10:00:00Z'
	};
}

function assistantMessage(content: string) {
	return {
		id: 'assistant-message',
		sessionId: 'session-id',
		runId: 'run-id',
		role: 'assistant' as const,
		content,
		createdAt: '2026-07-18T10:00:01Z',
		updatedAt: '2026-07-18T10:00:01Z'
	};
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
