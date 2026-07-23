import { page } from 'vitest/browser';

// Vitest's provider commands are delivered over a WebSocket RPC channel. Under Bun,
// the command response can arrive before Playwright has entered the tester iframe,
// so locator actions target the harness instead of the component. Keep Playwright
// as the browser provider, but perform the affected interactions in the tester
// frame where the locators and application event listeners live.
{
	const locator = page.getByTestId('__todai_locator_prototype__');
	const prototype = findLocatorPrototype(locator);

	prototype.click = async function () {
		await browserTask();
		const element = await this.findElement();
		if (element instanceof HTMLElement) {
			element.click();
		} else {
			element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
		}
		await browserTask();
	};
	prototype.fill = async function (text: string) {
		await browserTask();
		const element = await this.findElement();
		if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
			element.focus();
			element.value = text;
		} else if (element instanceof HTMLElement && element.isContentEditable) {
			element.focus();
			element.textContent = text;
		} else {
			throw new TypeError('fill requires an input, textarea, or contenteditable element');
		}
		element.dispatchEvent(
			new InputEvent('input', { bubbles: true, inputType: 'insertText', data: text })
		);
		await browserTask();
	};
	prototype.clear = async function () {
		await this.fill('');
	};
	prototype.selectOptions = async function (values: string | string[]) {
		await browserTask();
		const element = await this.findElement();
		if (!(element instanceof HTMLSelectElement)) {
			throw new TypeError('selectOptions requires a select element');
		}
		const selected = new Set(Array.isArray(values) ? values : [values]);
		for (const option of element.options) {
			option.selected = selected.has(option.value) || selected.has(option.text);
		}
		element.dispatchEvent(new InputEvent('input', { bubbles: true }));
		element.dispatchEvent(new Event('change', { bubbles: true }));
		await browserTask();
	};
}

interface LocatorPrototype {
	findElement(): Promise<HTMLElement | SVGElement>;
	click(): Promise<void>;
	fill(text: string): Promise<void>;
	clear(): Promise<void>;
	selectOptions(values: string | string[]): Promise<void>;
}

function findLocatorPrototype(locator: unknown): LocatorPrototype {
	let prototype = Object.getPrototypeOf(locator) as LocatorPrototype | null;
	while (prototype !== null && !Object.hasOwn(prototype, 'click')) {
		prototype = Object.getPrototypeOf(prototype) as LocatorPrototype | null;
	}
	if (prototype === null) {
		throw new Error('Vitest browser locator prototype was not found');
	}
	return prototype;
}

function browserTask(): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, 0));
}
