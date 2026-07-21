import { describe, expect, it } from 'vitest';
import {
	findShortcutCommand,
	formatShortcut,
	isApplePlatform,
	matchesShortcut,
	shortcutCommand,
	shortcutCommands
} from './registry';

describe('keyboard shortcut registry', () => {
	it('contains every product command in one registry', () => {
		expect(shortcutCommands.map((command) => command.id)).toEqual([
			'quick-add',
			'toggle-chat',
			'project-overview',
			'project-inbox',
			'project-today',
			'project-tasks',
			'project-activity',
			'project-settings',
			'toggle-help'
		]);
		expect(new Set(shortcutCommands.map((command) => command.code)).size).toBe(
			shortcutCommands.length
		);
	});

	it('detects the platform modifier and formats it for help', () => {
		expect(isApplePlatform('MacIntel')).toBe(true);
		expect(isApplePlatform('Win32')).toBe(false);
		expect(formatShortcut(shortcutCommand('quick-add'), true)).toBe('Cmd + N');
		expect(formatShortcut(shortcutCommand('quick-add'), false)).toBe('Ctrl + N');
	});

	it('matches only the exact primary modifier and physical key', () => {
		const command = shortcutCommand('project-today');
		expect(matchesShortcut(keyboardEvent({ code: 'Digit3', metaKey: true }), command, true)).toBe(
			true
		);
		expect(matchesShortcut(keyboardEvent({ code: 'Digit3', ctrlKey: true }), command, false)).toBe(
			true
		);
		expect(matchesShortcut(keyboardEvent({ code: 'Numpad3', metaKey: true }), command, true)).toBe(
			false
		);
		expect(
			matchesShortcut(
				keyboardEvent({ code: 'Digit3', metaKey: true, shiftKey: true }),
				command,
				true
			)
		).toBe(false);
		expect(
			matchesShortcut(
				keyboardEvent({ code: 'Digit3', ctrlKey: true, altKey: true }),
				command,
				false
			)
		).toBe(false);
		expect(
			matchesShortcut(
				keyboardEvent({ code: 'Digit3', metaKey: true, ctrlKey: true }),
				command,
				true
			)
		).toBe(false);
	});

	it('ignores composition, repeat, and events handled by nested surfaces', () => {
		const command = shortcutCommand('quick-add');
		for (const conflict of [{ isComposing: true }, { repeat: true }, { defaultPrevented: true }]) {
			expect(
				matchesShortcut(keyboardEvent({ code: 'KeyN', ctrlKey: true, ...conflict }), command, false)
			).toBe(false);
		}
	});

	it('resolves a command without waiting for a second key', () => {
		expect(findShortcutCommand(keyboardEvent({ code: 'Slash', ctrlKey: true }), false)?.id).toBe(
			'toggle-help'
		);
	});
});

function keyboardEvent(overrides: Partial<KeyboardEvent> = {}) {
	return {
		altKey: false,
		code: '',
		ctrlKey: false,
		defaultPrevented: false,
		isComposing: false,
		metaKey: false,
		repeat: false,
		shiftKey: false,
		...overrides
	};
}
