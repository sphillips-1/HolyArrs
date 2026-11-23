<script lang="ts">
	import { onMount } from 'svelte';
	import LibraryToolPage from '#lib/mediafiles/LibraryToolPage.svelte';
	import Button from '#lib/ui/Button.svelte';
	import EmptyState from '#lib/ui/EmptyState.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import { api, message } from '#lib/core/api.ts';
	import { to } from '#lib/core/paths.ts';
	import { session } from '#lib/core/session.svelte.ts';

	interface Catalog {
		items: { id: string; title: string; author: string; narrator: string }[];
		total: number;
		limit: number;
	}

	let catalog = $state<Catalog>();
	let page = $state(0);
	let busy = $state(false);
	let error = $state('');

	async function load(index: number) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			catalog = await api.get<Catalog>(`/audiobooks/library?page=${index}`);
			page = index;
		} catch (cause) {
			error = message(cause);
		} finally {
			busy = false;
		}
	}

	onMount(() => { if (session.s.role === 'owner') void load(0); });
</script>

<svelte:head><title>Audiobook Library · HolyArrs</title></svelte:head>

<LibraryToolPage title="Audiobooks" count={catalog ? String(catalog.total) : ''}>
	{#snippet bar()}
		{#if session.s.role === 'owner'}
			<span class="tp-gap"></span>
			<Button href={to('/audiobooks')} variant="primary" icon="search">Find audiobooks</Button>
		{/if}
	{/snippet}
	{#if session.s.role !== 'owner'}
		<EmptyState icon="audiobook" title="Sign in as the owner to open Audiobooks" />
	{:else}
		{#if error}<p role="alert">{error} <Button size="sm" onclick={() => load(page)}>Retry</Button></p>{/if}
		<div class="tp-sheet catalog" aria-busy={busy}>
			{#if catalog?.items.length}
				<table class="tp-table">
					<thead><tr><th>Title</th><th>Author</th><th>Narrator</th></tr></thead>
					<tbody>
						{#each catalog.items as book (book.id)}
							<tr>
								<td>
									<a class="tp-who" href={`https://audiobooks.shelfgoblin.dev/audiobookshelf/item/${encodeURIComponent(book.id)}`}>
										<span class="tp-art book-icon"><Icon name="audiobook" size={18} /></span>
										<span class="tp-tx">{book.title}</span>
									</a>
								</td>
								<td>{book.author || '—'}</td><td>{book.narrator || '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{:else if busy}
				<EmptyState icon="audiobook" title="Loading Audiobookshelf" />
			{:else if catalog}
				<EmptyState icon="audiobook" title="No audiobooks on this page" />
			{/if}
		</div>
		{#if catalog && catalog.total > catalog.limit}
			<nav aria-label="Audiobook library pages">
				<Button disabled={busy || page === 0} onclick={() => load(page - 1)}>Previous</Button>
				<span>Page {page + 1} of {Math.ceil(catalog.total / catalog.limit)}</span>
				<Button disabled={busy || (page + 1) * catalog.limit >= catalog.total} onclick={() => load(page + 1)}>Next</Button>
			</nav>
		{/if}
	{/if}
</LibraryToolPage>

<style>
	.catalog { overflow-x: auto; }
	.book-icon { display: grid; place-items: center; aspect-ratio: 1; color: var(--text-muted); background: var(--surface-2); }
	@media (max-width: 639px) {
		.catalog .tp-who { gap: 8px; }
		.catalog .book-icon { width: 28px; }
	}
	nav { display: flex; gap: 12px; align-items: center; justify-content: end; }
</style>
