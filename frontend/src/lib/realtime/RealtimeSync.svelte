<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { onMount } from 'svelte';
	import { pollActivityChanges, RealtimeRequestError } from './client';
	import { publishActivityEvent } from './events';

	type Poll = typeof pollActivityChanges;

	interface Props {
		poll?: Poll;
		refresh?: () => Promise<void>;
		currentPath?: () => string;
	}

	let {
		poll = pollActivityChanges,
		refresh = invalidateAll,
		currentPath = () => window.location.pathname
	}: Props = $props();
	let controller: AbortController | null = null;

	onMount(() => {
		controller = new AbortController();
		void consume(controller.signal);
		return () => {
			controller?.abort();
		};
	});

	async function consume(signal: AbortSignal) {
		let cursor: number | null = null;
		while (!signal.aborted) {
			try {
				const changes = await poll(fetch, cursor, signal);
				for (const event of changes.events) publishActivityEvent(event);
				const shouldRefresh =
					cursor === null ||
					changes.events.some(
						(event) => affectsApplicationData(event.type) || currentPath() === '/activity'
					);
				if (shouldRefresh) await refresh();
				cursor = changes.cursor;
				await delay(100, signal);
			} catch (error) {
				if (signal.aborted) return;
				if (error instanceof RealtimeRequestError && error.status === 401) return;
				await delay(1000, signal);
			}
		}
	}

	function affectsApplicationData(type: string): boolean {
		return /^(task|project|section|user_settings)\./.test(type);
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
