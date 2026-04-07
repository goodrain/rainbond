// Copyright 2012-2015 Apcera Inc. All rights reserved.

package termtables

import (
	"testing"
)

// capability_id: rainbond.util.termtables.render-text-table
func TestBasicRowRender(t *testing.T) {
	row := CreateRow([]interface{}{"foo", "bar"})
	style := &renderStyle{TableStyle: TableStyle{BorderX: "-", BorderY: "|", BorderI: "+",
		PaddingLeft: 1, PaddingRight: 1}, cellWidths: map[int]int{0: 3, 1: 3}}

	output := row.Render(style)
	if output != "| foo | bar |" {
		t.Fatal("Unexpected output:", output)
	}
}

// capability_id: rainbond.util.termtables.render-row-width-padding
func TestRowRenderWidthBasedPadding(t *testing.T) {
	row := CreateRow([]interface{}{"foo", "bar"})
	style := &renderStyle{TableStyle: TableStyle{BorderX: "-", BorderY: "|", BorderI: "+",
		PaddingLeft: 1, PaddingRight: 1}, cellWidths: map[int]int{0: 3, 1: 5}}

	output := row.Render(style)
	if output != "| foo | bar   |" {
		t.Fatal("Unexpected output:", output)
	}
}
