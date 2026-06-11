<script lang="ts">
	import { onMount } from 'svelte';
	import { toasts, dismissToast } from '$lib/toast';
	import { fade, fly } from 'svelte/transition';
	import { X } from 'lucide-svelte';

	let toastContainer: HTMLElement;

	function handleKeydown(e: KeyboardEvent, id: string) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			dismissToast(id);
		}
	}

	function handleClose(e: MouseEvent, id: string) {
		e.stopPropagation();
		dismissToast(id);
	}
</script>

<div bind:this={toastContainer} class="toast-container">
	{#each $toasts as toast (toast.id)}
		<div
			class="toast"
			class:toast-success={toast.type === 'success'}
			class:toast-error={toast.type === 'error'}
			class:toast-info={toast.type === 'info'}
			class:toast-warning={toast.type === 'warning'}
			transition:fly={{ y: -20, duration: 300 }}
			role="button"
			aria-label="Dismiss notification"
			tabindex="0"
			onclick={() => dismissToast(toast.id)}
			onkeydown={(e) => handleKeydown(e, toast.id)}
		>
			{toast.message}
			<button
				class="toast-close"
				onclick={(e) => handleClose(e, toast.id)}
			>
				<X size={16} />
			</button>
		</div>
	{/each}
</div>

<style>
	.toast-container {
		position: fixed;
		top: 5rem;
		left: 256px;
		z-index: 1100;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		align-items: flex-end;
		pointer-events: none;
	}

	@media (max-width: 767px) {
		.toast-container {
			left: 1rem;
			right: 1rem;
		}
	}

	.toast {
		position: relative;
		min-width: 300px;
		max-width: 400px;
		padding: 1rem;
		border-radius: var(--radius-sm);
		box-shadow: var(--shadow-md);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-weight: 500;
		transition: opacity 0.3s var(--ease-out);
		color: var(--text-primary);
		pointer-events: auto;
	}

	.toast-close {
		background: none;
		border: none;
		color: var(--text-tertiary);
		cursor: pointer;
		padding: 0.25rem;
		font-size: 1rem;
		transition: color var(--duration-fast) var(--ease-out);
	}

	.toast-close:hover {
		color: var(--text-primary);
	}

	.toast-success {
		background-color: var(--color-success);
	}

	.toast-error {
		background-color: var(--color-danger);
	}

	.toast-info {
		background-color: var(--color-primary);
	}

	.toast-warning {
		background-color: var(--color-warning);
	}
</style>