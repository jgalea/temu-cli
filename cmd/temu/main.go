// Command temu reads your own Temu order history from the command line, by
// driving a real Chrome profile you signed into once.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jgalea/temu-cli/internal/browser"
	"github.com/jgalea/temu-cli/internal/orders"
)

const usage = `temu - read your Temu orders from the command line

Usage:
  temu login            sign in by hand, once, in a dedicated Chrome profile
  temu status           check whether that profile is still signed in
  temu orders [flags]   list your orders
  temu dump [file]      save the rendered order page text, for fixing the parser

Flags:
  --account NAME  use a separate signed-in profile
  --limit N       how many orders to return (default all loaded)
  --expand        open each order for its item titles (slower)
  --grep TEXT     only orders whose items match TEXT (use with --expand)
  --json          emit JSON
  --show          run with a visible window (default for login)

Environment:
  TEMU_CONFIG_DIR  where the browser profile lives (default ~/.temu)
  TEMU_ACCOUNT     default for --account
  TEMU_CHROME      path to the Chrome or Chromium binary
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "temu: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usage)
		return nil
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 0, "how many orders to return (0 for all)")
	expand := fs.Bool("expand", false, "open each order for its item titles (slower)")
	grep := fs.String("grep", "", "only orders whose text matches this")
	asJSON := fs.Bool("json", false, "emit JSON")
	show := fs.Bool("show", false, "run with a visible window")
	accountName := fs.String("account", "", "use a separate signed-in profile")
	var pos []string
	for {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		rest = fs.Args()[1:]
	}

	name := *accountName
	if name == "" {
		name = os.Getenv("TEMU_ACCOUNT")
	}
	if name == "default" {
		name = ""
	}
	if err := browser.SetAccount(name); err != nil {
		return err
	}
	profile := filepath.Join(browser.ConfigDir(), "profile")

	switch cmd {
	case "login":
		return login(profile)
	case "status":
		s, err := session(profile, !*show, 90*time.Second)
		if err != nil {
			return err
		}
		defer s.Close()
		restore(s)
		ok, err := orders.OnOrders(s.Ctx)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("signed in")
		} else {
			fmt.Println("signed out (run `temu login`)")
		}
		return nil
	case "orders", "dump":
		s, err := session(profile, !*show, 5*time.Minute)
		if err != nil {
			return err
		}
		defer s.Close()
		restore(s)
		text, err := orders.PageText(s.Ctx, 40)
		if err != nil {
			return err
		}
		if cmd == "dump" {
			out := "temu-orders.txt"
			if len(pos) > 0 {
				out = pos[0]
			}
			if err := os.WriteFile(out, []byte(text), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "wrote %d bytes to %s\n", len(text), out)
			return nil
		}
		got := orders.Parse(text)
		if *limit > 0 && len(got) > *limit {
			got = got[:*limit]
		}
		if *expand {
			if err := orders.Expand(s.Ctx, got); err != nil {
				return err
			}
		}
		if *grep != "" {
			var kept []orders.Order
			for _, o := range got {
				hay := o.ID
				for _, it := range o.Items {
					hay += " " + it.Title + " " + it.Variant + " " + it.Seller
				}
				if strings.Contains(strings.ToLower(hay), strings.ToLower(*grep)) {
					kept = append(kept, o)
				}
			}
			got = kept
		}
		if *asJSON {
			b, err := orders.JSON(got)
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		for _, o := range got {
			fmt.Printf("%s  %s  %s  [%s]  items: %s\n", o.ID, o.Date, o.Total, o.Status, o.Count)
			for _, it := range o.Items {
				fmt.Printf("    %s  x%s  %s\n", truncate(it.Title, 90), it.Qty, it.Price)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q (try `temu help`)", cmd)
	}
}

func session(profile string, headless bool, timeout time.Duration) (*browser.Session, error) {
	return browser.Open(context.Background(), browser.Options{ProfileDir: profile, Headless: headless, Timeout: timeout})
}

func restore(s *browser.Session) {
	_, _ = browser.LoadCookies(s.Ctx, browser.CookiePath())
}

func login(profile string) error {
	s, err := session(profile, false, 0)
	if err != nil {
		return err
	}
	defer s.Close()
	fmt.Fprintln(os.Stderr, "A Chrome window is opening. Sign in to Temu there.")
	fmt.Fprintln(os.Stderr, "It waits and picks the session up on its own; press Enter here once you are in if it has not noticed.")
	enter := make(chan struct{}, 1)
	go func() {
		r := bufio.NewReader(os.Stdin)
		for {
			if _, err := r.ReadString('\n'); err != nil {
				return
			}
			select {
			case enter <- struct{}{}:
			default:
			}
		}
	}()
	if err := orders.WaitForLogin(s.Ctx, enter); err != nil {
		return err
	}
	n, err := browser.SaveCookies(s.Ctx, browser.CookiePath())
	if err != nil {
		return fmt.Errorf("signed in, but saving the session failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Signed in. Saved %d cookies to %s.\n", n, browser.CookiePath())
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
