# Audiobooks prototype

This fork modifies HolyArrs 1.2.0 on November 21, 2026, under AGPL-3.0.
It adds an owner-only Audiobooks page. The page
searches LibriVox editions and adds the selected Internet Archive torrent
paused to a dedicated qBittorrent 5.2+ instance. Torrents can contain several
copies of each chapter. In qBittorrent, select one recording format before
starting the torrent. The homelab importer uses the torrent's LibriVox
edition tag to import a verified recording automatically. Ambiguous recordings
and failed imports appear in **Review**, linked from the Audiobooks page.

LibriVox is selected by default. To add another indexer, open **Settings →
Sources** and create or edit a Torznab torrent Source. Enable **Audiobooks**
and **Hand search**. Its results appear in the Audiobooks release table and
go to the dedicated audiobook downloader when grabbed. Torznab searches use
category 3030 (Audiobook) unless the Source supplies an audiobook category.
Downloads from these Sources use the homelab’s existing import workflow.

Set these environment variables on the test HolyArrs instance:

- `HOLYARRS_AUDIOBOOK_QBIT_URL` is the audiobook client's internal base URL.
- `HOLYARRS_AUDIOBOOK_QBIT_KEY` is the qBittorrent API key. Keep it in a runtime
  secret and ensure the configured origin belongs to the audiobook downloader.
- `HOLYARRS_UPDATES=off` keeps the test build on the fork's code.

The test instance should use its own HolyArrs configuration and database. The
downloader should already save finished books to the importer pickup directory.
The fork reuses the upstream qBittorrent client's existing duplicate detection.

Search for **Pride and Prejudice**, select LibriVox edition 253, then queue it.
Verify the torrent appears paused on the audiobook downloader, select one set
of chapter files, and start it. After completion, verify the importer publishes
it to Audiobookshelf. Use a separate ambiguous fixture to check the Review path.
This requires the companion homelab importer changes and an active review worker.

The prototype covers public-domain catalog search and owner-initiated queueing.
A shared request queue and integrated download and import status would require
additional work. The account menu links to the corresponding source.
