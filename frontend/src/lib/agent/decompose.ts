import type { Task } from '$lib/tasks/client';
import { createAgentAPI, type AgentAPI, type AgentEvent } from './client';

export async function decomposeTaskWithAgent(
	task: Pick<Task, 'id' | 'projectId'>,
	api: AgentAPI = createAgentAPI(fetch, task.projectId)
): Promise<void> {
	const run = await api.startContextRun({ type: 'task', taskId: task.id, action: 'decompose' });
	await waitForRun(api, run.id, 0);
}

async function waitForRun(api: AgentAPI, runId: string, after: number): Promise<void> {
	const controller = new AbortController();
	let failure = '';
	let terminal = false;
	const handleEvent = (event: AgentEvent) => {
		if (event.runId !== runId) return;
		if (event.type === 'agent.run.failed') failure = eventError(event) || 'Decomposition failed.';
		if (event.type === 'agent.run.aborted') failure = 'Decomposition was stopped.';
		if (
			event.type === 'agent.run.completed' ||
			event.type === 'agent.run.failed' ||
			event.type === 'agent.run.aborted'
		) {
			terminal = true;
			controller.abort();
		}
	};

	try {
		await api.streamContextRunEvents(runId, after, handleEvent, controller.signal);
	} catch (error) {
		if (!controller.signal.aborted) throw error;
	}
	if (!terminal) throw new Error('The assistant connection ended before decomposition finished.');
	if (failure) throw new Error(failure);
}

function eventError(event: AgentEvent): string {
	const error = event.payload.error;
	if (typeof error === 'string') return error;
	if (typeof error === 'object' && error !== null && 'message' in error) {
		const message = (error as { message?: unknown }).message;
		if (typeof message === 'string') return message;
	}
	return '';
}
