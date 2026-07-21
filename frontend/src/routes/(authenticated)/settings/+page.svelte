<script lang="ts">
	import { updateSettings } from '$lib/settings/client';
	import { untrack } from 'svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const initial = initialForm();
	let current = $state(initial.settings);
	let timezone = $state(initial.timezone);
	let saving = $state(false);
	let saved = $state(false);
	let errorMessage = $state('');

	const timezones = supportedTimezones(initial.timezone);

	$effect(() => {
		const next = data.settings.settings;
		untrack(() => {
			if (next.version === current.version) return;
			current = next;
			timezone = next.timezone ?? detectedTimezone();
			saved = false;
		});
	});

	function initialForm() {
		const settings = data.settings.settings;
		return {
			settings,
			timezone: settings.timezone ?? detectedTimezone()
		};
	}

	async function save() {
		saving = true;
		saved = false;
		errorMessage = '';
		try {
			const updated = await updateSettings(fetch, {
				timezone,
				agentModel: current.agentModel,
				agentThinkingEffort: current.agentThinkingEffort,
				version: current.version
			});
			current = updated.settings;
			timezone = updated.settings.timezone ?? timezone;
			saved = true;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Could not save settings.';
		} finally {
			saving = false;
		}
	}

	function detectedTimezone(): string {
		return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
	}

	function supportedTimezones(currentTimezone: string): string[] {
		const intl = Intl as typeof Intl & { supportedValuesOf?: (key: 'timeZone') => string[] };
		const values = intl.supportedValuesOf?.('timeZone') ?? ['UTC'];
		return values.includes(currentTimezone) ? values : [currentTimezone, ...values];
	}
</script>

<svelte:head>
	<title>Settings — Todai</title>
</svelte:head>

<section class="settings-page" aria-labelledby="settings-title">
	<header>
		<p>ACCOUNT</p>
		<h1 id="settings-title">Settings</h1>
		<span>Personal preferences shared across all of your projects.</span>
	</header>

	<form
		onsubmit={(event) => {
			event.preventDefault();
			void save();
		}}
	>
		<section class="settings-group" aria-labelledby="general-settings">
			<div>
				<h2 id="general-settings">Time and dates</h2>
				<p>Controls Today and how the agent interprets relative dates.</p>
			</div>
			<label>
				<span>Time zone</span>
				<select bind:value={timezone}>
					{#each timezones as option (option)}
						<option value={option}>{option}</option>
					{/each}
				</select>
			</label>
		</section>

		<footer>
			{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}
			{#if saved}<p class="success" role="status">Settings saved.</p>{/if}
			<button type="submit" disabled={saving || !timezone}>
				{saving ? 'Saving…' : 'Save changes'}
			</button>
		</footer>
	</form>
</section>

<style>
	.settings-page {
		width: min(48rem, 100%);
		margin: 0 auto;
		color: #292927;
	}

	header {
		margin-bottom: 2.5rem;
	}

	header p {
		margin: 0 0 0.55rem;
		color: var(--theme-accent, #4f765c);
		font-size: 0.75rem;
		font-weight: 800;
		letter-spacing: 0.12em;
	}

	h1 {
		margin: 0 0 0.65rem;
		font-size: clamp(2.2rem, 6vw, 3.4rem);
		letter-spacing: -0.055em;
		line-height: 1;
	}

	header span,
	.settings-group p {
		color: #74746f;
		font-size: 0.9rem;
	}

	form {
		display: grid;
		gap: 1rem;
	}

	.settings-group {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(15rem, 19rem);
		gap: 2.5rem;
		align-items: center;
		padding: 1.5rem;
		border: 1px solid #dce4d9;
		border-radius: 0.9rem;
		background: #fff;
	}

	h2 {
		margin: 0 0 0.4rem;
		font-size: 1.05rem;
	}

	.settings-group p {
		margin: 0;
		line-height: 1.5;
	}

	label {
		display: grid;
		gap: 0.5rem;
		font-size: 0.78rem;
		font-weight: 750;
	}

	select {
		width: 100%;
		min-height: 2.8rem;
		padding: 0 0.8rem;
		border: 1px solid #cdd9ca;
		border-radius: 0.55rem;
		color: #292927;
		background: var(--theme-canvas, #fbfcfa);
		font: inherit;
		font-size: 0.88rem;
	}

	select:focus {
		outline: 3px solid rgb(45 101 64 / 16%);
		border-color: var(--theme-accent, #4f765c);
	}

	footer {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 1rem;
		min-height: 3rem;
	}

	footer p {
		margin: 0 auto 0 0;
		font-size: 0.82rem;
	}

	.error {
		color: #b83f34;
	}
	.success {
		color: var(--theme-accent, #2d6540);
	}

	button {
		padding: 0.72rem 1rem;
		border: 0;
		border-radius: 0.55rem;
		color: #fff;
		background: var(--theme-accent, #2d6540);
		font-weight: 750;
		cursor: pointer;
	}

	button:disabled {
		cursor: wait;
		opacity: 0.55;
	}

	@media (max-width: 42rem) {
		.settings-group {
			grid-template-columns: 1fr;
			gap: 1.2rem;
		}
	}
</style>
