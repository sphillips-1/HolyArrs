<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import SearchField from '#lib/discover/SearchField.svelte';
	import LibraryToolPage from '#lib/mediafiles/LibraryToolPage.svelte';
	import Button from '#lib/ui/Button.svelte';
	import EmptyState from '#lib/ui/EmptyState.svelte';
	import { api, message } from '#lib/core/api.ts';
	import { bytes } from '#lib/core/format.ts';
	import { to } from '#lib/core/paths.ts';
	import { audiobookSearch, audiobookSearchHooks } from '#lib/core/search-field.svelte.ts';
	import { session } from '#lib/core/session.svelte.ts';
	import type { SourceView } from '#lib/sources/index.ts';

	interface Release {
		id: string;
		title: string;
		authors?: { first_name: string; last_name: string }[];
		totaltime?: string;
		language?: string;
		size?: number;
		seeders?: number;
	}

	let source = $state('librivox');
	let sourceOptions = $state<SourceView[]>([]);
	let releases = $state<Release[]>([]);
	let searched = $state<{ title: string; source: string } | null>(null);
	let searching = $state(false);
	let grabbing = $state('');
	let notice = $state('');
	let error = $state('');
	let sourceError = $state('');
	const busy = $derived(searching || grabbing !== '');
	const sourceLabel = $derived(source === 'librivox' ? 'LibriVox' : sourceOptions.find((item) => String(item.id) === source)?.name ?? 'Source');
	$effect(() => { audiobookSearch.busy = busy; });

	onMount(() => {
		if (session.s.role !== 'owner') return;
		audiobookSearchHooks.submit = () => void search();
		const query = page.url.searchParams.get('q');
		if (query) {
			audiobookSearch.term = query.slice(0, 200);
			void search();
		}
		void api.get<{ sources: SourceView[] }>('/sources').then((result) => {
			sourceOptions = result.sources.filter((item) => item.state === 'enabled' && item.protocol === 'torrent'
				&& item.format === 'torznab' && item.use_for.hand_search && item.kinds?.includes('audiobook'));
		}).catch(() => {
			sourceError = 'Configured Sources could not be loaded.';
		});
		return () => {
			audiobookSearchHooks.submit = undefined;
			audiobookSearch.busy = false;
		};
	});

	function reset() {
		searched = null;
		releases = [];
		notice = '';
		error = '';
	}

	async function search() {
		const title = audiobookSearch.term.trim();
		if (busy || !title) return;
		reset();
		const query = { title, source };
		searching = true;
		try {
			releases = await api.get<Release[]>(`/audiobooks/search?${new URLSearchParams(query)}`);
			searched = query;
		} catch (cause) {
			error = message(cause);
		} finally {
			searching = false;
		}
	}

	async function queue(release: Release) {
		if (busy || !searched) return;
		grabbing = release.id;
		notice = '';
		error = '';
		const { title, source } = searched;
		try {
			const result = await api.post<{ duplicates: number; pending: number }>('/audiobooks/grab', source === 'librivox'
				? { id: release.id } : { id: release.id, title, source_id: Number(source) });
			if (result.duplicates) {
				notice = `${release.title} is already in the audiobook downloader.`;
			} else if (result.pending) {
				notice = `The downloader is processing ${release.title}.`;
			} else if (source === 'librivox') {
				notice = `${release.title} was queued paused. Select one set of chapters in qBittorrent, then start it.`;
			} else {
				notice = `${release.title} was sent to the audiobook downloader.`;
			}
		} catch (cause) {
			error = message(cause);
		} finally {
			grabbing = '';
		}
	}
</script>

<svelte:head><title>Audiobooks · HolyArrs</title></svelte:head>

<LibraryToolPage title="Audiobooks" count={releases.length ? String(releases.length) : ''}>
	{#snippet bar()}
		{#if session.s.role === 'owner'}
			<span class="tp-gap"></span>
			<div class="tp-cap">
				<Button href={to('/library/audiobooks')} icon="audiobook">Library</Button>
				<Button href={to('/settings/sources')} icon="settings">Sources</Button>
				<Button href="https://torrents.shelfgoblin.dev/imports/reviews/" icon="arrowUpRight">Review</Button>
			</div>
		{/if}
	{/snippet}
	{#if session.s.role === 'owner'}
		<div class="phone-field"><SearchField searchKey="" testid="search-box-phone" id="q-phone" /></div>
		<div class="source-controls">
			<label for="audiobook-source">Source</label>
			<select class="input" id="audiobook-source" bind:value={source} onchange={reset} disabled={busy}>
				<option value="librivox">LibriVox</option>
				{#each sourceOptions as item (item.id)}<option value={String(item.id)}>{item.name}</option>{/each}
			</select>
			<Button variant="primary" icon="search" busy={searching} disabled={busy || !audiobookSearch.term.trim()} onclick={search}>Search</Button>
		</div>
		{#if sourceError}<p role="status" class="muted">{sourceError}</p>{/if}
		{#if error}<p role="alert">{error}</p>{/if}
		{#if notice}<p role="status">{notice}</p>{/if}
		<div class="tp-sheet results">
			{#if releases.length}
				<table class="tp-table">
					<thead><tr><th>Release</th><th>Source</th><th>Details</th><th><span class="sr-only">Actions</span></th></tr></thead>
					<tbody>
						{#each releases as release (release.id)}
							<tr>
								<td><strong>{release.title}</strong>
									{#if release.authors?.length}<div class="muted">{release.authors.map((author) => [author.first_name, author.last_name].filter(Boolean).join(' ')).join(', ')}</div>{/if}
								</td>
								<td>{sourceLabel}</td>
								<td class="muted">
									{#if searched?.source === 'librivox'}
										{[release.language, release.totaltime].filter(Boolean).join(' · ')}
										<div><a href={`https://librivox.org/rss/${encodeURIComponent(release.id)}`}>Recording details</a></div>
									{:else}
										{[release.size ? bytes(release.size) : '', release.seeders != null ? `${release.seeders} seeders` : ''].filter(Boolean).join(' · ') || '—'}
									{/if}
								</td>
								<td><Button size="sm" icon="download" busy={grabbing === release.id} disabled={busy} onclick={() => queue(release)}>{searched?.source === 'librivox' ? 'Grab paused' : 'Grab'}</Button></td>
							</tr>
						{/each}
					</tbody>
				</table>
			{:else if searching}
				<EmptyState icon="search" title={`Searching ${sourceLabel}`} />
			{:else if searched}
				<EmptyState icon="search" title="No releases found" text="Try a different title or Source." />
			{:else}
				<EmptyState icon="audiobook" title="Find an audiobook" text="Search LibriVox or a torrent indexer you’ve enabled for audiobooks in Sources." />
			{/if}
		</div>
	{:else}
		<EmptyState icon="audiobook" title="Sign in as the owner to use Audiobooks" />
	{/if}
</LibraryToolPage>

<style>
	.phone-field { display: none; }
	@media (max-width: 767px) {
		.phone-field { display: block; }
		.phone-field :global(.ui-search) { width: 100%; }
	}
	.source-controls { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
	.source-controls label { font: var(--t-subhead); }
	.source-controls select { min-width: 0; max-width: 100%; }
	.results { overflow-x: auto; }
	.results td:last-child { text-align: end; }
</style>
