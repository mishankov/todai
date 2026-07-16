import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { InvalidCredentialsError } from '$lib/auth/client';
import LoginForm from './LoginForm.svelte';

describe('LoginForm', () => {
	it('submits the entered credentials', async () => {
		const authenticate = vi.fn(async () => {});
		const onAuthenticated = vi.fn(async () => {});
		render(LoginForm, { authenticate, onAuthenticated });

		await page.getByLabelText('Username').fill('owner');
		await page.getByLabelText('Password').fill('correct horse battery staple');
		await page.getByRole('button', { name: 'Sign in' }).click();

		expect(authenticate).toHaveBeenCalledWith(expect.any(Function), {
			login: 'owner',
			password: 'correct horse battery staple'
		});
		expect(onAuthenticated).toHaveBeenCalledOnce();
	});

	it('shows a safe error for invalid credentials', async () => {
		const authenticate = vi.fn(async () => {
			throw new InvalidCredentialsError();
		});
		const onAuthenticated = vi.fn(async () => {});
		render(LoginForm, { authenticate, onAuthenticated });

		await page.getByLabelText('Username').fill('owner');
		await page.getByLabelText('Password').fill('wrong password');
		await page.getByRole('button', { name: 'Sign in' }).click();

		await expect
			.element(page.getByRole('alert'))
			.toHaveTextContent('The username or password is incorrect.');
		expect(onAuthenticated).not.toHaveBeenCalled();
	});
});
