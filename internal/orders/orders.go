// Package orders reads the signed-in account's order list off the Temu web app.
package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	ListURL  = "https://www.temu.com/bgt_orders.html"
	LoginURL = "https://www.temu.com/login.html"
)

type Item struct {
	Title   string `json:"title"`
	Variant string `json:"variant,omitempty"`
	Qty     string `json:"qty,omitempty"`
	Price   string `json:"price,omitempty"`
	Seller  string `json:"seller,omitempty"`
}

type Order struct {
	ID     string `json:"id"`
	Date   string `json:"date,omitempty"`
	Status string `json:"status,omitempty"`
	Total  string `json:"total,omitempty"`
	Count  string `json:"items_count,omitempty"`
	URL    string `json:"url"`
	Items  []Item `json:"items"`
}

var idRe = regexp.MustCompile(`PO-\d{3}-\d{10,}`)

// OnOrders reports whether the page we landed on is the order list rather
// than a login or home redirect.
func OnOrders(ctx context.Context) (bool, error) {
	var current string
	err := chromedp.Run(ctx,
		chromedp.Navigate(ListURL),
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&current),
	)
	if err != nil {
		return false, err
	}
	return strings.Contains(current, "bgt_orders"), nil
}

// WaitForLogin polls until the order list loads without a redirect, or done closes.
func WaitForLogin(ctx context.Context, enter <-chan struct{}) error {
	if err := chromedp.Run(ctx, chromedp.Navigate(LoginURL)); err != nil {
		return err
	}
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-enter:
			var loc string
			if err := chromedp.Run(ctx, chromedp.Navigate(ListURL), chromedp.Sleep(5*time.Second), chromedp.Location(&loc)); err != nil {
				return err
			}
			if strings.Contains(loc, "bgt_orders") {
				return nil
			}
			fmt.Fprintln(os.Stderr, "Not signed in yet: the order page redirected. Finish signing in, then press Enter again.")
			if err := chromedp.Run(ctx, chromedp.Navigate(LoginURL)); err != nil {
				return err
			}
		case <-tick.C:
			var loc, text string
			if err := chromedp.Run(ctx, chromedp.Location(&loc), chromedp.Evaluate(`document.body.innerText`, &text)); err != nil {
				continue
			}
			if !strings.Contains(loc, "login") && idRe.MatchString(text) {
				return nil
			}
		}
	}
}

// PageText loads the order list and returns the rendered text, scrolling
// until the page stops growing.
func PageText(ctx context.Context, maxScrolls int) (string, error) {
	ok, err := OnOrders(ctx)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("not signed in; run `temu login` first")
	}
	last := -1
	var text string
	for i := 0; i < maxScrolls; i++ {
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil),
			chromedp.Sleep(2*time.Second),
			chromedp.Evaluate(`document.body.innerText`, &text),
		); err != nil {
			return "", err
		}
		if len(text) == last {
			break
		}
		last = len(text)
	}
	return text, nil
}

const zw = "\u200b"

func clean(text string) []string {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(text, zw, ""), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

var (
	countRe  = regexp.MustCompile(`^(\d+) items?:$`)
	totalRe  = regexp.MustCompile(`^Order total (.+)$`)
	statusRe = regexp.MustCompile(`^(Delivered on|Shipped|Processing|Refunded|Partially refunded|Cancelled|Canceled|Out for delivery|Estimated delivery).*$`)
	priceRe  = regexp.MustCompile(`^[\d.]+,\d{2}€$|^€[\d.,]+$`)
	qtyRe    = regexp.MustCompile(`^×\d+$`)
)

// Parse splits the rendered order page into orders. Each order's details
// come before its "Order ID:" line, so a block ends at an order id.
func Parse(text string) []Order {
	lines := clean(text)
	var out []Order
	seen := map[string]bool{}
	start := 0
	for i, l := range lines {
		if l == "Order guarantee" && len(out) == 0 {
			start = i + 1
		}
		if l != "Order ID:" || i+1 >= len(lines) || !idRe.MatchString(lines[i+1]) {
			continue
		}
		o := Order{ID: lines[i+1], Items: []Item{}, URL: DetailURL(lines[i+1])}
		for _, b := range lines[start:i] {
			switch {
			case countRe.MatchString(b):
				o.Count = countRe.FindStringSubmatch(b)[1]
			case totalRe.MatchString(b):
				o.Total = totalRe.FindStringSubmatch(b)[1]
			case statusRe.MatchString(b) && o.Status == "":
				o.Status = b
			}
		}
		if i > 0 && lines[i-1] != "Order Time:" {
			o.Date = lines[i-1]
		}
		start = i + 2
		if !seen[o.ID] {
			seen[o.ID] = true
			out = append(out, o)
		}
	}
	return out
}

func DetailURL(id string) string {
	return "https://www.temu.com/bgt_order_detail.html?parent_order_sn=" + id + "&page_from=1&belongTab=AO"
}

// Expand opens each order's detail page and fills in its items.
func Expand(ctx context.Context, got []Order) error {
	for i := range got {
		var text string
		if err := chromedp.Run(ctx,
			chromedp.Navigate(DetailURL(got[i].ID)),
			chromedp.Sleep(4*time.Second),
			chromedp.Evaluate(`document.body.innerText`, &text),
		); err != nil {
			return err
		}
		got[i].Items = parseItems(text)
	}
	return nil
}

func parseItems(text string) []Item {
	lines := clean(text)
	from, to := 0, len(lines)
	found := false
	for i, l := range lines {
		if l == "Shipping to" && !found {
			from, found = i, true
		}
		switch l {
		case "Track order", "Leave a review", "Buy this again", "Order total:", "Payment details", "Explore your interests", "Can't find your order?":
			if found && i > from && to == len(lines) {
				to = i
			}
		}
	}
	var items []Item
	for i := from; i < to; i++ {
		if !priceRe.MatchString(lines[i]) || i == 0 {
			continue
		}
		title := lines[i-1]
		if strings.HasSuffix(title, ":") || priceRe.MatchString(title) {
			continue
		}
		it := Item{Title: title, Price: lines[i], Qty: "1"}
		j := i + 1
		if j < to && !qtyRe.MatchString(lines[j]) && !strings.HasPrefix(lines[j], "Free shipping") {
			it.Variant = lines[j]
			j++
		}
		if j < to && qtyRe.MatchString(lines[j]) {
			it.Qty = strings.TrimPrefix(lines[j], "×")
			j++
		}
		if j < to && strings.HasPrefix(lines[j], "Free shipping") {
			if k := strings.Index(lines[j], "sold by "); k >= 0 {
				it.Seller = lines[j][k+len("sold by "):]
			}
		}
		items = append(items, it)
	}
	return items
}

func JSON(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }
