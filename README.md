# atom1c

> On Development

A self-hosted Atom feed aggregator and terminal reader accessible over SSH.

<p align="center">
  <img src="demo.gif" alt="Demo">
</p>

## motivation

It's a learning project. I'm using it to get properly comfortable with the Charm TUI stack (Bubble Tea, Bubbles, Lipgloss).

## quick start

Requires Go 1.26+.

```sh
git clone https://github.com/su1uv/atom1c.git
cd atom1c
echo 'GOOSE_DBSTRING=./atom1c.db' > .env
go run .
```

Database migrations run automatically at startup, so there's nothing else to set up.

Heads up: the SSH part isn't implemented yet. Right now atom1c runs as a plain local TUI.

## usage

Main view:

| key | action |
| --- | --- |
| `a` | add a feed |
| `j` / `k` (or arrows) | move up and down the list |
| `h` / `l` (or arrows) | previous / next page |
| `/` | filter the list |
| `tab` | open the selected feed's posts |
| `shift+tab` | back to feeds |
| `P` | toggle pagination |
| `q` / `ctrl+c` | quit |

Add feed modal:

| key | action |
| --- | --- |
| `tab` / `shift+tab` (or `up` / `down`) | move between fields |
| `enter` | submit |
| `ctrl+r` | change cursor style |
| `esc` | close |

## contributing

I'm not accepting pull requests for now.

That said, if you spot something wrong or weird, or just have an opinion, please open an issue; I'd like to hear it.
