export type AgentRole = 'user' | 'assistant';
export type AgentRunStatus = 'queued' | 'running' | 'completed' | 'failed' | 'aborted';

export const agentSessionStorageKey = 'todai.agent.session-id';

export interface AgentSession {
	id: string;
	createdAt: string;
	updatedAt: string;
}

export interface AgentMessage {
	id: string;
	sessionId: string;
	runId: string | null;
	role: AgentRole;
	content: string;
	context?: AgentRunContext;
	createdAt: string;
	updatedAt: string;
}

export interface AgentRunContext {
	type: 'task';
	taskId: string;
	action: 'decompose';
}

export interface AgentMessageRequest {
	message: string;
}

export interface AgentRun {
	id: string;
	sessionId: string;
	status: AgentRunStatus;
	correlationId: string;
	startedAt: string | null;
	completedAt: string | null;
	error: string | null;
	createdAt: string;
	updatedAt: string;
}

export interface AgentConversation {
	session: AgentSession;
	messages: AgentMessage[];
	runs: AgentRun[];
	lastStreamOffset: number;
}

export interface PostedAgentMessage {
	message: AgentMessage;
	run: AgentRun;
}

export interface AgentEvent {
	streamOffset: number;
	runId: string;
	sessionId: string;
	sequence: number;
	type: string;
	occurredAt: string;
	payload: Record<string, unknown>;
}

export interface AgentAPI {
	createSession(): Promise<AgentSession>;
	getSession(sessionId: string): Promise<AgentConversation>;
	startContextRun(context: AgentRunContext): Promise<AgentRun>;
	postMessage(sessionId: string, request: AgentMessageRequest): Promise<PostedAgentMessage>;
	abortRun(runId: string): Promise<AgentRun>;
	streamEvents(
		sessionId: string,
		after: number,
		onEvent: (event: AgentEvent) => void | Promise<void>,
		signal: AbortSignal
	): Promise<number>;
	streamContextRunEvents(
		runId: string,
		after: number,
		onEvent: (event: AgentEvent) => void | Promise<void>,
		signal: AbortSignal
	): Promise<number>;
}

export class AgentRequestError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
		this.name = 'AgentRequestError';
	}
}

export function createAgentAPI(fetcher: typeof fetch = fetch): AgentAPI {
	return {
		createSession: () => createAgentSession(fetcher),
		getSession: (sessionId) => getAgentSession(fetcher, sessionId),
		startContextRun: (context) => startAgentContextRun(fetcher, context),
		postMessage: (sessionId, request) => postAgentMessage(fetcher, sessionId, request),
		abortRun: (runId) => abortAgentRun(fetcher, runId),
		streamEvents: (sessionId, after, onEvent, signal) =>
			streamAgentEvents(fetcher, sessionId, after, onEvent, signal),
		streamContextRunEvents: (runId, after, onEvent, signal) =>
			streamAgentContextRunEvents(fetcher, runId, after, onEvent, signal)
	};
}

export async function startAgentContextRun(
	fetcher: typeof fetch,
	context: AgentRunContext
): Promise<AgentRun> {
	return requestJSON(
		fetcher,
		'/api/agent/runs',
		{ method: 'POST', body: JSON.stringify({ context }) },
		'Could not start the agent action.'
	);
}

export async function createAgentSession(fetcher: typeof fetch): Promise<AgentSession> {
	return requestJSON(fetcher, '/api/agent/sessions', { method: 'POST' }, 'Could not start a chat.');
}

export async function getAgentSession(
	fetcher: typeof fetch,
	sessionId: string
): Promise<AgentConversation> {
	return requestJSON(
		fetcher,
		`/api/agent/sessions/${encodeURIComponent(sessionId)}`,
		{},
		'Could not load the chat.'
	);
}

