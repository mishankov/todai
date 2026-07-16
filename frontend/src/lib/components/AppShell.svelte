<script lang="ts">
	import { resolve } from '$app/paths';
	import type { Snippet } from 'svelte';

	interface Props {
		username: string;
		onLogout: () => Promise<void>;
		currentPath?: string;
		children?: Snippet;
	}

	let { username, onLogout, currentPath = '/', children }: Props = $props();
	let signingOut = $state(false);
	let errorMessage = $state('');

	async function signOut() {
		signingOut = true;
		errorMessage = '';

		try {
			await onLogout();
		} catch {
			errorMessage = 'Sign out failed. Please try again.';
			signingOut = false;
		}
	}
</script>

<main class="shell">
	<header>
		<div class="brand">
			<span class="mark" aria-hidden="true">T</span>
			<span>Todai</span>
		</div>

		<nav aria-label="Primary navigation">
			<a
				href={resolve('/')}
				class:active={currentPath === '/'}
				aria-current={currentPath === '/' ? 'page' : undefined}>Inbox</a
			>
			<a
				href={resolve('/today')}
				class:active={currentPath === '/today'}
				aria-current={currentPath === '/today' ? 'page' : undefined}>Today</a
			>
		</nav>

		<div class="session">
			<span class="username">{username}</span>
			<button type="button" disabled={signingOut} onclick={() => void signOut()}>
				{signingOut ? 'Signing out…' : 'Log out'}
			</button>
		</div>
	</header>

	<section class="workspace">
		{#if children}
			{@render children()}
		{/if}
		{#if errorMessage}
			<p class="error" role="alert">{errorMessage}</p>
		{/if}
	</section>
</main>

<style>
	.shell {
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		min-height: 100vh;
		padding: 1.5rem clamp(1.25rem, 4vw, 4rem);
	}

	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		font-weight: 700;
		letter-spacing: -0.02em;
	}

	nav {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		padding: 0.3rem;
		border: 1px solid #dfe5dc;
		border-radius: 0.8rem;
		background: rgb(255 255 255 / 48%);
	}

	nav a {
		padding: 0.5rem 0.7rem;
		border-radius: 0.55rem;
		color: #637068;
		font-size: 0.8rem;
		font-weight: 700;
		text-decoration: none;
	}

	nav a:hover,
	nav a.active {
		color: #2d6540;
		background: #fff;
	}

	.mark {
		display: grid;
		width: 2rem;
		height: 2rem;
		place-items: center;
		border-radius: 0.65rem;
		color: #fff;
		background: #2d6540;
	}

	.session {
		display: flex;
		align-items: center;
		gap: 0.85rem;
	}

	.username {
		color: #58625a;
		font-size: 0.88rem;
		font-weight: 650;
	}

	button {
		padding: 0.6rem 0.8rem;
		border: 1px solid #cbd5ca;
		border-radius: 0.65rem;
		color: #31523a;
		background: rgb(255 255 255 / 75%);
		font-size: 0.82rem;
		font-weight: 700;
		cursor: pointer;
	}

	button:hover:not(:disabled) {
		border-color: #9fb3a0;
		background: #fff;
	}

	button:disabled {
		cursor: wait;
		opacity: 0.6;
	}

	.workspace {
		padding: clamp(3.5rem, 9vw, 7rem) 0;
	}

	.error {
		margin: 1rem 0 0;
		color: #8c2828;
		font-size: 0.86rem;
	}

	@media (max-width: 34rem) {
		header {
			flex-wrap: wrap;
		}

		nav {
			order: 3;
			width: 100%;
		}

		nav a {
			flex: 1;
			text-align: center;
		}

		.username {
			display: none;
		}
	}
</style>
