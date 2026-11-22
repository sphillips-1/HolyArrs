<script lang="ts">
	import { api, ApiError } from '#lib/core/api.ts';
	import { session } from '#lib/core/session.svelte.ts';

	interface Edition {
		id: string;
		title: string;
		authors: { first_name: string; last_name: string }[];
		totaltime: string;
		language: string;
	}

	let title = $state('');
	let editions = $state<Edition[]>([]);
	let active = $state(false);
	let notice = $state('');
	let error = $state('');

	function problem(cause: unknown): string {
		if (cause instanceof ApiError) {
			return cause.message;
		}
		return 'The audiobook service could not be reached.';
	}

	async function search(event: SubmitEvent) {
		event.preventDefault();
		if (active || !title.trim()) return;
		active = true;
		notice = '';
		error = '';
		editions = [];
		try {
			editions = await api.get<Edition[]>(`/audiobooks/search?title=${encodeURIComponent(title.trim())}`);
			if (!editions.length) notice = 'No LibriVox editions found.';
		} catch (cause) {
			error = problem(cause);
		} finally {
			active = false;
		}
	}

	async function queue(edition: Edition) {
		if (active) return;
		active = true;
		notice = '';
		error = '';
		try {
			const result = await api.request<{ duplicates: number; pending: number }>('/audiobooks/grab', {
				method: 'POST',
				body: { id: edition.id }
			});
			if (result.duplicates) {
				notice = `${edition.title} is already in the audiobook downloader.`;
			} else if (result.pending) {
				notice = `The downloader is processing ${edition.title}. Check qBittorrent for its status.`;
			} else {
				notice = `${edition.title} was queued paused. Select one set of chapters in qBittorrent, then start it.`;
			}
		} catch (cause) {
			error = problem(cause);
		} finally {
			active = false;
		}
	}
</script>

<svelte:head><title>Audiobooks · HolyArrs</title></svelte:head>

<section aria-labelledby="audiobooks-heading">
	<h1 id="audiobooks-heading">Audiobooks</h1>
	{#if session.s.role === 'owner'}
		<p>Find public-domain LibriVox recordings. A complete recording can import automatically. Files that need a decision appear in <a href="https://torrents.shelfgoblin.dev/imports/reviews/">Review</a>.</p>
		<form onsubmit={search}>
			<label for="audiobook-title">Book title</label>
			<div class="search-fields">
				<input id="audiobook-title" bind:value={title} maxlength="200" required />
				<button type="submit" disabled={active}>Search</button>
			</div>
		</form>
		{#if error}<p role="alert">{error}</p>{/if}
		<p role="status">{notice}</p>
		{#if editions.length}
			<ul>
				{#each editions as edition (edition.id)}
					<li>
						<div>
							<strong>{edition.title}</strong>
							<div>{edition.authors?.map((author) => [author.first_name, author.last_name].filter(Boolean).join(' ')).join(', ')}</div>
							<div>{[edition.language, edition.totaltime].filter(Boolean).join(' · ')}</div>
							<a href={`https://librivox.org/rss/${encodeURIComponent(edition.id)}`}>Recording details</a>
						</div>
						<button type="button" disabled={active} onclick={() => queue(edition)}>Queue paused</button>
					</li>
				{/each}
			</ul>
		{/if}
	{:else}
		<p>Sign in as the owner to use Audiobooks.</p>
	{/if}
</section>

<style>
	section { max-width: 48rem; margin: 2rem auto; padding: 0 1rem; }
	label { display: block; margin-bottom: .4rem; }
	.search-fields, li { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; }
	li { justify-content: space-between; padding: .75rem 0; border-bottom: 1px solid currentColor; }
	input { flex: 1 1 16rem; }
	input, button { padding: .5rem .75rem; border: 1px solid currentColor; border-radius: .4rem; }
	ul { list-style: none; padding: 0; }
</style>
