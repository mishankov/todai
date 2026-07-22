<script lang="ts">
	import { updateSettings } from '$lib/settings/client';
	import { publishSavedAppearance, type Appearance } from '$lib/appearance/theme';
	import { untrack } from 'svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const initial = initialForm();
	let current = $state(initial.settings);
	let timezone = $state(initial.timezone);
	let appearance = $state<Appearance>(initial.appearance);
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
			appearance = next.appearance;
			saved = false;
		});
	});

	function initialForm() {
		const settings = data.settings.settings;
		return {
			settings,
			timezone: settings.timezone ?? detectedTimezone(),
			appearance: settings.appearance
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
				appearance,
				version: current.version
			});
			current = updated.settings;
			timezone = updated.settings.timezone ?? timezone;
			appearance = updated.settings.appearance;
			publishSavedAppearance(updated.settings.appearance);
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

		<section class="settings-group" aria-labelledby="appearance-settings">
			<div>
				<h2 id="appearance-settings">Appearance</h2>
				<p>Choose how Todai looks across every project and device.</p>
			</div>
			<fieldset class="appearance-options">
				<legend>Color scheme</legend>
				{#each [{ id: 'system', label: 'System', description: 'Follow this device' }, { id: 'light', label: 'Light', description: 'Always use light' }, { id: 'dark', label: 'Dark', description: 'Always use dark' }] as option (option.id)}
					<label class="appearance-option">
						<input type="radio" name="appearance" value={option.id} bind:group={appearance} />
						<span><strong>{option.label}</strong><small>{option.description}</small></span>
					</label>
				{/each}
			</fieldset>
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
		color: var(--color-text);
	}

	header {
		margin-bottom: 2.5rem;
	}

	header p {
		margin: 0 0 0.55rem;
		color: var(--theme-accent);
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
		color: var(--color-text-secondary);
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
		border: 1px solid var(--color-border);
		border-radius: 0.9rem;
		background: var(--color-surface);
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
		border: 1px solid var(--color-border-strong);
		border-radius: 0.55rem;
		color: var(--color-text);
		background: var(--color-control);
		font: inherit;
		font-size: 0.88rem;
	}

	select:focus {
		outline: 3px solid var(--theme-focus, var(--color-focus));
		border-color: var(--theme-accent);
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
		color: var(--color-error);
	}
	.success {
		color: var(--theme-accent);
	}

	button {
		padding: 0.72rem 1rem;
		border: 0;
		border-radius: 0.55rem;
		color: var(--color-on-accent);
		background: var(--theme-accent-solid, var(--theme-accent));
		font-weight: 750;
		cursor: pointer;
	}

	button:disabled {
		cursor: wait;
		opacity: 0.55;
	}

	.appearance-options {
		display: grid;
		gap: 0.55rem;
		margin: 0;
		padding: 0;
		border: 0;
	}

	.appearance-options legend {
		margin-bottom: 0.5rem;
		font-size: 0.78rem;
		font-weight: 750;
	}

	.appearance-option {
		display: flex;
		grid-template-columns: none;
		align-items: center;
		gap: 0.7rem;
		padding: 0.7rem;
		border: 1px solid var(--color-border);
		border-radius: 0.6rem;
		background: var(--color-control);
		cursor: pointer;
	}

	.appearance-option:has(input:checked) {
		border-color: var(--theme-accent);
		background: var(--theme-accent-soft);
		box-shadow: 0 0 0 2px var(--theme-focus);
	}

	.appearance-option input {
		width: 1rem;
		height: 1rem;
		accent-color: var(--theme-accent-solid, var(--theme-accent));
	}

	.appearance-option span {
		display: grid;
		gap: 0.12rem;
	}

	.appearance-option small {
		color: var(--color-text-secondary);
		font-size: 0.72rem;
		font-weight: 500;
	}

	@media (max-width: 42rem) {
		.settings-group {
			grid-template-columns: 1fr;
			gap: 1.2rem;
		}
	}
</style>
