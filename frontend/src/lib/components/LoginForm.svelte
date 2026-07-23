<script lang="ts">
	import { InvalidCredentialsError, login, type LoginCredentials } from '$lib/auth/client';

	interface Props {
		onAuthenticated: () => void | Promise<void>;
		authenticate?: (fetcher: typeof fetch, credentials: LoginCredentials) => Promise<void>;
	}

	let { onAuthenticated, authenticate = login }: Props = $props();
	let username = $state('');
	let password = $state('');
	let submitting = $state(false);
	let errorMessage = $state('');

	async function submit() {
		submitting = true;
		errorMessage = '';

		try {
			await authenticate(fetch, { login: username, password });
			await onAuthenticated();
		} catch (error) {
			errorMessage =
				error instanceof InvalidCredentialsError
					? 'The username or password is incorrect.'
					: 'Sign in is temporarily unavailable. Please try again.';
		} finally {
			submitting = false;
		}
	}
</script>

<main class="login-page">
	<section class="login-card" aria-labelledby="login-heading">
		<div class="brand" aria-label="Todai">
			<span class="mark" aria-hidden="true">T</span>
			<span>Todai</span>
		</div>

		<div class="intro">
			<p class="eyebrow">Personal workspace</p>
			<h1 id="login-heading">Welcome back.</h1>
			<p>Sign in to continue to your tasks.</p>
		</div>

		<form
			onsubmit={(event) => {
				event.preventDefault();
				void submit();
			}}
		>
			<label for="username">Username</label>
			<input
				id="username"
				name="username"
				type="text"
				autocomplete="username"
				bind:value={username}
				required
			/>

			<label for="password">Password</label>
			<input
				id="password"
				name="password"
				type="password"
				autocomplete="current-password"
				bind:value={password}
				required
			/>

			{#if errorMessage}
				<p class="error" role="alert">{errorMessage}</p>
			{/if}

			<button type="submit" disabled={submitting}>
				{submitting ? 'Signing in…' : 'Sign in'}
			</button>
		</form>
	</section>
</main>

<style>
	.login-page {
		display: grid;
		min-height: 100vh;
		place-items: center;
		padding: 1.5rem;
		background:
			radial-gradient(
				circle at 15% 15%,
				color-mix(in srgb, var(--theme-accent) 18%, transparent),
				transparent 34rem
			),
			var(--color-canvas);
	}

	.login-card {
		width: min(27rem, 100%);
		padding: clamp(1.75rem, 6vw, 3rem);
		border: 1px solid var(--color-border);
		border-radius: 1.5rem;
		background: color-mix(in srgb, var(--color-surface) 92%, transparent);
		box-shadow: var(--shadow-modal);
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		font-weight: 750;
		letter-spacing: -0.02em;
	}

	.mark {
		display: grid;
		width: 2rem;
		height: 2rem;
		place-items: center;
		border-radius: 0.65rem;
		color: var(--color-on-accent);
		background: var(--theme-accent-solid, var(--theme-accent));
	}

	.intro {
		margin: 3rem 0 2rem;
	}

	.eyebrow {
		margin: 0 0 0.65rem;
		color: var(--theme-accent);
		font-size: 0.76rem;
		font-weight: 750;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: clamp(2.25rem, 10vw, 3.5rem);
		line-height: 1;
		letter-spacing: -0.055em;
	}

	.intro > p:last-child {
		margin: 1rem 0 0;
		color: var(--color-text-secondary);
	}

	form {
		display: grid;
		gap: 0.65rem;
	}

	label {
		margin-top: 0.7rem;
		font-size: 0.85rem;
		font-weight: 700;
	}

	input {
		width: 100%;
		padding: 0.85rem 0.95rem;
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		color: var(--color-text);
		background: var(--color-surface);
		outline: none;
		transition:
			border-color 120ms ease,
			box-shadow 120ms ease;
	}

	input:focus {
		border-color: var(--theme-accent);
		box-shadow: 0 0 0 0.2rem var(--theme-focus);
	}

	button {
		margin-top: 1rem;
		padding: 0.9rem 1rem;
		border: 0;
		border-radius: 0.75rem;
		color: var(--color-on-accent);
		background: var(--theme-accent-solid, var(--theme-accent));
		font-weight: 750;
		cursor: pointer;
	}

	button:hover:not(:disabled) {
		background: color-mix(in srgb, var(--theme-accent-solid, var(--theme-accent)) 86%, black);
	}

	button:disabled {
		cursor: wait;
		opacity: 0.65;
	}

	.error {
		margin: 0.5rem 0 0;
		padding: 0.75rem 0.85rem;
		border-radius: 0.65rem;
		color: var(--color-error);
		background: var(--color-error-soft);
		font-size: 0.85rem;
		line-height: 1.45;
	}
</style>
