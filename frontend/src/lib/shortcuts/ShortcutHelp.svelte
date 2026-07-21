<script lang="ts">
	import { onMount } from 'svelte';
	import { formatShortcuts, shortcutCommands } from './registry';

	interface Props {
		applePlatform: boolean;
		close: () => void;
	}

	let { applePlatform, close }: Props = $props();
	let dialog: HTMLDivElement;
	let closeButton: HTMLButtonElement;

	onMount(() => closeButton.focus());

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Tab') return;
		const focusable = Array.from(dialog.querySelectorAll<HTMLElement>('button:not([disabled])'));
		if (focusable.length === 0) return;
		const first = focusable[0];
		const last = focusable.at(-1)!;
		if (event.shiftKey && document.activeElement === first) {
			event.preventDefault();
			last.focus();
		} else if (!event.shiftKey && document.activeElement === last) {
			event.preventDefault();
			first.focus();
		}
	}
</script>

<div
	class="backdrop"
	role="presentation"
	onclick={(event) => {
		if (event.target === event.currentTarget) close();
	}}
>
	<div
		bind:this={dialog}
		class="dialog"
		role="dialog"
		tabindex="-1"
		aria-modal="true"
		aria-labelledby="shortcut-help-title"
		onkeydown={handleKeydown}
	>
		<header>
			<div>
				<p>REFERENCE</p>
				<h2 id="shortcut-help-title">Keyboard shortcuts</h2>
			</div>
			<button
				bind:this={closeButton}
				type="button"
				aria-label="Close keyboard shortcuts"
				onclick={close}>×</button
			>
		</header>
		<ul>
			{#each shortcutCommands as command (command.id)}
				<li>
					<span><strong>{command.label}</strong><small>{command.description}</small></span>
					<span class="shortcut-keys">
						{#each formatShortcuts(command, applePlatform) as shortcut (shortcut)}
							<kbd>{shortcut}</kbd>
						{/each}
					</span>
				</li>
			{/each}
		</ul>
		<footer>Shortcuts require one simultaneous keypress. Escape closes the top surface.</footer>
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 90;
		display: grid;
		place-items: center;
		padding: 1rem;
		background: rgb(24 25 23 / 45%);
		backdrop-filter: blur(2px);
	}
	.dialog {
		width: min(38rem, 100%);
		max-height: min(46rem, calc(100vh - 2rem));
		border: 1px solid var(--theme-border, #dfe5dc);
		border-radius: 1rem;
		background: #fff;
		box-shadow: 0 1.5rem 5rem rgb(20 28 21 / 24%);
		overflow: auto;
	}
	header {
		position: sticky;
		top: 0;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.15rem 1.25rem;
		border-bottom: 1px solid var(--theme-border, #dfe5dc);
		background: var(--theme-canvas, #fbfcfa);
	}
	header p,
	h2 {
		margin: 0;
	}
	header p {
		margin-bottom: 0.3rem;
		color: var(--theme-accent, #2d6540);
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.1em;
	}
	h2 {
		font-size: 1.35rem;
	}
	header button {
		width: 2.25rem;
		height: 2.25rem;
		border: 0;
		border-radius: 50%;
		background: transparent;
		font-size: 1.5rem;
		cursor: pointer;
	}
	header button:hover {
		background: var(--theme-hover, #e6ece4);
	}
	ul {
		margin: 0;
		padding: 0.45rem 1.25rem;
		list-style: none;
	}
	li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.75rem 0;
		border-bottom: 1px solid var(--theme-border, #dfe5dc);
	}
	li:last-child {
		border-bottom: 0;
	}
	li > span:first-child {
		display: grid;
		gap: 0.2rem;
	}
	.shortcut-keys {
		display: grid;
		flex: none;
		gap: 0.35rem;
	}
	strong {
		font-size: 0.85rem;
	}
	small,
	footer {
		color: #777772;
		font-size: 0.72rem;
	}
	kbd {
		flex: none;
		min-width: 5rem;
		padding: 0.38rem 0.55rem;
		border: 1px solid #d7d9d5;
		border-bottom-width: 2px;
		border-radius: 0.4rem;
		background: #f4f5f3;
		font-family: inherit;
		font-size: 0.72rem;
		font-weight: 750;
		text-align: center;
	}
	footer {
		padding: 0.9rem 1.25rem;
		border-top: 1px solid var(--theme-border, #dfe5dc);
		background: var(--theme-canvas, #fbfcfa);
	}
</style>
