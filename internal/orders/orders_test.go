package orders

import "testing"

const listPage = `Order guarantee
 Delivered on Mar 3, 2026
View order details
2 items:
Order total 13,21€
13,21€
Order Time:
Mar 1, 2026
Order ID:
PO-111-00000000000000001
Refunded
View order details
1 item:
Order total 8,64€
Order Time:
Feb 2, 2026
Order ID:
PO-111-00000000000000002
Can't find your order?`

func TestParseList(t *testing.T) {
	got := Parse(listPage)
	if len(got) != 2 {
		t.Fatalf("got %d orders, want 2", len(got))
	}
	if got[0].ID != "PO-111-00000000000000001" || got[0].Total != "13,21€" || got[0].Count != "2" || got[0].Date != "Mar 1, 2026" {
		t.Errorf("first order parsed wrong: %+v", got[0])
	}
	if got[1].Status != "Refunded" || got[1].Total != "8,64€" {
		t.Errorf("second order parsed wrong: %+v", got[1])
	}
}

const detailPage = `Shipping to
Example Person
Delivered on Mar 3, 2026
Tape cartridge 12mm
4,25€
white on black
×2
Free shipping from Temu, sold by Example Store
Warranty Information
RC car tool
3,22€
×1
Track order
Order total:
57,66€
Explore your interests
Unrelated product
€9.99
9,99€`

func TestParseItems(t *testing.T) {
	got := parseItems(detailPage)
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(got), got)
	}
	if got[0].Title != "Tape cartridge 12mm" || got[0].Variant != "white on black" || got[0].Qty != "2" || got[0].Seller != "Example Store" {
		t.Errorf("first item wrong: %+v", got[0])
	}
	if got[1].Title != "RC car tool" || got[1].Variant != "" || got[1].Price != "3,22€" {
		t.Errorf("second item wrong: %+v", got[1])
	}
}
