<script module lang="ts">
	let nextQuickPickId = 0;
	function createQuickPickId(): number {
		nextQuickPickId += 1;
		return nextQuickPickId;
	}
</script>

<script lang="ts">
	import type { QuickPickItem } from '$lib/tasks/quick-picks';
	import { tick } from 'svelte';

	interface Props {
		label: string;
		buttonText: string;
		items: QuickPickItem[];
		value?: string;
		select: (value: string) => void;
		disabled?: boolean;
		searchable?: boolean;
		searchPlaceholder?: string;
		customInput?: 'date' | 'time';
		customValue?: string;
		align?: 'start' | 'end';
		refresh?: () => void;
	}

	let {
		label,
		buttonText,
		items,
		value = '',
		select,
		disabled = false,
		searchable = false,
		searchPlaceholder = 'Search',
		customInput,
		customValue = '',
		align = 'start',
		refresh
	}: Props = $props();
	let wrapper: HTMLDivElement;
	let trigger: HTMLButtonElement;
	let panel = $state<HTMLDivElement>();
	let customControl = $state<HTMLInputElement>();
	let open = $state(false);
	let query = $state('');
	let activeIndex = $state(0);
	let panelStyle = $state('');
	const listboxId = `quick-pick-${createQuickPickId()}`;
	let filteredItems = $derived(
		query
			? items.filter((item) =>
					`${item.label} ${item.detail ?? ''}`
						.toLocaleLowerCase()
						.includes(query.toLocaleLowerCase())
				)
			: items
	);

	$effect(() => {
		const itemCount = filteredItems.length;
		if (activeIndex >= itemCount) activeIndex = Math.max(0, itemCount - 1);
	});

	async function show(direction: 1 | -1 = 1) {
		if (disabled) return;
		refresh?.();
		open = true;
		query = '';
		const selectedIndex = items.findIndex((item) => item.id === value);
		activeIndex = selectedIndex >= 0 ? selectedIndex : direction === 1 ? 0 : items.length - 1;
		await tick();
		positionPanel();
		if (searchable) {
			panel?.querySelector<HTMLInputElement>('input[type="search"]')?.focus();
		} else {
			focusActive();
		}
	}

	function hide(restoreFocus = false) {
		open = false;
		query = '';
		if (restoreFocus) queueMicrotask(() => trigger.focus());
	}

	function choose(item: QuickPickItem) {
		if (item.custom && customInput) {
			customControl?.focus();
			customControl?.showPicker?.();
			return;
		}
		select(item.id);
		hide(true);
	}

	function move(step: 1 | -1) {
		if (filteredItems.length === 0) return;
		activeIndex = (activeIndex + step + filteredItems.length) % filteredItems.length;
		void tick().then(focusActive);
	}

	function focusActive() {
		panel?.querySelector<HTMLButtonElement>(`[data-option-index="${activeIndex}"]`)?.focus();
	}

	function handleTriggerKeydown(event: KeyboardEvent) {
		if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
		event.preventDefault();
		void show(event.key === 'ArrowDown' ? 1 : -1);
	}

	function handlePanelKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
			hide(true);
			return;
		}
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			move(event.key === 'ArrowDown' ? 1 : -1);
			return;
		}
		if ((event.key === 'Enter' || event.key === ' ') && !isTextInput(event.target)) {
			event.preventDefault();
			const item = filteredItems[activeIndex];
			if (item) choose(item);
		}
	}

	function isTextInput(target: EventTarget | null): boolean {
		return target instanceof HTMLInputElement && target.type === 'search';
	}

	function positionPanel() {
		if (!open || !trigger || !panel) return;
		const triggerRect = trigger.getBoundingClientRect();
		const margin = 8;
		const width = Math.min(320, window.innerWidth - margin * 2);
		const desiredLeft = align === 'end' ? triggerRect.right - width : triggerRect.left;
		const left = Math.max(margin, Math.min(desiredLeft, window.innerWidth - width - margin));
		const roomBelow = window.innerHeight - triggerRect.bottom - margin * 2;
		const roomAbove = triggerRect.top - margin * 2;
		const placeAbove = roomBelow < 220 && roomAbove > roomBelow;
		const maxHeight = Math.max(180, Math.min(360, placeAbove ? roomAbove : roomBelow));
		const top = placeAbove
			? Math.max(margin, triggerRect.top - Math.min(panel.scrollHeight, maxHeight) - margin)
			: Math.min(window.innerHeight - margin, triggerRect.bottom + 6);
		panelStyle = `left:${left}px;top:${top}px;width:${width}px;max-height:${maxHeight}px`;
	}
</script>

<svelte:window onresize={positionPanel} onscroll={positionPanel} />

<div
	bind:this={wrapper}
	class="quick-pick"
	onfocusout={(event) => {
		if (!wrapper.contains(event.relatedTarget as Node | null)) hide();
	}}
