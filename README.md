# nihongo

**A minimal, blazing-fast Japanese dictionary CLI tool** powered by [Yomitan](https://github.com/yomidevs/yomitan)-compatible dictionaries (default: [Jitendex](https://jitendex.org/)).

Search instantly by **romaji**, **hiragana**, **katakana**, **kanji**, or **mixed kana+kanji**. Or look up Japanese words from English using the `-e` flag. All in a single static binary with zero external dependencies.

## Features

- ✅ Reads full definitions, translations, examples, part-of-speech, and tags from any Yomitan-compatible `.zip` dictionary
- ✅ Interactive **fuzzy search** with `go-fuzzyfinder` (or plain list mode with `-fzf=false`)
- ✅ **Single static binary** — no runtime dependencies, works everywhere Go does
- ✅ Lightning-fast prefix + fuzzy matching on expressions and romaji readings
- ✅ Supports input styles:
  - Romaji: `sonzai`, `aruji`, `gohan`
  - Hiragana: `ぱん`, `ごはん`, `かんじる`
  - Katakana: `パン`, `パソコン`, `スーパーマーケット`
  - Kanji: `日本語`, `食べる`, `人`
  - Mixed: `日本語`, `感じる`, `行く`
  - English → Japanese: `nihongo -e bread`, `nihongo -e existence`
- ✅ One-command dictionary setup: `nihongo -download` (pulls latest Jitendex automatically)
- ✅ Import your own Yomitan dictionary: `nihongo -import /path/to/dict.zip`
- ✅ View full entry details (definitions, example sentences, tags, etc.)
- ✅ Lookup by internal ID: `nihongo -id 115754`
- ✅ Show installed dictionary metadata: `nihongo -info`

## Installation

### Run with Nix
```bash
nix run github:jim-ww/nihongo -- 日本語
```
Or add it to your flake.nix.
### Pre-built binaries

Check the [Releases](https://github.com/jim-ww/nihongo/releases) page for static binaries for Linux, macOS, and Windows.

### Build from source

```bash
git clone https://github.com/jim-ww/nihongo.git
cd nihongo
go build -o nihongo .
sudo mv nihongo /usr/local/bin/
```

Or install directly:

```bash
go install github.com/jim-ww/nihongo@latest
```

## Usage

```bash
nihongo [flags] <search-term>
```

### Quick Examples

**Japanese search (default interactive fuzzy mode):**

```bash
$ nihongo 日本語
```

You will see a interactive list. Select any entry to view the full dictionary card.

**Non-interactive list mode with internal IDs (`-fzf=false`):**

```bash
$ nihongo -fzf=false 日本語
```

**English search:**

```bash
$ nihongo -e computer
```

**Romaji / Kana / Kanji all work out of the box:**

```bash
$ nihongo sonzai
$ nihongo ごはん
$ nihongo パソコン
$ nihongo 食べる
$ nihongo 感じる
```

**Other useful commands:**

```bash
# Download & install the default dictionary (Jitendex)
nihongo -download

# Or import a custom Yomitan dictionary
nihongo -import ~/Downloads/my-dict.zip

# Show info about currently installed dictionary
nihongo -info

# Lookup a specific entry by its internal ID
nihongo -id 115754

# Show romaji readings in the result list
nihongo -r 日本語

# Limit number of results
nihongo -l 10 日本語

# Clear cached downloads
nihongo -clearCache
```

## Command-line Flags

| Flag                  | Description                                      | Default |
|-----------------------|--------------------------------------------------|---------|
| `-download`           | Download latest Jitendex dictionary              | false   |
| `-import <path>`      | Import a Yomitan `.zip` dictionary               | —       |
| `-e`                  | Treat input as English (search definitions)      | false   |
| `-r`                  | Show romaji reading in result list               | false   |
| `-fzf`                | Enable/disable interactive fuzzy finder          | true    |
| `-l <n>`              | Limit number of search results                   | 20      |
| `-id <n>`             | Print full entry by database row ID              | —       |
| `-info`               | Show metadata of installed dictionary            | false   |
| `-showID`             | Show database IDs in results (fzf mode only)     | true    |
| `-truncate`           | Wipe previous dictionary before new import       | true    |
| `-dictDownloadUrl`    | Custom dictionary download URL                   | Jitendex latest |
| `-clearCache`         | Delete cached downloaded zip files               | false   |
| `-version`            | Print version and build info                     | false   |

## Contributing

Bug fixes, improvements, new features, and ideas are **welcome**

## Donate
If nihongo saves you time or brings you joy, consider a small donation:

**Monero (XMR)**
`83YGRqP8uHed6NeegZQeX9ccCxbzoRHHEEi7pTwk4aqdJZEVXXA6NWtetnsEM2v33zFBBt3Rp6DNhU9qhJEGPspU14yN8t7`

All donations are greatly appreciated and go directly toward keeping the project alive and adding new features.

## License Notice

This program is free software licensed under the **GNU General Public License v3 (GPLv3)**.

It means this is **free software** — you are free to use, study, share, and modify it however you like (as long as you keep the same freedoms for others).
