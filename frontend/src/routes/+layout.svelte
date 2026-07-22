<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '$lib/appearance/theme.css';
	import SystemAppearanceController from '$lib/appearance/SystemAppearanceController.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<SystemAppearanceController />
{@render children()}

<style>
	:global(*) {
		box-sizing: border-box;
	}

	:global(html) {
		font-family:
			Inter,
			ui-sans-serif,
			system-ui,
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			sans-serif;
		color: var(--color-text);
		background: var(--color-canvas);
	}

	:global(body) {
		margin: 0;
		min-width: 320px;
		min-height: 100vh;
	}

	:global(button),
	:global(input),
	:global(select),
	:global(textarea) {
		font: inherit;
	}

	:global(::placeholder) {
		color: var(--color-placeholder);
	}
</style>
