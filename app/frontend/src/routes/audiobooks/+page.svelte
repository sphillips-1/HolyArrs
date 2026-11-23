<script lang="ts">
	import { onMount } from 'svelte';
	import LibraryToolPage from '#lib/mediafiles/LibraryToolPage.svelte';
	import Button from '#lib/ui/Button.svelte';
	import EmptyState from '#lib/ui/EmptyState.svelte';
	import { api, message } from '#lib/core/api.ts';
	import { bytes } from '#lib/core/format.ts';
	import { to } from '#lib/core/paths.ts';
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

	let title = $state('');
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

	onMount(() => {
		if (session.s.role !== 'owner') return;
		void api.get<{ sources: SourceView[] }>('/sources').then((result) => {
			sourceOptions = result.sources.filter((item) => item.state === 'enabled' && item.protocol === 'torrent'
				&& item.format === 'torznab' && item.use_for.hand_search && item.kinds?.includes('audiobook'));
		}).catch(() => {
			sourceError = 'Configured Sources could not be loaded.';
		});
	});

	function reset() {
		searched = null;
		releases = [];
		notice = '';
		error = '';
	}

	async function search(event: SubmitEvent) {
		event.preventDefault();
		if (busy || !title.trim()) return;
		reset();
		const query = { title: title.trim(), source };
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
		<form class="search-form tp-sheet tp-pad" onsubmit={search}>
			<div>
				<label for="audiobook-title">Book title</label>
				<input class="input" id="audiobook-title" bind:value={title} maxlength="200" placeholder="Search audiobooks" required disabled={busy} />
			</div>
			<div>
				<label for="audiobook-source">Source</label>
				<select class="input" id="audiobook-source" bind:value={source} onchange={reset} disabled={busy}>
					<option value="librivox">LibriVox</option>
					{#each sourceOptions as item (item.id)}<option value={String(item.id)}>{item.name}</option>{/each}
				</select>
			</div>
			<Button type="submit" variant="primary" icon="search" busy={searching} disabled={busy}>Search</Button>
		</form>
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
	.search-form { display: flex; flex-wrap: wrap; align-items: end; gap: 12px; }
	.search-form > div:first-child { flex: 2 1 14rem; }
	.search-form > div:nth-child(2) { flex: 1 1 10rem; }
	.search-form label { display: block; margin-bottom: 4px; font: var(--t-subhead); }
	.search-form .input { width: 100%; }
	.results { overflow-x: auto; }
	.results td:last-child { text-align: end; }
</style>