>
	<button
		bind:this={trigger}
		class="quick-pick-trigger"
		type="button"
		aria-label={`${label}: ${buttonText}`}
		aria-haspopup="listbox"
		aria-controls={open ? listboxId : undefined}
		aria-expanded={open}
		{disabled}
		onkeydown={handleTriggerKeydown}
		onclick={() => (open ? hide() : void show())}
	>
		<span>{buttonText}</span>
		<svg aria-hidden="true" viewBox="0 0 16 16"><path d="m4 6 4 4 4-4" /></svg>
	</button>

	{#if open}
		<div
			bind:this={panel}
			id={listboxId}
			class="quick-pick-panel"
			role="listbox"
			tabindex="-1"
			aria-label={label}
			style={panelStyle}
			onkeydown={handlePanelKeydown}
		>
			{#if searchable}
				<label class="search-row">
					<span class="sr-only">{searchPlaceholder}</span>
					<input
						type="search"
						placeholder={searchPlaceholder}
						aria-label={searchPlaceholder}
						bind:value={query}
					/>
				</label>
			{/if}

			{#if filteredItems.length === 0}
				<p class="empty">No matches</p>
			{:else}
				{#each filteredItems as item, index (item.id)}
					{#if item.group && (index === 0 || filteredItems[index - 1]?.group !== item.group)}
						<p class="group-label">{item.group}</p>
					{/if}
					<button
						class="quick-pick-option"
						class:active={index === activeIndex}
						type="button"
						role="option"
						aria-selected={item.id === value}
						data-option-index={index}
						tabindex={index === activeIndex ? 0 : -1}
						onmouseenter={() => (activeIndex = index)}
						onclick={() => choose(item)}
					>
						<span class="option-copy">
							<strong>{item.label}</strong>
							{#if item.detail}<small>{item.detail}</small>{/if}
						</span>
						{#if item.id === value}<span class="check" aria-hidden="true">✓</span>{/if}
					</button>
					{#if item.custom && customInput}
						<label class="custom-control">
							<span class="sr-only">{item.label}</span>
							{#if customInput === 'date'}
								<input
									bind:this={customControl}
									type="date"
									aria-label={item.label}
									value={customValue}
									onchange={(event) => {
										select(event.currentTarget.value);
										hide(true);
									}}
								/>
							{:else}
								<input
									bind:this={customControl}
									type="time"
									aria-label={item.label}
									value={customValue}
									onchange={(event) => {
										select(event.currentTarget.value);
										hide(true);
									}}
								/>
							{/if}
						</label>
					{/if}
				{/each}
			{/if}
		</div>
	{/if}
</div>

<style>
	.quick-pick {
		position: relative;
		min-width: 0;
	}
	.quick-pick-trigger {
		display: flex;
		width: 100%;
		min-width: 0;
		min-height: 2.25rem;
		align-items: center;
		justify-content: space-between;
		gap: 0.45rem;
		padding: 0.48rem 0.62rem;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.65rem;
		color: #29332c;
		background: #fff;
		font: inherit;
		font-size: 0.76rem;
		font-weight: 650;
		cursor: pointer;
	}
	.quick-pick-trigger > span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.quick-pick-trigger svg {
		width: 0.8rem;
		flex: none;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.5;
	}
	.quick-pick-trigger:hover:not(:disabled),
	.quick-pick-trigger:focus-visible {
		border-color: var(--theme-accent, #477d56);
		background: var(--theme-hover, #f3f7f2);
		outline: none;
		box-shadow: 0 0 0 0.18rem var(--theme-focus, rgb(71 125 86 / 12%));
	}
	.quick-pick-trigger:disabled {
		cursor: not-allowed;
		opacity: 0.48;
	}
	.quick-pick-panel {
		position: fixed;
		z-index: 100;
		box-sizing: border-box;
		overflow: auto;
		padding: 0.45rem;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.78rem;
		background: #fff;
		box-shadow: 0 1rem 3rem rgb(31 54 38 / 18%);
	}
	.search-row {
		display: block;
		position: sticky;
		top: -0.45rem;
		z-index: 1;
		padding: 0.35rem;
		background: #fff;
	}
	.search-row input {
		box-sizing: border-box;
		width: 100%;
		padding: 0.55rem 0.62rem;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.55rem;
		font: inherit;
		outline: none;
	}
	.search-row input:focus {
		border-color: var(--theme-accent, #477d56);
		box-shadow: 0 0 0 0.16rem var(--theme-focus, rgb(71 125 86 / 12%));
	}
	.group-label {
		margin: 0.5rem 0.48rem 0.2rem;
		color: #7d857f;
		font-size: 0.64rem;
		font-weight: 750;
		letter-spacing: 0.07em;
		text-transform: uppercase;
	}
	.quick-pick-option {
		display: flex;
		width: 100%;
		align-items: center;
		justify-content: space-between;
		gap: 0.65rem;
		padding: 0.58rem 0.62rem;
		border: 0;
		border-radius: 0.55rem;
		color: #253029;
		background: transparent;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	.quick-pick-option:hover,
	.quick-pick-option.active,
	.quick-pick-option:focus-visible {
		background: var(--theme-hover, #eff5ef);
		outline: none;
	}
	.option-copy {
		display: grid;
		min-width: 0;
		gap: 0.12rem;
	}
	.option-copy strong {
		font-size: 0.78rem;
		font-weight: 680;
	}
	.option-copy small {
		color: #727a74;
		font-size: 0.7rem;
	}
	.check {
		color: var(--theme-accent, #2d6540);
		font-weight: 800;
	}
	.custom-control {
		display: block;
		padding: 0 0.62rem 0.5rem;
	}
	.custom-control input {
		box-sizing: border-box;
		width: 100%;
		padding: 0.45rem 0.55rem;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.5rem;
		font: inherit;
	}
	.empty {
		margin: 0;
		padding: 1rem;
		color: #737b75;
		font-size: 0.76rem;
		text-align: center;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
	@media (max-width: 34rem) {
		.quick-pick-panel {
			top: auto !important;
			right: 0 !important;
			bottom: 0;
			left: 0 !important;
			width: 100% !important;
			max-height: min(72vh, 30rem) !important;
			padding: 0.75rem;
			border-radius: 1rem 1rem 0 0;
		}
		.quick-pick-option {
			min-height: 2.9rem;
		}
	}
</style>
