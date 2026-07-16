<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

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
		color: #292927;
		background: #f5f7f3;
	}

	:global(body) {
		margin: 0;
		min-width: 320px;
		min-height: 100vh;
	}

	:global(button),
	:global(input) {
		font: inherit;
	}
</style>
