# Atom1c

> On Development

A self-hosted Atom feed aggregator and terminal reader accessible over SSH.

<p align="center">
  <img src="demo.gif" alt="Demo">
</p>

## Motivation

It's a learning project. I'm using it to get properly comfortable with the Charm TUI stack (Bubble Tea, Bubbles, Lipgloss).

## Quick Start

Requires Go 1.26+.

```sh
git clone https://github.com/su1uv/atom1c.git
cd atom1c
echo 'GOOSE_DBSTRING=./atom1c.db' > .env
go run .
```

Database migrations run automatically at startup, so there's nothing else to set up.

Heads up: the SSH part isn't implemented yet. Right now atom1c runs as a plain local TUI.

## Usage

Main view:

| key | action |
| --- | --- |
| `a` | add a feed |
| `j` / `k` (or up / down) | move within the current list page |
| `h` / `l` (or left / right) | previous / next page when the feeds pane is focused |
| `/` | search all feeds by name (case-insensitive substring) |
| `tab` | open the selected feed's posts |
| `shift+tab` | back to feeds |
| `P` | toggle the pagination indicator for the focused pane |
| `r` | retry a failed feed load |
| `q` / `ctrl+c` | quit |

Feed search updates as you type. Press `enter` to stop editing, `esc` to leave
search editing, and backspace to change or clear the query. Feed pages fit the
current pane size; the list is loaded from SQLite rather than limited to a fixed
number of feeds.

Add feed modal:

| key | action |
| --- | --- |
| `tab` / `shift+tab` (or `up` / `down`) | move between fields |
| `enter` | submit |
| `ctrl+r` | change cursor style |
| `esc` | close before saving |

Feed names must be nonblank and URLs must be absolute HTTP(S) URLs. The modal
remains open while saving; failed saves preserve the draft so it can be corrected
or retried.

## Contributing

I'm not accepting pull requests for now.

That said, if you spot something wrong or weird, or just have an opinion, please open an issue; I'd like to hear it.
