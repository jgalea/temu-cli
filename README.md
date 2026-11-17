<h1 align="center">temu-cli</h1>

<p align="center">
  <a href="go.mod"><img src="https://img.shields.io/badge/GO-1.26%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/LICENSE-MIT-5C9E31?style=for-the-badge" alt="License"></a>
  <a href="https://github.com/jgalea"><img src="https://img.shields.io/badge/BUILT%20BY-JEAN%20GALEA-8A2BE2?style=for-the-badge" alt="Built by"></a>
</p>

<p align="center"><strong>Unofficial CLI for your own Temu account. Sign in once by hand, then read your order history as text or JSON.</strong></p>

---

Temu has no consumer API. `temu` drives a real Chrome profile you signed into yourself and reads the order list and each order's detail page out of the rendered page. There is no scripted login, no stored password, and no attempt to get past a captcha.

It is the sibling of [ali-cli](https://github.com/jgalea/ali-cli).

## Install

```
go install github.com/jgalea/temu-cli/cmd/temu@latest
```

Needs Go 1.26+ and a local Chrome or Chromium.

## Use

```
temu login             # opens a window; sign in yourself
temu status            # is that profile still signed in?
temu orders            # every order: id, date, total, status, item count
temu orders --expand   # open each order for its item titles, prices and sellers
temu orders --expand --grep "tamiya" --json
temu dump orders.txt   # save the rendered order page text, for fixing the parser
```

Use `--account NAME` for a second signed-in profile. `TEMU_CONFIG_DIR` moves the profile (default `~/.temu`), and `TEMU_CHROME` points at the browser binary.

## License

MIT
