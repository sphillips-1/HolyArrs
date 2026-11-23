<script lang="ts">
	import { onMount } from 'svelte';
	import LibraryToolPage from '#lib/mediafiles/LibraryToolPage.svelte';
	import Button from '#lib/ui/Button.svelte';
	import EmptyState from '#lib/ui/EmptyState.svelte';
	import Poster from '#lib/ui/Poster.svelte';
	import { api, message } from '#lib/core/api.ts';
	import { to } from '#lib/core/paths.ts';
	import { session } from '#lib/core/session.svelte.ts';
	import { registerField } from '#lib/core/search-field.svelte.ts';

	type Book = { id: string; title: string; author: string; narrator: string };

	interface Catalog {
		items: Book[];
		total: number;
		limit: number;
	}

	let catalog = $state<Catalog>();
	let page = $state(0);
	let busy = $state(false);
	let error = $state('');
	let field = $state<HTMLInputElement>();
	$effect(() => field ? registerField(field) : undefined);
	let query = $state('');
	let searchItems = $state<Book[]>();
	let searchPage = $state(0);
	let searchBusy = $state(false);
	let searchError = $state('');
	let searchRequest: AbortController | undefined;
	const words = $derived(query.trim().toLowerCase().split(/\s+/).filter(Boolean));
	const searching = $derived(words.length > 0);
	const matches = $derived(searchItems?.filter((book) => {
		const text = [book.title, book.author, book.narrator].join(' ').toLowerCase();
		return words.every((word) => text.includes(word));
	}));
	const limit = $derived(catalog?.limit ?? 50);
	const visible = $derived(searching ? matches?.slice(searchPage * limit, (searchPage + 1) * limit) : catalog?.items);
	const total = $derived(searching ? matches?.length : catalog?.total);
	const currentPage = $derived(searching ? searchPage : page);
	const pending = $derived(searching ? searchBusy : busy);

	async function loadSearch() {
		if (searchItems || searchRequest) return;
		const request = new AbortController();
		searchRequest = request;
		searchBusy = true;
		searchError = '';
		try {
			const first = await api.get<Catalog>('/audiobooks/library?page=0', request.signal);
			const items = new Map(first.items.map((book) => [book.id, book]));
			const pages = Math.ceil(first.total / first.limit);
			for (let index = 1; index < pages; index++) {
				const next = await api.get<Catalog>(`/audiobooks/library?page=${index}`, request.signal);
				for (const book of next.items) items.set(book.id, book);
			}
			if (!request.signal.aborted) searchItems = [...items.values()];
		} catch (cause) {
			if (!request.signal.aborted) searchError = message(cause);
		} finally {
			searchRequest = undefined;
			searchBusy = false;
		}
	}

	function updateSearch(value: string) {
		query = value;
		searchPage = 0;
		if (value.trim()) void loadSearch();
	}

	function changePage(index: number) {
		if (searching) searchPage = index;
		else void load(index);
	}

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

	onMount(() => {
		if (session.s.role === 'owner') void load(0);
		return () => searchRequest?.abort();
	});
</script>

<svelte:head><title>Audiobook Library · HolyArrs</title></svelte:head>

<LibraryToolPage title="Audiobooks" count={total === undefined ? '' : searching && catalog ? `${total} of ${catalog.total}` : String(total)}>
	{#snippet bar()}
		{#if session.s.role === 'owner'}
			<span class="tp-gap"></span>
			<label class="sr-only" for="audiobook-library-search">Search audiobook library</label>
			<input class="input library-search" id="audiobook-library-search" type="search" bind:this={field} value={query}
				oninput={(event) => updateSearch(event.currentTarget.value)}
				onkeydown={(event) => {
					if (event.key === 'Escape' && query) {
						event.preventDefault();
						updateSearch('');
					}
				}}
				placeholder="Search audiobook library" autocomplete="off" />
			<Button href={to('/audiobooks')} variant="primary" icon="search">Find audiobooks</Button>
		{/if}
	{/snippet}
	{#if session.s.role !== 'owner'}
		<EmptyState icon="audiobook" title="Sign in as the owner to open Audiobooks" />
	{:else}
		{#if searching && searchError}<p role="alert">{searchError} <Button size="sm" onclick={loadSearch}>Retry</Button></p>
		{:else if !searching && error}<p role="alert">{error} <Button size="sm" onclick={() => load(page)}>Retry</Button></p>{/if}
		<div class="tp-sheet catalog" aria-busy={pending}>
			{#if visible?.length}
				<table class="tp-table">
					<thead><tr><th>Title</th><th>Author</th><th>Narrator</th></tr></thead>
					<tbody>
						{#each visible as book (book.id)}
							<tr>
								<td>
									<a class="tp-who" href={`https://audiobooks.shelfgoblin.dev/audiobookshelf/item/${encodeURIComponent(book.id)}`}>
										<span class="tp-art book-cover" style="--poster-ratio: 1">
											<Poster src={`/api/v1/audiobooks/cover/${encodeURIComponent(book.id)}`} name={book.title} />
										</span>
										<span class="tp-tx">{book.title}</span>
									</a>
								</td>
								<td>{book.author || '—'}</td><td>{book.narrator || '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{:else if pending}
				<EmptyState icon="audiobook" title={searching ? 'Loading the audiobook catalog' : 'Loading Audiobookshelf'} />
			{:else if searching && matches}
				<EmptyState icon="search" title="No matching audiobooks" />
			{:else if !searching && catalog}
				<EmptyState icon="audiobook" title="No audiobooks on this page" />
			{/if}
		</div>
		{#if total !== undefined && total > limit}
			<nav aria-label="Audiobook library pages">
				<Button disabled={pending || currentPage === 0} onclick={() => changePage(currentPage - 1)}>Previous</Button>
				<span>Page {currentPage + 1} of {Math.ceil(total / limit)}</span>
				<Button disabled={pending || (currentPage + 1) * limit >= total} onclick={() => changePage(currentPage + 1)}>Next</Button>
			</nav>
		{/if}
	{/if}
</LibraryToolPage>

<style>
	.library-search { width: min(100%, 18rem); }
	.catalog { overflow-x: auto; }
	.catalog .book-cover { width: 44px; }
	@media (max-width: 639px) {
		.catalog .tp-who { gap: 8px; }
		.catalog .book-cover { width: 32px; }
	}
	nav { display: flex; gap: 12px; align-items: center; justify-content: end; }
</style>
