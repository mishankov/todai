<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { AgentRequestError, createAgentAPI, type AgentAPI, type AgentEvent } from './client';
	import {
		activeAgentRun,
		applyAgentEvent,
		stateFromConversation,
		visibleAgentMessages,
		type AgentChatState,
		type AgentToolActivity
	} from './state';

	interface Props {
		api?: AgentAPI;
		storage?: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;
	}

	const storageKey = 'todai.agent.session-id';
	let { api = createAgentAPI(), storage }: Props = $props();
	let chatState = $state<AgentChatState | null>(null);
	let open = $state(false);
	let initialized = $state(false);
	let loading = $state(false);
	let posting = $state(false);
	let stopping = $state(false);
	let creating = $state(false);
	let reconnecting = $state(false);
	let draft = $state('');
	let errorMessage = $state('');
	let streamController: AbortController | null = null;
	let composer: HTMLTextAreaElement;
	let conversationLog: HTMLDivElement;

	let messages = $derived(chatState ? visibleAgentMessages(chatState) : []);
	let activeRun = $derived(chatState ? activeAgentRun(chatState) : null);
	let visibleTools = $derived(chatState ? recentTools(chatState.tools, activeRun?.id ?? null) : []);
	let canSend = $derived(
		chatState !== null && draft.trim() !== '' && !posting && !stopping && activeRun === null
	);

	onMount(() => {
		storage ??= window.localStorage;
		return () => streamController?.abort();
	});

	async function openChat() {
		open = true;
		if (!initialized) {
			initialized = true;
			await restoreOrCreateSession();
		}
		await tick();
		await scrollConversation();
		if (open) composer?.focus();
	}

	function closeChat() {
		open = false;
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) closeChat();
	}

	async function restoreOrCreateSession() {
		loading = true;
		errorMessage = '';
		const storedSessionID = storage?.getItem(storageKey);
		if (storedSessionID) {
			try {
				const conversation = await api.getSession(storedSessionID);
				openConversation(stateFromConversation(conversation));
				loading = false;
				return;
			} catch (error) {
				if (!(error instanceof AgentRequestError) || error.status !== 404) {
					loading = false;
					errorMessage = errorText(error, 'Could not load the chat.');
					return;
				}
				storage?.removeItem(storageKey);
			}
		}
		await createConversation();
		loading = false;
	}

	async function createConversation() {
		creating = true;
		errorMessage = '';
		try {
			const session = await api.createSession();
			storage?.setItem(storageKey, session.id);
			openConversation({
				sessionId: session.id,
				messages: [],
				runs: [],
				cursor: 0,
				deltas: {},
				tools: []
			});
			await tick();
			if (open) composer?.focus();
		} catch (error) {
			errorMessage = errorText(error, 'Could not start a chat.');
		} finally {
			creating = false;
		}
	}

	function openConversation(next: AgentChatState) {
		chatState = next;
		streamController?.abort();
		streamController = new AbortController();
		void consumeEvents(next.sessionId, streamController.signal);
	}

	async function consumeEvents(sessionId: string, signal: AbortSignal) {
		while (!signal.aborted) {
			try {
				const cursor = await api.streamEvents(
					sessionId,
					chatState?.sessionId === sessionId ? chatState.cursor : 0,
					handleEvent,
					signal
				);
				if (chatState?.sessionId === sessionId && cursor > chatState.cursor)
					chatState = { ...chatState, cursor };
				reconnecting = true;
			} catch (error) {
				if (signal.aborted) return;
				reconnecting = true;
				if (error instanceof AgentRequestError && error.status === 401) {
					errorMessage = 'Your session expired. Sign in again to continue.';
					return;
				}
			}
			await delay(1000, signal);
		}
	}

	async function handleEvent(event: AgentEvent) {
		if (!chatState || event.sessionId !== chatState.sessionId) return;
		chatState = applyAgentEvent(chatState, event);
		reconnecting = false;
		await scrollConversation();
		if (isTerminalEvent(event.type)) {
			stopping = false;
			await reconcileConversation();
			await tick();
			if (open) composer?.focus();
		}
	}

	async function sendMessage() {
		if (!canSend || !chatState) return;
		const sessionId = chatState.sessionId;
		const message = draft.trim();
		posting = true;
		errorMessage = '';
		try {
			const posted = await api.postMessage(sessionId, message);
			if (chatState?.sessionId !== sessionId) return;
			const streamedRun = chatState.runs.find((run) => run.id === posted.run.id);
			const mergedRun = streamedRun
				? { ...posted.run, status: streamedRun.status, error: streamedRun.error }
				: posted.run;
			chatState = {
				...chatState,
				messages: chatState.messages.some((item) => item.id === posted.message.id)
					? chatState.messages
					: [...chatState.messages, posted.message],
				runs: [...chatState.runs.filter((run) => run.id !== posted.run.id), mergedRun]
			};
			draft = '';
			await scrollConversation();
		} catch (error) {
			errorMessage = errorText(error, 'Could not send the message.');
			await reconcileConversation();
		} finally {
			posting = false;
		}
	}

	async function stopRun() {
		if (!activeRun || stopping) return;
		stopping = true;
		errorMessage = '';
		try {
			await api.abortRun(activeRun.id);
		} catch (error) {
			errorMessage = errorText(error, 'Could not stop the assistant.');
			stopping = false;
		}
	}

	async function newConversation() {
		if (activeRun || creating) return;
		storage?.removeItem(storageKey);
		chatState = null;
		streamController?.abort();
		await createConversation();
	}

	async function reconcileConversation() {
		if (!chatState) return;
		const sessionId = chatState.sessionId;
		try {
			const conversation = await api.getSession(sessionId);
			if (chatState?.sessionId === sessionId) chatState = stateFromConversation(conversation);
		} catch (error) {
			if (!errorMessage) errorMessage = errorText(error, 'Could not refresh the chat.');
		}
	}

	function handleComposerKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
		event.preventDefault();
		void sendMessage();
	}

	async function scrollConversation() {
		await tick();
		if (conversationLog) conversationLog.scrollTop = conversationLog.scrollHeight;
	}

	function errorText(error: unknown, fallback: string): string {
		return error instanceof Error && error.message ? error.message : fallback;
	}

	function isTerminalEvent(type: string): boolean {
		return (
			type === 'agent.run.completed' || type === 'agent.run.failed' || type === 'agent.run.aborted'
		);
	}

	function recentTools(tools: AgentToolActivity[], runId: string | null): AgentToolActivity[] {
		const relevant = runId ? tools.filter((tool) => tool.runId === runId) : tools;
		return relevant.slice(-3);
	}

	function toolLabel(tool: AgentToolActivity): string {
		const labels: Record<string, string> = {
			task_get: 'Reading a task',
			task_search: 'Searching tasks',
			project_list: 'Reading projects',
			view_query: 'Reading your task list',
			task_create: 'Creating a task',
			task_update: 'Updating a task',
			task_complete: 'Completing a task',
			task_reopen: 'Reopening a task',
			task_move: 'Moving a task',
			task_reorder: 'Reordering a task'
		};
		return labels[tool.name] ?? tool.name.replaceAll('_', ' ');
	}

	function delay(milliseconds: number, signal: AbortSignal): Promise<void> {
		return new Promise((resolve) => {
			if (signal.aborted) return resolve();
			const timeout = window.setTimeout(resolve, milliseconds);
			signal.addEventListener(
				'abort',
				() => {
					window.clearTimeout(timeout);
					resolve();
				},
				{ once: true }
			);
		});
	}
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<button
	class="chat-launcher"
	class:hidden={open}
	type="button"
	aria-label="Open assistant"
	aria-haspopup="dialog"
	aria-expanded={open}
	aria-controls="assistant-popup"
	onclick={() => void openChat()}