export async function postAgentMessage(
	fetcher: typeof fetch,
	sessionId: string,
	request: AgentMessageRequest
): Promise<PostedAgentMessage> {
	return requestJSON(
		fetcher,
		`/api/agent/sessions/${encodeURIComponent(sessionId)}/messages`,
		{ method: 'POST', body: JSON.stringify(request) },
		'Could not send the message.'
	);
}

export async function abortAgentRun(fetcher: typeof fetch, runId: string): Promise<AgentRun> {
	return requestJSON(
		fetcher,
		`/api/agent/runs/${encodeURIComponent(runId)}/abort`,
		{ method: 'POST' },
		'Could not stop the assistant.'
	);
}

export async function streamAgentEvents(
	fetcher: typeof fetch,
	sessionId: string,
	after: number,
	onEvent: (event: AgentEvent) => void | Promise<void>,
	signal: AbortSignal
): Promise<number> {
	return streamAgentEventSource(
		fetcher,
		`/api/agent/sessions/${encodeURIComponent(sessionId)}/events`,
		after,
		onEvent,
		signal
	);
}

export async function streamAgentContextRunEvents(
	fetcher: typeof fetch,
	runId: string,
	after: number,
	onEvent: (event: AgentEvent) => void | Promise<void>,
	signal: AbortSignal
): Promise<number> {
	return streamAgentEventSource(
		fetcher,
		`/api/agent/runs/${encodeURIComponent(runId)}/events`,
		after,
		onEvent,
		signal
	);
}

async function streamAgentEventSource(
	fetcher: typeof fetch,
	path: string,
	after: number,
	onEvent: (event: AgentEvent) => void | Promise<void>,
	signal: AbortSignal
): Promise<number> {
	const headers: Record<string, string> = { Accept: 'text/event-stream' };
	if (after > 0) headers['Last-Event-ID'] = String(after);
	const response = await fetcher(path, {
		credentials: 'same-origin',
		headers,
		signal
	});
	if (!response.ok || response.body === null) {
		throw new AgentRequestError('Could not connect to the assistant.', response.status);
	}

	const reader = response.body.getReader();
	const decoder = new TextDecoder();
	let buffer = '';
	let cursor = after;
	while (!signal.aborted) {
		const { done, value } = await reader.read();
		buffer += decoder.decode(value, { stream: !done });
		const parsed = extractSSERecords(buffer);
		buffer = parsed.remainder;
		for (const record of parsed.records) {
			if (record.id <= cursor) continue;
			cursor = record.id;
			await onEvent(record.event);
		}
		if (done) return cursor;
	}
	return cursor;
}

async function requestJSON<T>(
	fetcher: typeof fetch,
	path: string,
	init: RequestInit,
	message: string
): Promise<T> {
	const response = await fetcher(path, {
		...init,
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json', ...init.headers }
	});
	if (!response.ok) throw new AgentRequestError(message, response.status);
	return (await response.json()) as T;
}

interface ParsedSSERecord {
	id: number;
	event: AgentEvent;
}

function extractSSERecords(input: string): { records: ParsedSSERecord[]; remainder: string } {
	const normalized = input.replaceAll('\r\n', '\n');
	const parts = normalized.split('\n\n');
	const remainder = parts.pop() ?? '';
	const records: ParsedSSERecord[] = [];
	for (const part of parts) {
		const parsed = parseSSERecord(part);
		if (parsed) records.push(parsed);
	}
	return { records, remainder };
}

function parseSSERecord(record: string): ParsedSSERecord | null {
	let rawID = '';
	const data: string[] = [];
	for (const line of record.split('\n')) {
		if (line.startsWith(':')) continue;
		const separator = line.indexOf(':');
		const field = separator === -1 ? line : line.slice(0, separator);
		const value = separator === -1 ? '' : line.slice(separator + 1).replace(/^ /, '');
		if (field === 'id') rawID = value;
		if (field === 'data') data.push(value);
	}
	const id = Number(rawID);
	if (!Number.isSafeInteger(id) || id < 1 || data.length === 0) return null;
	const event = JSON.parse(data.join('\n')) as AgentEvent;
	return { id, event };
}