>
	<svg viewBox="0 0 24 24" aria-hidden="true">
		<path d="M5 5h14v11H9l-4 3z" />
		<path d="M9 9h6M9 12h4" />
	</svg>
	{#if activeRun}<span class="run-indicator" aria-label="Assistant is working"></span>{/if}
</button>

<div
	id="assistant-popup"
	class="chat-popup"
	class:open
	role="dialog"
	aria-modal="false"
	aria-labelledby="assistant-heading"
	aria-hidden={!open}
>
	<header class="popup-header">
		<div class="popup-identity">
			<span class="popup-mark" aria-hidden="true">T</span>
			<div>
				<h2 id="assistant-heading">Assistant</h2>
				<p>{activeRun ? 'Working on your request…' : 'Your task companion'}</p>
			</div>
		</div>
		<div class="popup-actions">
			<button
				class="new-chat"
				type="button"
				disabled={!initialized || creating || activeRun !== null}
				onclick={() => void newConversation()}
			>
				{creating ? 'Starting…' : 'New chat'}
			</button>
			<button class="close-chat" type="button" aria-label="Close assistant" onclick={closeChat}>
				<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m7 7 10 10M17 7 7 17" /></svg>
			</button>
		</div>
	</header>

	<div class="chat-panel">
		<div
			class="conversation"
			bind:this={conversationLog}
			role="log"
			aria-live="polite"
			aria-label="Assistant conversation"
		>
			{#if loading}
				<div class="loading-state" role="status">Loading conversation…</div>
			{:else if !chatState}
				<div class="empty-state">
					<h2>Assistant unavailable</h2>
					<p>Try starting a new conversation.</p>
				</div>
			{:else if messages.length === 0}
				<div class="empty-state">
					<span class="assistant-mark" aria-hidden="true">T</span>
					<h2>What would you like to get done?</h2>
					<p>I can find, create, update, move, and complete tasks.</p>
					<div class="suggestions" aria-label="Example prompts">
						<button type="button" onclick={() => (draft = 'What should I focus on today?')}
							>What should I focus on today?</button
						>
						<button type="button" onclick={() => (draft = 'Create a task to plan tomorrow')}
							>Create a task for tomorrow</button
						>
					</div>
				</div>
			{:else}
				<div class="message-list">
					{#each messages as message (message.id)}
						<article class:from-user={message.role === 'user'} class="message">
							<p class="message-role">{message.role === 'user' ? 'You' : 'Assistant'}</p>
							<p class="message-content">{message.content}</p>
						</article>
					{/each}
				</div>
			{/if}

			{#if visibleTools.length > 0}
				<div class="tool-activity" aria-label="Assistant activity">
					{#each visibleTools as tool (tool.id)}
						<div class:failed={tool.status === 'failed'} role="status">
							<span class:spinning={tool.status === 'running'} aria-hidden="true"></span>
							<span
								>{toolLabel(tool)}{tool.status === 'completed'
									? ' — done'
									: tool.status === 'failed'
										? ' — failed'
										: '…'}</span
							>
						</div>
					{/each}
				</div>
			{/if}

			{#if activeRun && visibleTools.length === 0}
				<p class="run-status" role="status">
					{activeRun.status === 'queued' ? 'Getting ready…' : 'Thinking…'}
				</p>
			{/if}
		</div>

		<div class="composer-area">
			{#if reconnecting}
				<p class="connection-status" role="status">Reconnecting…</p>
			{/if}
			{#if errorMessage}
				<p class="error" role="alert">{errorMessage}</p>
			{/if}
			<form
				onsubmit={(event) => {
					event.preventDefault();
					void sendMessage();
				}}
			>
				<label for="agent-message">Message the assistant</label>
				<textarea
					bind:this={composer}
					bind:value={draft}
					id="agent-message"
					rows="2"
					placeholder="Ask about your tasks…"
					disabled={loading || chatState === null || activeRun !== null}
					onkeydown={handleComposerKeydown}></textarea>
				<div class="composer-actions">
					<span>Enter to send · Shift+Enter for a new line</span>
					{#if activeRun}
						<button class="stop" type="button" disabled={stopping} onclick={() => void stopRun()}
							>{stopping ? 'Stopping…' : 'Stop'}</button
						>
					{:else}
						<button class="send" type="submit" disabled={!canSend}
							>{posting ? 'Sending…' : 'Send'}</button
						>
					{/if}
				</div>
			</form>
		</div>
	</div>
</div>

<style>
	.chat-launcher {
		position: fixed;
		z-index: 41;
		right: 1.5rem;
		bottom: 1.5rem;
		display: grid;
		width: 3.7rem;
		height: 3.7rem;
		place-items: center;
		padding: 0;
		border: 0;
		border-radius: 50%;
		color: #fff;
		background: #2d6540;
		box-shadow: 0 0.8rem 2.2rem rgb(35 77 49 / 28%);
		cursor: pointer;
		transition:
			transform 160ms ease,
			opacity 160ms ease,
			box-shadow 160ms ease;
	}
	.chat-launcher:hover {
		transform: translateY(-2px);
		box-shadow: 0 1rem 2.5rem rgb(35 77 49 / 34%);
	}
	.chat-launcher:focus-visible {
		outline: 3px solid rgb(45 101 64 / 24%);
		outline-offset: 3px;
	}
	.chat-launcher.hidden {
		transform: scale(0.75);
		opacity: 0;
		pointer-events: none;
	}
	.chat-launcher svg,
	.close-chat svg {
		width: 1.45rem;
		height: 1.45rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.7;
	}
	.run-indicator {
		position: absolute;
		top: 0.2rem;
		right: 0.15rem;
		width: 0.75rem;
		height: 0.75rem;
		border: 2px solid #fff;
		border-radius: 50%;
		background: #e7b44b;
	}
	.chat-popup {
		position: fixed;
		z-index: 40;
		right: 1.5rem;
		bottom: 1.5rem;
		width: min(26rem, calc(100vw - 2rem));
		height: min(42rem, calc(100dvh - 3rem));
		border: 1px solid #d8e1d5;
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 1.5rem 4rem rgb(31 51 36 / 22%);
		overflow: hidden;
		opacity: 0;
		visibility: hidden;
		transform: translateY(0.8rem) scale(0.98);
		transform-origin: right bottom;
		pointer-events: none;
		transition:
			opacity 160ms ease,
			transform 160ms ease,
			visibility 160ms ease;
	}
	.chat-popup.open {
		opacity: 1;
		visibility: visible;
		transform: none;
		pointer-events: auto;
	}
	.popup-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		height: 4.35rem;
		padding: 0 0.85rem 0 1rem;
		border-bottom: 1px solid #e1e7df;
		background: #f7faf6;
	}
	.popup-identity,
	.popup-actions {
		display: flex;
		align-items: center;
	}
	.popup-identity {
		gap: 0.7rem;
		min-width: 0;
	}
	.popup-actions {
		gap: 0.35rem;
	}
	.popup-mark {
		display: grid;
		width: 2rem;
		height: 2rem;
		flex: none;
		place-items: center;
		border-radius: 0.6rem;
		color: #fff;
		background: #2d6540;
		font-weight: 800;
	}
	.popup-header h2 {
		margin: 0;
		color: #2d2d2a;
		font-size: 0.95rem;
		letter-spacing: -0.01em;
	}
	.popup-header p {
		margin: 0.15rem 0 0;
		color: #7c827b;
		font-size: 0.7rem;
	}
	.new-chat {
		padding: 0.45rem 0.55rem;
		border: 0;
		border-radius: 0.45rem;
		color: #315e40;
		background: transparent;
		font-size: 0.72rem;
		font-weight: 720;
		cursor: pointer;
	}
	.new-chat:hover:not(:disabled) {
		background: #edf3ec;
	}
	.new-chat:disabled {
		cursor: not-allowed;
		opacity: 0.55;
	}
	.close-chat {
		display: grid;
		width: 2rem;
		height: 2rem;
		place-items: center;
		padding: 0;
		border: 0;
		border-radius: 0.45rem;
		color: #6c716b;
		background: transparent;
		cursor: pointer;
	}
	.close-chat:hover {
		color: #2d2d2a;
		background: #e9efe7;
	}
	.chat-panel {
		display: grid;
		grid-template-rows: minmax(0, 1fr) auto;
		height: calc(100% - 4.35rem);
		background: #fff;
	}
	.conversation {
		min-height: 0;
		padding: 1.15rem;
		overflow-y: auto;
	}
	.message-list {
		display: grid;
		gap: 1.1rem;
	}
	.message {
		width: min(90%, 22rem);
	}
	.message.from-user {
		justify-self: end;
		padding: 0.9rem 1rem;
		border-radius: 0.85rem 0.85rem 0.2rem 0.85rem;
		background: #e8f0e6;
	}
	.message-role {
		margin: 0 0 0.35rem;
		color: #50715a;
		font-size: 0.7rem;
		font-weight: 760;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}
	.message-content {
		margin: 0;
		color: #30302d;
		line-height: 1.62;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.empty-state,
	.loading-state {
		display: grid;
		min-height: 20rem;
		place-items: center;
		align-content: center;
		text-align: center;
	}
	.empty-state h2 {
		margin: 1rem 0 0.45rem;
		color: #31312e;
		font-size: 1.35rem;
	}
	.empty-state p,
	.loading-state {
		margin: 0;
		color: #7b7b75;
	}
	.assistant-mark {
		display: grid;
		width: 2.6rem;
		height: 2.6rem;
		place-items: center;
		border-radius: 0.8rem;
		color: #fff;
		background: #2d6540;
		font-weight: 800;
	}
	.suggestions {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 0.55rem;
		margin-top: 1.4rem;
	}
	.suggestions button {
		padding: 0.55rem 0.75rem;
		border: 1px solid #d6dfd3;
		border-radius: 999px;
		color: #4f6253;
		background: #fbfcfa;
		cursor: pointer;
	}
	.suggestions button:hover {
		border-color: #9eb5a2;
		background: #f0f5ef;
	}
	.tool-activity {
		display: grid;
		gap: 0.4rem;
		margin-top: 1.3rem;
		padding-top: 1rem;
		border-top: 1px solid #edf0eb;
	}
	.tool-activity div,
	.run-status {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		margin: 0;
		color: #68736a;
		font-size: 0.8rem;
	}
	.tool-activity div.failed {
		color: #ad493f;
	}
	.tool-activity div > span:first-child {
		width: 0.55rem;
		height: 0.55rem;
		border: 1.5px solid #8da594;
		border-radius: 50%;
	}
	.tool-activity div > span:first-child.spinning {
		border-right-color: transparent;
		animation: spin 700ms linear infinite;
	}
	.run-status {
		margin-top: 1.25rem;
	}
	.composer-area {
		padding: 1rem;
		border-top: 1px solid #dfe5dc;
		background: #f8faf7;
	}
	form {
		padding: 0.7rem 0.8rem 0.6rem;
		border: 1px solid #ccd8c9;
		border-radius: 0.75rem;
		background: #fff;
		box-shadow: 0 0.4rem 1.2rem rgb(45 65 49 / 5%);
	}
	form:focus-within {
		border-color: #78a184;
		box-shadow: 0 0 0 3px rgb(76 127 91 / 11%);
	}
	label {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
	textarea {
		width: 100%;
		min-height: 3.2rem;
		resize: none;
		padding: 0;
		border: 0;
		outline: 0;
		color: #292927;
		background: transparent;
		font: inherit;
		line-height: 1.5;
	}
	textarea::placeholder {
		color: #999d96;
	}
	textarea:disabled {
		cursor: not-allowed;
	}
	.composer-actions {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-top: 0.45rem;
	}
	.composer-actions > span {
		color: #93958f;
		font-size: 0.7rem;
	}
	.send,
	.stop {
		min-width: 4.2rem;
		padding: 0.48rem 0.72rem;
		border: 0;
		border-radius: 0.5rem;
		font-weight: 750;
		cursor: pointer;
	}
	.send {
		color: #fff;
		background: #2d6540;
	}
	.stop {
		color: #9f3d35;
		background: #f7eae8;
	}
	.send:disabled,
	.stop:disabled {
		cursor: not-allowed;
		opacity: 0.5;
	}
	.error,
	.connection-status {
		margin: 0 0 0.65rem;
		font-size: 0.78rem;
	}
	.error {
		color: #ad493f;
	}
	.connection-status {
		color: #777b75;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	@media (max-width: 48rem) {
		.chat-launcher {
			right: 1rem;
			bottom: 1rem;
		}
		.chat-popup {
			inset: 0;
			width: 100%;
			height: 100dvh;
			border: 0;
			border-radius: 0;
			transform: translateY(1rem);
		}
		.chat-popup.open {
			transform: none;
		}
		.popup-header {
			height: 4rem;
			padding-inline: 0.8rem;
		}
		.chat-panel {
			height: calc(100% - 4rem);
		}
		.conversation {
			padding: 1rem;
		}
		.message {
			width: 94%;
		}
		.composer-area {
			padding: 0.8rem;
		}
		.composer-actions > span {
			display: none;
		}
	}
</style>
